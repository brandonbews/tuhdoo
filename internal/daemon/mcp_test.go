package daemon

// MCP surface integration tests (B9 Accept): a real SDK client speaking
// streamable HTTP over the daemon's real unix socket. The actor header
// is injected by the test HTTP client, exactly as the tuhdoo mcp shim
// does in production.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/brandonbews/tuhdoo/internal/event"
)

// mcpTestTransport injects the identity headers on every request, like
// the shim's actorTransport. client, when set, asks the daemon to mint
// the agent half of the principal.
type mcpTestTransport struct {
	base   http.RoundTripper
	actor  string
	client string
}

func (t mcpTestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	if t.actor != "" {
		req.Header.Set("X-Tuhdoo-Actor", t.actor)
	}
	if t.client != "" {
		req.Header.Set("X-Tuhdoo-Agent-Name", t.client)
	}
	return t.base.RoundTrip(req)
}

// killableDialer dials the daemon socket and remembers every
// connection, so a test can sever them all at once — a hard drop, with
// no DELETE and no clean close, as if the agent's machine vanished.
type killableDialer struct {
	socket string
	mu     sync.Mutex
	conns  []net.Conn
	dead   bool
}

func (k *killableDialer) DialContext(ctx context.Context, _, _ string) (net.Conn, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.dead {
		return nil, errors.New("connection killed by test")
	}
	var d net.Dialer
	c, err := d.DialContext(ctx, "unix", k.socket)
	if err != nil {
		return nil, err
	}
	k.conns = append(k.conns, c)
	return c, nil
}

// kill severs every open connection and refuses new ones, so the SDK
// client can neither answer keepalive pings nor reconnect.
func (k *killableDialer) kill() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.dead = true
	for _, c := range k.conns {
		c.Close()
	}
}

// mcpConnect connects an SDK client to the daemon's /mcp endpoint as
// actor, using rt as the underlying transport (nil for a plain one).
func mcpConnect(t *testing.T, d *Daemon, actor string, rt http.RoundTripper) *mcp.ClientSession {
	t.Helper()
	if rt == nil {
		rt = &http.Transport{
			DialContext: (&killableDialer{socket: d.SocketPath()}).DialContext,
		}
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test-agent", Version: "0"}, nil)
	cs, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint:   "http://tuhdoo/mcp",
		HTTPClient: &http.Client{Transport: mcpTestTransport{base: rt, actor: actor}},
	}, nil)
	if err != nil {
		t.Fatalf("mcp connect as %s: %v", actor, err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

// mcpTryConnect attempts a session with explicit identity headers and
// returns the SDK's verdict — for exercising the daemon's session-bind
// rejections, which mcpConnect would turn into test failures.
func mcpTryConnect(d *Daemon, actor, client string) (*mcp.ClientSession, error) {
	rt := &http.Transport{
		DialContext: (&killableDialer{socket: d.SocketPath()}).DialContext,
	}
	c := mcp.NewClient(&mcp.Implementation{Name: "test-agent", Version: "0"}, nil)
	return c.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint:   "http://tuhdoo/mcp",
		HTTPClient: &http.Client{Transport: mcpTestTransport{base: rt, actor: actor, client: client}},
	}, nil)
}

// callTool invokes one tool and fails the test on protocol errors.
func callTool(t *testing.T, cs *mcp.ClientSession, name string, args any) *mcp.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}
	return res
}

// mustToolOK invokes a tool, requires success, and decodes the
// structured content into dst.
func mustToolOK(t *testing.T, cs *mcp.ClientSession, name string, args, dst any) {
	t.Helper()
	res := callTool(t, cs, name, args)
	if res.IsError {
		t.Fatalf("%s returned a tool error: %s", name, contentText(res))
	}
	if dst == nil {
		return
	}
	b, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("%s: marshal structured content: %v", name, err)
	}
	if err := json.Unmarshal(b, dst); err != nil {
		t.Fatalf("%s: decode structured content %s: %v", name, b, err)
	}
}

