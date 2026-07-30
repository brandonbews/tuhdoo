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

// mcpTestTransport injects the actor header on every request, like the
// shim's actorTransport.
type mcpTestTransport struct {
	base  http.RoundTripper
	actor string
}

func (t mcpTestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	if t.actor != "" {
		req.Header.Set("X-Tuhdoo-Actor", t.actor)
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

// mcpTools is the T5 surface: exactly these ten, nothing else.
var mcpTools = []string{
	"add_note", "claim_next", "claim_task", "create_task", "escalate",
	"finish_run", "get_backlog", "get_task", "release_claim", "update_task",
}

// Accept 1: a scripted client runs the full loop — create, claim_next,
// add_note, finish_run — and the ledger holds exactly those events with
// the session principal stamped on every one.
func TestMCPFullLoop(t *testing.T) {
	d, _ := startDaemon(t)
	const actor = "brandon/impl-1"
	cs := mcpConnect(t, d, actor, nil)

	// The surface is exactly the ten T5 verbs.
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