func contentText(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

// flushedEvents flushes the batcher and loads the full event log.
func flushedEvents(t *testing.T, d *Daemon) []event.Event {
	t.Helper()
	if err := d.batcher.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	events, err := d.store.LoadEvents()
	if err != nil {
		t.Fatalf("LoadEvents: %v", err)
	}
	return events
}

// mcpTools is the T5 surface: exactly these eleven, nothing else.
var mcpTools = []string{
	"add_note", "claim_next", "claim_task", "create_task", "escalate",
	"finish_run", "get_backlog", "get_task", "relay_answer", "release_claim",
	"update_task",
}

// Accept 1: a scripted client runs the full loop — create, claim_next,
// add_note, finish_run — and the ledger holds exactly those events with
// the session principal stamped on every one.
func TestMCPFullLoop(t *testing.T) {
	d, _ := startDaemon(t)
	const actor = "brandon/impl-1"
	cs := mcpConnect(t, d, actor, nil)

	// The surface is exactly the eleven T5 verbs.
	var names []string
	for tool, err := range cs.Tools(context.Background(), nil) {
		if err != nil {
			t.Fatalf("list tools: %v", err)
		}
		names = append(names, tool.Name)
	}
	if fmt.Sprint(names) != fmt.Sprint(mcpTools) {
		t.Fatalf("tool surface = %v, want %v", names, mcpTools)
	}

	var created struct {
		IDs []string `json:"ids"`
	}
	mustToolOK(t, cs, "create_task", map[string]any{
		"tasks": []map[string]any{{"title": "wire the MCP surface", "priority": 3}},
	}, &created)
	if len(created.IDs) != 1 {
		t.Fatalf("create_task ids = %v, want one", created.IDs)
	}
	task := created.IDs[0]

	var claimed claimNextResult
	mustToolOK(t, cs, "claim_next", map[string]any{}, &claimed)
	if !claimed.Claimed || claimed.Task == nil {
		t.Fatalf("claim_next = %+v, want a claim", claimed)
	}
	if claimed.Task.Task.ID != task {
		t.Fatalf("claim_next picked %s, want %s", claimed.Task.Task.ID, task)
	}
	if claimed.Task.Claim == nil || claimed.Task.Claim.Actor != actor {
		t.Fatalf("hydrated claim = %+v, want actor %s", claimed.Task.Claim, actor)
	}

	mustToolOK(t, cs, "add_note", map[string]any{
		"task": task, "text": "found the socket path handling",
	}, nil)
	mustToolOK(t, cs, "finish_run", map[string]any{
		"task": task, "outcome": "done", "summary": "wired and tested",
	}, nil)

	// An exhausted pool is a normal outcome, not an error.
	var again claimNextResult
	mustToolOK(t, cs, "claim_next", map[string]any{}, &again)
	if again.Claimed || again.Reason == "" {
		t.Fatalf("claim_next on empty pool = %+v, want claimed:false with a reason", again)
	}

	// Agents may not write daemon-only outcomes.
	res := callTool(t, cs, "finish_run", map[string]any{"task": task, "outcome": "interrupted"})
	if !res.IsError || !strings.Contains(contentText(res), "invalid outcome") {
		t.Fatalf("finish_run(interrupted) = isError %v %q, want a tool error", res.IsError, contentText(res))
	}

	// The ledger: exactly the loop's events, all stamped with the
	// session principal.
	events := flushedEvents(t, d)
	wantTypes := []string{
		event.TypeTaskCreated, event.TypeClaimMade,
		event.TypeNoteAdded, event.TypeRunFinished,
	}
	if len(events) != len(wantTypes) {
		t.Fatalf("event count = %d, want %d", len(events), len(wantTypes))
	}
	for i, e := range events {
		if e.Type != wantTypes[i] {
			t.Errorf("event %d type = %s, want %s", i, e.Type, wantTypes[i])
		}
		if e.Actor != actor {
			t.Errorf("event %d (%s) actor = %s, want %s", i, e.Type, e.Actor, actor)
		}
		if e.Task != task {
			t.Errorf("event %d (%s) task = %s, want %s", i, e.Type, e.Task, task)
		}
	}
}

// Inbox and held over the MCP surface (2026-07-31): title-only capture
// through create_task lands as a real inbox event; get_backlog shows
// the shelves as their own arrays; claim_next skips them; and the full
// promote round trip runs through update_task — fields, not verbs.
func TestMCPInboxCaptureAndPromotion(t *testing.T) {
	d, _ := startDaemon(t)
	const actor = "brandon/impl-1"
	cs := mcpConnect(t, d, actor, nil)

	// Title-only capture: the minimum capture demands. A held create
	// works too (create-into-held is deliberately permitted).
	var created createTasksResult
	mustToolOK(t, cs, "create_task", map[string]any{
		"tasks": []map[string]any{
			{"title": "idea: dark mode", "status": "inbox"},
			{"title": "polish docs", "status": "held", "priority": 9},
		},
	}, &created)
	if len(created.IDs) != 2 {
		t.Fatalf("create_task ids = %v, want two", created.IDs)
	}
	idea, parked := created.IDs[0], created.IDs[1]

	// get_backlog: the shelves are their own arrays, ready is empty.
	var backlog backlogResult
	mustToolOK(t, cs, "get_backlog", map[string]any{}, &backlog)
	if len(backlog.Ready) != 0 {
		t.Fatalf("ready = %+v, want empty", backlog.Ready)
	}
	if len(backlog.Inbox) != 1 || backlog.Inbox[0].ID != idea {
		t.Fatalf("inbox = %+v, want just %s", backlog.Inbox, idea)
	}
	if len(backlog.Held) != 1 || backlog.Held[0].ID != parked {
		t.Fatalf("held = %+v, want just %s", backlog.Held, parked)
	}

	// claim_next never serves a shelf, even a p9 held task.
	var claimed claimNextResult
	mustToolOK(t, cs, "claim_next", map[string]any{}, &claimed)
	if claimed.Claimed {
		t.Fatalf("claim_next served a shelved task: %+v", claimed)
	}
	// claim_task refuses by status, as a tool error the model can read.
	res := callTool(t, cs, "claim_task", map[string]any{"task": idea})
	if !res.IsError || !strings.Contains(contentText(res), "status is inbox") {
		t.Fatalf("claim_task(inbox) = isError %v %q, want status-is-inbox error", res.IsError, contentText(res))
	}

	// Promotion is update_task with a real description — no new verb.
	mustToolOK(t, cs, "update_task", map[string]any{
		"task": idea, "status": "open",
		"description": "Context, the ask, acceptance criteria.",
	}, nil)
	mustToolOK(t, cs, "claim_next", map[string]any{}, &claimed)
	if !claimed.Claimed || claimed.Task.Task.ID != idea {
		t.Fatalf("claim_next after promotion = %+v, want %s", claimed, idea)
	}
	// Pause the claimed-and-released work mid-flight: open→held→open.
	mustToolOK(t, cs, "release_claim", map[string]any{"task": idea, "reason": "testing pause"}, nil)
	var updated taskJSON
	mustToolOK(t, cs, "update_task", map[string]any{"task": idea, "status": "held"}, &updated)
	if updated.Status != "held" {
		t.Fatalf("paused status = %q, want held", updated.Status)
	}
	mustToolOK(t, cs, "claim_next", map[string]any{}, &claimed)
	if claimed.Claimed {
		t.Fatalf("claim_next served a paused task: %+v", claimed)
	}
	mustToolOK(t, cs, "update_task", map[string]any{"task": idea, "status": "open"}, &updated)
	if updated.Status != "open" {
		t.Fatalf("resumed status = %q, want open", updated.Status)
	}

	// The ledger: the capture landed as a v2 task.created carrying its
	// status — synced shared state, not a local nicety.
	events := flushedEvents(t, d)
	var capture *event.Event
	for i := range events {
		if events[i].Type == event.TypeTaskCreated && events[i].Task == idea {
			capture = &events[i]
		}
	}
	if capture == nil {
		t.Fatal("no task.created event for the capture")
	}
	if capture.V != 2 {
		t.Errorf("task.created v = %d, want 2 (old binaries must fail safe, not mis-bucket)", capture.V)
	}
	var p event.TaskCreated
	if err := json.Unmarshal(capture.Data, &p); err != nil {
		t.Fatalf("decode capture payload: %v", err)
	}
	if p.Status != "inbox" || p.Title != "idea: dark mode" {
		t.Errorf("capture payload = status %q title %q, want inbox / idea: dark mode", p.Status, p.Title)
	}
}

// Curation over the MCP surface (parity audit, 2026-07-31): every
// steering action a human asks for rides update_task fields, not verbs.
// Archive is update_task status cancelled — the task leaves every
// get_backlog array but stays readable by ID (nothing is deleted);
// retitle, redescribe, and reprioritize land field-wise leaving the
// unsent fields untouched; edge lists are full replacements, so an
// empty list is how an edge is removed.
func TestMCPCurationUpdates(t *testing.T) {
	d, _ := startDaemon(t)
	const actor = "brandon/impl-1"
	cs := mcpConnect(t, d, actor, nil)

	var created createTasksResult
	mustToolOK(t, cs, "create_task", map[string]any{
		"tasks": []map[string]any{
			{"title": "idea: stale capture", "status": "inbox"},
			{"title": "the epic", "description": "parent for edge tests"},
			{"title": "the dep", "description": "dependency for edge tests"},
			{"title": "worked task", "description": "original body", "priority": 1},
		},
	}, &created)
	if len(created.IDs) != 4 {
		t.Fatalf("create_task ids = %v, want four", created.IDs)
	}
	doomed, epic, dep, worked := created.IDs[0], created.IDs[1], created.IDs[2], created.IDs[3]

	// Archive: status cancelled through the ordinary update surface.
	var updated taskJSON
	mustToolOK(t, cs, "update_task", map[string]any{"task": doomed, "status": "cancelled"}, &updated)
	if updated.Status != "cancelled" {
		t.Fatalf("archived status = %q, want cancelled", updated.Status)
	}

	// The archived task appears in no backlog array…
	var backlog backlogResult
	mustToolOK(t, cs, "get_backlog", map[string]any{}, &backlog)
	for _, rows := range map[string][]taskJSON{
		"ready": backlog.Ready, "inbox": backlog.Inbox, "held": backlog.Held,
	} {
		for _, r := range rows {
			if r.ID == doomed {
				t.Fatalf("archived task %s still listed in %+v", doomed, rows)
			}
		}
	}
	// …but nothing is deleted: get_task still reads it by ID.
	var h hydratedTask
	mustToolOK(t, cs, "get_task", map[string]any{"task": doomed}, &h)
	if h.Task.Status != "cancelled" || h.Task.Title != "idea: stale capture" {
		t.Fatalf("get_task on archived = %+v, want cancelled with title intact", h.Task)
	}
	// And it can never be claimed.
	res := callTool(t, cs, "claim_task", map[string]any{"task": doomed})
	if !res.IsError || !strings.Contains(contentText(res), "status is cancelled") {
		t.Fatalf("claim_task(cancelled) = isError %v %q, want status-is-cancelled error", res.IsError, contentText(res))
	}

	// Retitle + reprioritize in one call: only the sent fields change.
	mustToolOK(t, cs, "update_task", map[string]any{
		"task": worked, "title": "renamed task", "priority": 5,
	}, &updated)
	if updated.Title != "renamed task" || updated.Priority != 5 {
		t.Fatalf("after retitle = title %q priority %d, want renamed task / 5", updated.Title, updated.Priority)
	}
	if updated.Description != "original body" || updated.Status != "open" {
		t.Fatalf("unsent fields changed: %+v", updated)
	}

	// Edges land as full replacements: add a parent and a dependency…
	mustToolOK(t, cs, "update_task", map[string]any{
		"task": worked, "parents": []string{epic}, "depends_on": []string{dep},
	}, &updated)
	if len(updated.Parents) != 1 || updated.Parents[0] != epic ||
		len(updated.DependsOn) != 1 || updated.DependsOn[0] != dep {
		t.Fatalf("edges = parents %v depends_on %v, want [%s] / [%s]", updated.Parents, updated.DependsOn, epic, dep)
	}
	// …which dep-blocks the task out of ready…
	mustToolOK(t, cs, "get_backlog", map[string]any{}, &backlog)
	for _, r := range backlog.Ready {
		if r.ID == worked {
			t.Fatalf("dep-blocked task %s still in ready: %+v", worked, backlog.Ready)
		}
	}
	// …and an explicit empty list is the removal, leaving the parent
	// edge (unsent) untouched.
	mustToolOK(t, cs, "update_task", map[string]any{
		"task": worked, "depends_on": []string{},
	}, &updated)
	if len(updated.DependsOn) != 0 {
		t.Fatalf("depends_on after clearing = %v, want empty", updated.DependsOn)
	}
	if len(updated.Parents) != 1 || updated.Parents[0] != epic {
		t.Fatalf("parents after clearing depends_on = %v, want still [%s]", updated.Parents, epic)
	}

	// An edge naming an unknown task is rejected whole.
	res = callTool(t, cs, "update_task", map[string]any{
		"task": worked, "parents": []string{"tuh-01NOPE"},
	})
	if !res.IsError || !strings.Contains(contentText(res), "unknown task") {
		t.Fatalf("bad edge = isError %v %q, want unknown-task tool error", res.IsError, contentText(res))
	}
}

// relay_answer (T5, 2026-07-30 revision): an agent records an
// out-of-band answer; the event's actor is the agent, the payload
// attributes the answer to the session's root principal, and the task
// unblocks exactly as a steering answer would unblock it.
func TestMCPRelayAnswer(t *testing.T) {
	d, hc := startDaemon(t)
	task := createOne(t, hc, "brandon", map[string]any{"title": "needs a decision"})

	const agent = "brandon/impl-1"
	cs := mcpConnect(t, d, agent, nil)

	// The blocking protocol, up to the point the human answers in chat:
	// claim, escalate blocking, release.
	var claimed claimNextResult
	mustToolOK(t, cs, "claim_next", map[string]any{}, &claimed)
	if !claimed.Claimed {
		t.Fatalf("claim_next = %+v, want a claim", claimed)
	}
	var raised eventIDResult
	mustToolOK(t, cs, "escalate", map[string]any{
		"task": task, "question": "which auth flow?", "blocking": true,
	}, &raised)
	mustToolOK(t, cs, "release_claim", map[string]any{
		"task": task, "reason": "blocked on the auth-flow question",
	}, nil)

	// Blocked: the pool has nothing to serve.
	var empty claimNextResult
	mustToolOK(t, cs, "claim_next", map[string]any{}, &empty)
	if empty.Claimed {
		t.Fatalf("claim_next served a task with an open blocking escalation: %+v", empty)
	}

	// The out-of-band answer lands through the agent.
	mustToolOK(t, cs, "relay_answer", map[string]any{
		"escalation": raised.ID, "answer": "oauth, device flow",
	}, nil)

	// Attribution: answered by the root principal, relayed by the agent.
	d.mu.Lock()
	esc := d.state.Escalations[raised.ID]
	d.mu.Unlock()
	if esc == nil || !esc.Answered {
		t.Fatalf("escalation after relay = %+v, want answered", esc)
	}
	if esc.AnsweredBy != "brandon" || esc.RelayedBy != agent {
		t.Fatalf("answered_by=%q relayed_by=%q, want brandon / %s", esc.AnsweredBy, esc.RelayedBy, agent)
	}

	// Readiness unblocks exactly like a TUI answer: the pool serves the
	// task again.
	var again claimNextResult
	mustToolOK(t, cs, "claim_next", map[string]any{}, &again)
	if !again.Claimed || again.Task.Task.ID != task {
		t.Fatalf("claim_next after relay = %+v, want %s back in the pool", again, task)
	}
	mustToolOK(t, cs, "release_claim", map[string]any{"task": task, "reason": "test done"}, nil)

	// The ledger: the answered event carries the agent on the envelope
	// and the root in the payload — the scribe/decider split on record.
	events := flushedEvents(t, d)
	var found *event.Event
	for i := range events {
		if events[i].Type == event.TypeEscalationAnswered {
			found = &events[i]
		}
	}
	if found == nil {
		t.Fatal("no escalation.answered event on the ledger")
	}
	if found.Actor != agent {
		t.Errorf("answered event actor = %q, want the relaying agent %s", found.Actor, agent)
	}
	var p event.EscalationAnswered
	if err := json.Unmarshal(found.Data, &p); err != nil {
		t.Fatalf("decode answered payload: %v", err)
	}
	if p.AnsweredBy != "brandon" {
		t.Errorf("payload answered_by = %q, want brandon", p.AnsweredBy)
	}

	// Guard rails: a settled answer cannot be re-relayed…
	res := callTool(t, cs, "relay_answer", map[string]any{
		"escalation": raised.ID, "answer": "second thoughts",
	})
	if !res.IsError || !strings.Contains(contentText(res), "already answered") {
		t.Fatalf("re-relay = isError %v %q, want already-answered tool error", res.IsError, contentText(res))
	}
	// …and an unknown escalation is a tool error, not a protocol one.
	res = callTool(t, cs, "relay_answer", map[string]any{
		"escalation": "01NOPE", "answer": "into the void",
	})
	if !res.IsError || !strings.Contains(contentText(res), "unknown escalation") {
		t.Fatalf("unknown relay = isError %v %q, want unknown-escalation tool error", res.IsError, contentText(res))
	}

	// Non-blocking escalations relay too, and a root-principal session
	// attributes to itself with no relay marker.
	rootCS := mcpConnect(t, d, "brandon", nil)
	var fyi eventIDResult
	mustToolOK(t, rootCS, "escalate", map[string]any{
		"task": task, "question": "rename the flag later?", "blocking": false,
	}, &fyi)
	mustToolOK(t, rootCS, "relay_answer", map[string]any{
		"escalation": fyi.ID, "answer": "yes, in v1",
	}, nil)
	d.mu.Lock()
	e2 := d.state.Escalations[fyi.ID]
	d.mu.Unlock()
	if e2 == nil || !e2.Answered || e2.AnsweredBy != "brandon" || e2.RelayedBy != "" {
		t.Fatalf("root-session relay = %+v, want answered by brandon with no relay marker", e2)
	}
}

// A session with a missing or malformed principal never comes up.
func TestMCPRejectsInvalidActor(t *testing.T) {
	d, _ := startDaemon(t)
	for _, actor := range []string{"", "a/b/c", "bad actor"} {
		client := mcp.NewClient(&mcp.Implementation{Name: "test-agent", Version: "0"}, nil)
		cs, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
			Endpoint: "http://tuhdoo/mcp",
			HTTPClient: &http.Client{Transport: mcpTestTransport{
				base:  &http.Transport{DialContext: (&killableDialer{socket: d.SocketPath()}).DialContext},
				actor: actor,
			}},
			MaxRetries: -1,
		}, nil)
		if err == nil {
			cs.Close()
			t.Fatalf("connect with actor %q succeeded, want rejection", actor)
		}
	}
}

// Accept 2: hard connection drop — no DELETE, no close — stops lease
// renewal; within the TTL the task is back in the pool and replay has
// synthesized an interrupted run for the orphaned claim.
func TestMCPConnectionDropExpiresLease(t *testing.T) {
	const ttl = 2 * time.Second
	d, hc := startDaemonOpts(t, Options{
		Quiet:        50 * time.Millisecond,
		LeaseTTL:     ttl,
		MCPKeepAlive: 100 * time.Millisecond,
		Log:          log.New(io.Discard, "", 0),
	})

	task := createOne(t, hc, "brandon", map[string]any{"title": "doomed to interruption"})

	dialer := &killableDialer{socket: d.SocketPath()}
	cs := mcpConnect(t, d, "brandon/impl-2", &http.Transport{DialContext: dialer.DialContext})

	var claimed claimNextResult
	mustToolOK(t, cs, "claim_next", map[string]any{}, &claimed)
	if !claimed.Claimed || claimed.Task == nil || claimed.Task.Claim == nil {
		t.Fatalf("claim_next = %+v, want a claim", claimed)
	}
	claimID := claimed.Task.Claim.ID

	// Sever every connection. The SDK client cannot answer keepalive
	// pings or reconnect; the daemon's pings fail, the session closes,
	// renewals stop, and the lease runs out.
	dropped := time.Now()
	dialer.kill()

	// One renewal tick (TTL/3) may land before the keepalive verdict,
	// so allow TTL past that plus scheduling slack.
	deadline := dropped.Add(ttl + ttl/3 + 2*time.Second)
	for {
		if err := d.Refresh(); err != nil {
			t.Fatalf("Refresh: %v", err)
		}
		d.mu.Lock()
		ready := d.state.Ready(task)
		var interrupted bool
		for i := range d.state.Runs {
			r := &d.state.Runs[i]
			if r.Task == task && r.Claim == claimID &&
				r.Outcome == event.OutcomeInterrupted && r.Synthesized {
				interrupted = true
			}
		}
		d.mu.Unlock()
		if ready && interrupted {
			break // the task is claimable again, the attempt is on record
		}
		if time.Now().After(deadline) {
			t.Fatalf("task not returned to pool within %v of the drop (ready=%v interrupted=%v)",
				time.Since(dropped), ready, interrupted)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// While the session lives, the daemon renews the lease on its own —
// with a short TTL, only auto-renewal can keep the claim active past
// several expiry windows (T5: no heartbeat verb).
func TestMCPSessionAutoRenewsLease(t *testing.T) {
	const ttl = time.Second
	d, hc := startDaemonOpts(t, Options{
		Quiet:        50 * time.Millisecond,
		LeaseTTL:     ttl,
		MCPKeepAlive: 100 * time.Millisecond,
		Log:          log.New(io.Discard, "", 0),
	})

	task := createOne(t, hc, "brandon", map[string]any{"title": "long haul"})
	cs := mcpConnect(t, d, "brandon/impl-3", nil)

	var claimed claimNextResult
	mustToolOK(t, cs, "claim_next", map[string]any{}, &claimed)
	if !claimed.Claimed {
		t.Fatalf("claim_next = %+v, want a claim", claimed)
	}

	time.Sleep(3 * ttl) // three expiry windows, zero agent calls
	if err := d.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	d.mu.Lock()
	c := d.state.ActiveClaim(task)
	d.mu.Unlock()
	if c == nil || c.Actor != "brandon/impl-3" {
		t.Fatalf("claim after %v = %+v, want still held by brandon/impl-3", 3*ttl, c)
	}
}

// Accept 3: a 15-task DAG batch with tmp refs lands atomically with
// edges resolved to real IDs; a batch with one bad ref lands nothing.
func TestMCPBatchDAGAtomic(t *testing.T) {
	d, _ := startDaemon(t)
	cs := mcpConnect(t, d, "brandon", nil)

	// One epic, seven children of it, and a seven-deep dependency chain
	// hanging off the last child: 15 tasks, every edge a tmp ref.
	batch := []map[string]any{{"tmp": "epic", "title": "the epic"}}
	for i := 1; i <= 7; i++ {
		batch = append(batch, map[string]any{
			"tmp":     fmt.Sprintf("child-%d", i),
			"title":   fmt.Sprintf("child %d", i),
			"parents": []string{"tmp:epic"},
		})
	}
	prev := "tmp:child-7"
	for i := 1; i <= 7; i++ {
		name := fmt.Sprintf("step-%d", i)
		batch = append(batch, map[string]any{
			"tmp":        name,
			"title":      fmt.Sprintf("step %d", i),
			"depends_on": []string{prev},
		})
		prev = "tmp:" + name
	}

	var created createTasksResult
	mustToolOK(t, cs, "create_task", map[string]any{"tasks": batch}, &created)
	if len(created.IDs) != 15 || len(created.Tmp) != 15 {
		t.Fatalf("got %d ids, %d tmp mappings, want 15 and 15", len(created.IDs), len(created.Tmp))
	}

	// Spot-check resolved edges through the MCP surface itself.
	var child hydratedTask
	mustToolOK(t, cs, "get_task", map[string]any{"task": created.Tmp["child-3"]}, &child)
	if len(child.Task.Parents) != 1 || child.Task.Parents[0] != created.Tmp["epic"] {
		t.Fatalf("child-3 parents = %v, want [%s]", child.Task.Parents, created.Tmp["epic"])
	}
	var step hydratedTask
	mustToolOK(t, cs, "get_task", map[string]any{"task": created.Tmp["step-2"]}, &step)
	if len(step.Task.DependsOn) != 1 || step.Task.DependsOn[0] != created.Tmp["step-1"] {
		t.Fatalf("step-2 depends_on = %v, want [%s]", step.Task.DependsOn, created.Tmp["step-1"])
	}

	if n := len(flushedEvents(t, d)); n != 15 {
		t.Fatalf("event count = %d, want 15", n)
	}

	// One bad ref poisons the whole batch: nothing lands.
	res := callTool(t, cs, "create_task", map[string]any{"tasks": []map[string]any{
		{"title": "fine"},
		{"title": "poisoned", "depends_on": []string{"tmp:missing"}},
	}})
	if !res.IsError || !strings.Contains(contentText(res), "unknown tmp ref") {
		t.Fatalf("bad batch = isError %v %q, want unknown-tmp-ref tool error", res.IsError, contentText(res))
	}
	if n := len(flushedEvents(t, d)); n != 15 {
		t.Fatalf("event count after rejected batch = %d, want still 15", n)
	}
}

// Auto-minted principals (D7): when the shim forwards the harness's
// clientInfo.name, the daemon completes the principal at session bind
// as <human>/<client>-<n> — and two concurrent sessions from the same
// human and harness get distinct principals.
func TestMCPMintsDistinctSessionPrincipals(t *testing.T) {
	d, _ := startDaemon(t)
	cs1, err := mcpTryConnect(d, "brandon", "Claude Code")
	if err != nil {
		t.Fatalf("connect session 1: %v", err)
	}
	defer cs1.Close()
	cs2, err := mcpTryConnect(d, "brandon", "Claude Code")
	if err != nil {
		t.Fatalf("connect session 2: %v", err)
	}
	defer cs2.Close()

	actorOf := func(cs *mcp.ClientSession) string {
		t.Helper()
		var created struct {
			IDs []string `json:"ids"`
		}
		mustToolOK(t, cs, "create_task",
			map[string]any{"tasks": []map[string]any{{"title": "who am I"}}}, &created)
		var h hydratedTask
		mustToolOK(t, cs, "get_task", map[string]any{"task": created.IDs[0]}, &h)
		return h.Task.CreatedBy
	}
	a1, a2 := actorOf(cs1), actorOf(cs2)
	if a1 != "brandon/claude-code-1" {
		t.Errorf("session 1 principal = %q, want brandon/claude-code-1", a1)
	}
	if a2 != "brandon/claude-code-2" {
		t.Errorf("session 2 principal = %q, want brandon/claude-code-2", a2)
	}
	if a1 == a2 {
		t.Errorf("concurrent sessions share principal %q", a1)
	}
}

// Session-bind rejections: minting requires a valid root human; a full
// agent principal or a garbage human fails the connect, loudly, at the
// door.
func TestMCPMintRejectsBadHumans(t *testing.T) {
	d, _ := startDaemon(t)
	for _, tt := range []struct{ actor, client string }{
		{"brandon/impl-1", "claude-code"}, // agent principal cannot take a minted suffix
		{"", "claude-code"},               // empty human
		{"two words", "claude-code"},      // invalid principal
	} {
		if cs, err := mcpTryConnect(d, tt.actor, tt.client); err == nil {
			cs.Close()
			t.Errorf("actor %q + client %q connected; want rejection", tt.actor, tt.client)
		}
	}
}

func TestSanitizeAgentName(t *testing.T) {
	tests := []struct{ in, want string }{
		{"claude-code", "claude-code"},
		{"Claude Code", "claude-code"},
		{"  spaced   out  ", "spaced-out"},
		{"weird!!name", "weird-name"},
		{"v2.1_beta", "v2.1_beta"},
		{"///", "agent"},
		{"", "agent"},
		{"ünïcode", "n-code"},
	}
	for _, tt := range tests {
		if got := sanitizeAgentName(tt.in); got != tt.want {
			t.Errorf("sanitizeAgentName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
