package daemon

// Integration tests: real git repositories, a real daemon, real HTTP
// over the real unix socket. Repo setup follows internal/store's
// pattern; requests go through an http.Client whose transport dials the
// daemon's socket.

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brandonbews/tuhdoo/internal/event"
	"github.com/brandonbews/tuhdoo/internal/gitx"
	"github.com/brandonbews/tuhdoo/internal/store"
	"github.com/brandonbews/tuhdoo/internal/views"
)

var testIdent = gitx.Identity{Name: "Test Bot", Email: "bot@example.com"}

func setGitEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_AUTHOR_NAME", testIdent.Name)
	t.Setenv("GIT_AUTHOR_EMAIL", testIdent.Email)
	t.Setenv("GIT_COMMITTER_NAME", testIdent.Name)
	t.Setenv("GIT_COMMITTER_EMAIL", testIdent.Email)
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// shortTempDir returns a temp directory whose absolute path stays short
// enough for a unix socket bound inside it: macOS caps sun_path at 104
// bytes, and t.TempDir() paths routinely blow past that.
func shortTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "tuhdoo")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// startDaemon builds a fresh repo, starts a daemon on it, and returns
// it with a client that speaks HTTP over the daemon's unix socket.
func startDaemon(t *testing.T) (*Daemon, *http.Client) {
	t.Helper()
	return startDaemonOpts(t, Options{
		Quiet: 50 * time.Millisecond,
		Log:   log.New(io.Discard, "", 0),
	})
}

// startDaemonOpts is startDaemon with explicit Options, for tests that
// shorten lease TTLs or keepalive intervals.
func startDaemonOpts(t *testing.T, opts Options) (*Daemon, *http.Client) {
	t.Helper()
	setGitEnv(t)
	root := shortTempDir(t)
	runGit(t, root, "init", "--quiet", "-b", "main")
	return startDaemonAt(t, root, opts)
}

// startDaemonAt starts a daemon on an existing repository — for tests
// that wire remotes or run multiple daemons around one bare repo
// (gate_test.go). Callers own the git setup.
func startDaemonAt(t *testing.T, root string, opts Options) (*Daemon, *http.Client) {
	t.Helper()
	d, err := New(root, opts)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	go d.Run()
	t.Cleanup(func() { d.Shutdown("test cleanup") })
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var nd net.Dialer
				return nd.DialContext(ctx, "unix", d.SocketPath())
			},
		},
	}
	return d, client
}

// do performs one API request. It never touches t, so it is safe from
// any goroutine.
func do(c *http.Client, method, path, actor string, body any) (int, []byte, error) {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, "http://tuhdoo"+path, rd)
	if err != nil {
		return 0, nil, err
	}
	if actor != "" {
		req.Header.Set("X-Tuhdoo-Actor", actor)
	}
	resp, err := c.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	return resp.StatusCode, data, err
}

// mustDo performs a request and fails the test unless it returns
// wantStatus. Returns the response body.
func mustDo(t *testing.T, c *http.Client, method, path, actor string, body any, wantStatus int) []byte {
	t.Helper()
	status, data, err := do(c, method, path, actor, body)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	if status != wantStatus {
		t.Fatalf("%s %s: status %d, want %d; body: %s", method, path, status, wantStatus, data)
	}
	return data
}

func unmarshalInto(t *testing.T, data []byte, dst any) {
	t.Helper()
	if err := json.Unmarshal(data, dst); err != nil {
		t.Fatalf("unmarshal %s: %v", data, err)
	}
}

// createOne creates a single task and returns its ID.
func createOne(t *testing.T, c *http.Client, actor string, item map[string]any) string {
	t.Helper()
	var resp struct {
		IDs []string `json:"ids"`
	}
	unmarshalInto(t, mustDo(t, c, "POST", "/v0/tasks", actor, []map[string]any{item}, http.StatusOK), &resp)
	if len(resp.IDs) != 1 {
		t.Fatalf("created %d tasks, want 1", len(resp.IDs))
	}
	return resp.IDs[0]
}

// Test 1: two concurrent clients hammering task creation produce a
// linear, gap-free event history — no lost writes, all IDs distinct.
func TestConcurrentCreatesLinearHistory(t *testing.T) {
	d, c := startDaemon(t)

	const clients, each = 2, 25
	errCh := make(chan error, clients*each)
	var wg sync.WaitGroup
	for g := 0; g < clients; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			actor := fmt.Sprintf("brandon/agent-%d", g)
			for i := 0; i < each; i++ {
				body := []map[string]any{{"title": fmt.Sprintf("task %d-%d", g, i)}}
				status, data, err := do(c, "POST", "/v0/tasks", actor, body)
				if err != nil {
					errCh <- err
				} else if status != http.StatusOK {
					errCh <- fmt.Errorf("status %d: %s", status, data)
				}
			}
		}(g)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("create: %v", err)
	}

	if err := d.batcher.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	events, err := d.store.LoadEvents()
	if err != nil {
		t.Fatalf("LoadEvents: %v", err)
	}
	if len(events) != clients*each {
		t.Fatalf("event count = %d, want %d", len(events), clients*each)
	}
	seen := make(map[string]bool)
	for _, e := range events {
		if seen[e.ID] {
			t.Fatalf("duplicate event id %s", e.ID)
		}
		seen[e.ID] = true
		if e.Type != event.TypeTaskCreated {
			t.Fatalf("unexpected event type %s", e.Type)
		}
	}
}

// Test 2: the claim lifecycle over the API — claim_next hydrates and
// leases, renewal extends, holder checks bite, release returns the
// task to the pool.
// Inbox and held (2026-07-31): captures and paused tasks are ordinary
// shared state, but the claim verbs never serve them — open is the only
// claimable status. Title-only capture works; promote/pause/resume
// round-trips run through the ordinary update surface.
func TestClaimNeverServesInboxOrHeld(t *testing.T) {
	_, c := startDaemon(t)

	// Title-only capture: the minimum the capture tier demands.
	idea := createOne(t, c, "brandon", map[string]any{"title": "idea: dark mode", "status": "inbox"})
	parked := createOne(t, c, "brandon", map[string]any{"title": "polish docs", "status": "held", "priority": 9})

	// Born-terminal creates are caller mistakes.
	mustDo(t, c, "POST", "/v0/tasks", "brandon",
		[]map[string]any{{"title": "stillborn", "status": "done"}}, http.StatusBadRequest)
	mustDo(t, c, "POST", "/v0/tasks", "brandon",
		[]map[string]any{{"title": "nonsense", "status": "someday"}}, http.StatusBadRequest)

	// claim_next with only shelved tasks: the pool is empty — even
	// though the held task carries the highest priority on the board.
	mustDo(t, c, "POST", "/v0/claims", "brandon/a1", map[string]any{"next": true}, http.StatusConflict)

	// claim_task on a shelf is a conflict that names the status.
	body := mustDo(t, c, "POST", "/v0/claims", "brandon/a1", map[string]any{"task": idea}, http.StatusConflict)
	if !strings.Contains(string(body), "status is inbox") {
		t.Fatalf("claim of inbox task: %s, want a status-is-inbox conflict", body)
	}
	body = mustDo(t, c, "POST", "/v0/claims", "brandon/a1", map[string]any{"task": parked}, http.StatusConflict)
	if !strings.Contains(string(body), "status is held") {
		t.Fatalf("claim of held task: %s, want a status-is-held conflict", body)
	}

	// Promote the capture (description supplied in the same breath, as
	// the protocol asks) and pause/resume the other: ordinary updates.
	mustDo(t, c, "PATCH", "/v0/tasks/"+idea, "brandon",
		map[string]any{"status": "open", "description": "Context, ask, acceptance."}, http.StatusOK)
	mustDo(t, c, "PATCH", "/v0/tasks/"+parked, "brandon", map[string]any{"status": "open"}, http.StatusOK)
	mustDo(t, c, "PATCH", "/v0/tasks/"+parked, "brandon", map[string]any{"status": "held"}, http.StatusOK)

	// Now the pool serves exactly the promoted task.
	var h hydratedTask
	unmarshalInto(t, mustDo(t, c, "POST", "/v0/claims", "brandon/a1",
		map[string]any{"next": true}, http.StatusOK), &h)
	if h.Task.ID != idea {
		t.Fatalf("claim_next served %s, want the promoted %s", h.Task.ID, idea)
	}
	mustDo(t, c, "POST", "/v0/claims", "brandon/a1", map[string]any{"next": true}, http.StatusConflict)

	// The stored task.created carries the status in its payload.
	var st stateResp
	unmarshalInto(t, mustDo(t, c, "GET", "/v0/state", "", nil, http.StatusOK), &st)
	for _, task := range st.Tasks {
		if task.ID == parked && task.Status != "held" {
			t.Fatalf("parked task status = %q, want held", task.Status)
		}
	}
}

// Close metadata rides both read payloads the TUI snapshot is built
// from (history view, 2026-08-02): the /v0/state listing and the
// single-task hydration. Open tasks carry none; a cancel stamps both;
// reopening clears both.
func TestStateAndHydrationCarryCloseMetadata(t *testing.T) {
	_, c := startDaemon(t)
	id := createOne(t, c, "brandon", map[string]any{"title": "short-lived"})
	stateOf := func() stateTask {
		var st stateResp
		unmarshalInto(t, mustDo(t, c, "GET", "/v0/state", "", nil, http.StatusOK), &st)
		for _, task := range st.Tasks {
			if task.ID == id {
				return task
			}
		}
		t.Fatalf("task %s missing from state", id)
		return stateTask{}
	}
	hydrationOf := func() taskJSON {
		var h hydratedTask
		unmarshalInto(t, mustDo(t, c, "GET", "/v0/tasks/"+id, "", nil, http.StatusOK), &h)
		return h.Task
	}

	// Open: no close metadata anywhere.
	if st := stateOf(); st.ClosedAt != nil || st.ClosedBy != "" {
		t.Fatalf("open task's state row carries close metadata: %+v", st)
	}
	if hy := hydrationOf(); hy.ClosedAt != nil || hy.ClosedBy != "" {
		t.Fatalf("open task's hydration carries close metadata: %+v", hy)
	}

	// Cancelled: both payloads stamp the close, by the cancelling actor.
	mustDo(t, c, "PATCH", "/v0/tasks/"+id, "brandon", map[string]any{"status": "cancelled"}, http.StatusOK)
	st := stateOf()
	if st.ClosedAt == nil || st.ClosedBy != "brandon" {
		t.Fatalf("cancelled state row = %+v, want a close stamp by brandon", st)
	}
	hy := hydrationOf()
	if hy.ClosedAt == nil || hy.ClosedBy != "brandon" {
		t.Fatalf("cancelled hydration = %+v, want a close stamp by brandon", hy)
	}
	if !hy.ClosedAt.Equal(*st.ClosedAt) {
		t.Errorf("hydration close %v != state close %v", hy.ClosedAt, st.ClosedAt)
	}

	// Reopened: the stamp clears from both.
	mustDo(t, c, "PATCH", "/v0/tasks/"+id, "brandon", map[string]any{"status": "open"}, http.StatusOK)
	if st := stateOf(); st.ClosedAt != nil || st.ClosedBy != "" {
		t.Fatalf("reopened task's state row still closed: %+v", st)
	}
	if hy := hydrationOf(); hy.ClosedAt != nil || hy.ClosedBy != "" {
		t.Fatalf("reopened task's hydration still closed: %+v", hy)
	}
}

// One classifier (tuh-01KZ0ES83SFH6MKWP82YRXWQD6): /v0/state serves
// core's verdict per task — situation always present (ready /
// in_progress / blocked for open tasks, the status word otherwise) and
// the blocker ID lists for every task regardless of status — so no
// client re-derives the rules.
func TestStateServesTheDerivedSituation(t *testing.T) {
	_, c := startDaemon(t)

	unblocked := createOne(t, c, "brandon", map[string]any{"title": "ready and waiting"})
	dep := createOne(t, c, "brandon", map[string]any{"title": "the unfinished dep"})
	depBlocked := createOne(t, c, "brandon", map[string]any{
		"title": "dep-blocked", "depends_on": []string{dep}})
	escBlocked := createOne(t, c, "brandon", map[string]any{"title": "escalation-blocked"})
	// A shelved task with an unmet dep: the lists ride regardless of
	// status (core.ClaimBlockers' contract), while situation stays the
	// status word.
	captured := createOne(t, c, "brandon", map[string]any{
		"title": "idea: dark mode", "status": "inbox", "depends_on": []string{dep}})

	var esc struct {
		ID string `json:"id"`
	}
	unmarshalInto(t, mustDo(t, c, "POST", "/v0/escalations", "brandon/a1",
		map[string]any{"task": escBlocked, "question": "which way?", "blocking": true},
		http.StatusOK), &esc)
	mustDo(t, c, "POST", "/v0/claims", "brandon/a1", map[string]any{"task": dep}, http.StatusOK)

	var st stateResp
	unmarshalInto(t, mustDo(t, c, "GET", "/v0/state", "", nil, http.StatusOK), &st)
	rows := make(map[string]stateTask, len(st.Tasks))
	for _, task := range st.Tasks {
		rows[task.ID] = task
	}
	want := map[string]stateTask{
		unblocked:  {Situation: "ready"},
		dep:        {Situation: "in_progress"},
		depBlocked: {Situation: "blocked", UnmetDeps: []string{dep}},
		escBlocked: {Situation: "blocked", BlockingEscalations: []string{esc.ID}},
		captured:   {Situation: "inbox", UnmetDeps: []string{dep}},
	}
	for id, w := range want {
		got, ok := rows[id]
		if !ok {
			t.Fatalf("task %s missing from state", id)
		}
		if got.Situation != w.Situation {
			t.Errorf("%s situation = %q, want %q", got.Title, got.Situation, w.Situation)
		}
		if !reflect.DeepEqual(got.UnmetDeps, w.UnmetDeps) {
			t.Errorf("%s unmet_deps = %v, want %v", got.Title, got.UnmetDeps, w.UnmetDeps)
		}
		if !reflect.DeepEqual(got.BlockingEscalations, w.BlockingEscalations) {
			t.Errorf("%s blocking_escalations = %v, want %v", got.Title, got.BlockingEscalations, w.BlockingEscalations)
		}
	}
}

// claim_task's not-ready conflict names the actual blockers
// (tuh-01KYWKT8NQ980F0NF4MN3VMT0Y): the open blocking escalation's ID
// for an escalation-blocked task — never the old catch-all "unmet
// dependencies" — the unmet dep IDs for a dep-blocked one, and both
// when both hold.
func TestClaimTaskConflictNamesBlockers(t *testing.T) {
	_, c := startDaemon(t)

	dep := createOne(t, c, "brandon", map[string]any{"title": "the unfinished dep"})
	depBlocked := createOne(t, c, "brandon", map[string]any{
		"title": "dep-blocked", "depends_on": []string{dep}})
	escBlocked := createOne(t, c, "brandon", map[string]any{"title": "escalation-blocked"})
	dual := createOne(t, c, "brandon", map[string]any{
		"title": "double-blocked", "depends_on": []string{dep}})

	raise := func(task string) string {
		t.Helper()
		var resp struct {
			ID string `json:"id"`
		}
		unmarshalInto(t, mustDo(t, c, "POST", "/v0/escalations", "brandon/a1",
			map[string]any{"task": task, "question": "which way?", "blocking": true},
			http.StatusOK), &resp)
		return resp.ID
	}
	escID := raise(escBlocked)
	dualEscID := raise(dual)

	// Escalation-only: the conflict names the escalation so the caller
	// can act on it, and the old misdiagnosis stays dead.
	body := string(mustDo(t, c, "POST", "/v0/claims", "brandon/a2",
		map[string]any{"task": escBlocked}, http.StatusConflict))
	if !strings.Contains(body, "blocked by open escalation "+escID) {
		t.Errorf("escalation-blocked conflict does not name the escalation: %s", body)
	}
	if strings.Contains(body, "unmet dependencies") {
		t.Errorf("escalation-blocked conflict still says unmet dependencies: %s", body)
	}

	// Deps-only names the dep IDs and invents no escalation.
	body = string(mustDo(t, c, "POST", "/v0/claims", "brandon/a2",
		map[string]any{"task": depBlocked}, http.StatusConflict))
	if !strings.Contains(body, "unmet dependencies "+dep) {
		t.Errorf("dep-blocked conflict does not name the dep: %s", body)
	}
	if strings.Contains(body, "escalation") {
		t.Errorf("dep-blocked conflict invents an escalation: %s", body)
	}

	// Both at once: both named.
	body = string(mustDo(t, c, "POST", "/v0/claims", "brandon/a2",
		map[string]any{"task": dual}, http.StatusConflict))
	for _, want := range []string{
		"unmet dependencies " + dep,
		"blocked by open escalation " + dualEscID,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("double-blocked conflict missing %q: %s", want, body)
		}
	}
}

func TestClaimLifecycle(t *testing.T) {
	d, c := startDaemon(t)

	high := createOne(t, c, "brandon", map[string]any{"title": "urgent", "priority": 5})
	low := createOne(t, c, "brandon", map[string]any{"title": "later", "priority": 1})

	// claim_next: highest priority first, hydrated, with a lease.
	var h hydratedTask
	unmarshalInto(t, mustDo(t, c, "POST", "/v0/claims", "brandon/a1", map[string]any{"next": true}, http.StatusOK), &h)
	if h.Task.ID != high {
		t.Fatalf("claim_next picked %s, want %s", h.Task.ID, high)
	}
	if h.Claim == nil || h.Claim.Actor != "brandon/a1" || h.Claim.Expires == nil {
		t.Fatalf("hydrated claim = %+v, want active claim by brandon/a1 with expiry", h.Claim)
	}
	leases, err := d.store.ReadLeases()
	if err != nil {
		t.Fatalf("ReadLeases: %v", err)
	}
	orig, ok := leases[h.Claim.ID]
	if !ok {
		t.Fatalf("no lease written for claim %s", h.Claim.ID)
	}

	// Renewal by a non-holder is forbidden.
	mustDo(t, c, "POST", "/v0/claims/renew", "brandon/a2", map[string]any{"task": high}, http.StatusForbidden)

	// Renewal by the holder extends the lease. Lease expiry is stored
	// at second precision, so cross a second boundary to observe it.
	time.Sleep(1100 * time.Millisecond)
	var renewed struct {
		Claim   string    `json:"claim"`
		Expires time.Time `json:"expires"`
	}
	unmarshalInto(t, mustDo(t, c, "POST", "/v0/claims/renew", "brandon/a1", map[string]any{"task": high}, http.StatusOK), &renewed)
	if renewed.Claim != h.Claim.ID {
		t.Fatalf("renewed claim %s, want %s", renewed.Claim, h.Claim.ID)
	}
	if !renewed.Expires.After(orig) {
		t.Fatalf("renewed expiry %v is not after original %v", renewed.Expires, orig)
	}

	// Another actor's claim_next gets the other task.
	var h2 hydratedTask
	unmarshalInto(t, mustDo(t, c, "POST", "/v0/claims", "brandon/a2", map[string]any{"next": true}, http.StatusOK), &h2)
	if h2.Task.ID != low {
		t.Fatalf("second claim_next picked %s, want %s", h2.Task.ID, low)
	}

	// Pool exhausted: claim_next conflicts; claiming a held task names
	// the holder.
	mustDo(t, c, "POST", "/v0/claims", "brandon/a3", map[string]any{"next": true}, http.StatusConflict)
	status, data, err := do(c, "POST", "/v0/claims", "brandon/a3", map[string]any{"task": high})
	if err != nil || status != http.StatusConflict {
		t.Fatalf("claim of held task: status %d, err %v; body: %s", status, err, data)
	}
	if !strings.Contains(string(data), "brandon/a1") {
		t.Fatalf("conflict body should name the holder: %s", data)
	}

	// Release by a non-holder is forbidden; by the holder it returns
	// the task to the pool.
	mustDo(t, c, "DELETE", "/v0/claims", "brandon/a2", map[string]any{"task": high, "reason": "not mine"}, http.StatusForbidden)
	mustDo(t, c, "DELETE", "/v0/claims", "brandon/a1", map[string]any{"task": high, "reason": "standing down"}, http.StatusOK)
	var h3 hydratedTask
	unmarshalInto(t, mustDo(t, c, "POST", "/v0/claims", "brandon/a3", map[string]any{"task": high}, http.StatusOK), &h3)
	if h3.Claim == nil || h3.Claim.Actor != "brandon/a3" {
		t.Fatalf("reclaim after release: claim = %+v, want brandon/a3", h3.Claim)
	}
}

// Test 3: batch create with intra-batch tmp refs lands atomically with
// resolved real IDs; invalid batches create nothing.
func TestBatchCreateTmpRefs(t *testing.T) {
	d, c := startDaemon(t)

	batch := []map[string]any{
		{"tmp": "epic", "title": "the epic"},
		{"tmp": "a", "title": "a", "parents": []string{"tmp:epic"}},
		{"tmp": "b", "title": "b", "parents": []string{"tmp:epic"}, "depends_on": []string{"tmp:a"}},
		{"tmp": "c", "title": "c", "depends_on": []string{"tmp:a", "tmp:b"}},
		{"tmp": "d", "title": "d", "depends_on": []string{"tmp:c"}},
	}
	var resp struct {
		IDs []string          `json:"ids"`
		Tmp map[string]string `json:"tmp"`
	}
	unmarshalInto(t, mustDo(t, c, "POST", "/v0/tasks", "brandon", batch, http.StatusOK), &resp)
	if len(resp.IDs) != 5 || len(resp.Tmp) != 5 {
		t.Fatalf("got %d ids, %d tmp mappings, want 5 and 5", len(resp.IDs), len(resp.Tmp))
	}
	for name, id := range resp.Tmp {
		if !strings.HasPrefix(id, "tuh-") {
			t.Fatalf("tmp %q resolved to %q, want a tuh- id", name, id)
		}
	}

	// Refs resolved to real IDs.
	var hc hydratedTask
	unmarshalInto(t, mustDo(t, c, "GET", "/v0/tasks/"+resp.Tmp["c"], "", nil, http.StatusOK), &hc)
	want := []string{resp.Tmp["a"], resp.Tmp["b"]}
	if len(hc.Task.DependsOn) != 2 || hc.Task.DependsOn[0] != want[0] || hc.Task.DependsOn[1] != want[1] {
		t.Fatalf("c depends_on = %v, want %v", hc.Task.DependsOn, want)
	}

	// Atomic: all five task.created events land in one commit.
	if err := d.batcher.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	events, err := d.store.LoadEvents()
	if err != nil {
		t.Fatalf("LoadEvents: %v", err)
	}
	if len(events) != 5 {
		t.Fatalf("event count = %d, want 5", len(events))
	}
	count := strings.TrimSpace(runGit(t, d.root, "rev-list", "--count", store.DefaultRef))
	if count != "2" { // init root + one batch commit
		t.Fatalf("commit count = %s, want 2 (init + one atomic batch)", count)
	}

	// Unknown tmp ref: rejected atomically, zero tasks created.
	bad := []map[string]any{
		{"title": "x", "depends_on": []string{"tmp:missing"}},
		{"title": "y"},
	}
	mustDo(t, c, "POST", "/v0/tasks", "brandon", bad, http.StatusBadRequest)

	// Cyclic tmp refs: rejected.
	cyc := []map[string]any{
		{"tmp": "p", "title": "p", "depends_on": []string{"tmp:q"}},
		{"tmp": "q", "title": "q", "depends_on": []string{"tmp:p"}},
	}
	status, data, err := do(c, "POST", "/v0/tasks", "brandon", cyc)
	if err != nil || status != http.StatusBadRequest {
		t.Fatalf("cyclic batch: status %d, err %v; body: %s", status, err, data)
	}
	if !strings.Contains(string(data), "cyclic") {
		t.Fatalf("cyclic batch error should say so: %s", data)
	}

	if err := d.batcher.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	events, err = d.store.LoadEvents()
	if err != nil {
		t.Fatalf("LoadEvents: %v", err)
	}
	if len(events) != 5 {
		t.Fatalf("event count after rejected batches = %d, want still 5", len(events))
	}
}

// Test 4: a second daemon instance on the same repo fails cleanly,
// naming the live pid, while the first keeps running.
func TestSecondInstanceFails(t *testing.T) {
	d, c := startDaemon(t)

	_, err := New(d.root, Options{Log: log.New(io.Discard, "", 0)})
	if err == nil {
		t.Fatal("second New succeeded, want failure while the first daemon runs")
	}
	if !strings.Contains(err.Error(), strconv.Itoa(os.Getpid())) {
		t.Fatalf("error should name the live pid %d: %v", os.Getpid(), err)
	}

	// The first daemon is unharmed.
	mustDo(t, c, "GET", "/v0/state", "", nil, http.StatusOK)
}

// Test 5: T3 fail-safe — an incomprehensible event flips the daemon to
// read-only: writes 503 with the upgrade message, reads keep serving
// the last good state.
func TestFailSafeReadOnly(t *testing.T) {
	d, c := startDaemon(t)

	id := createOne(t, c, "brandon", map[string]any{"title": "survivor"})
	if err := d.batcher.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	// Inject an event from a "newer" daemon directly via the store.
	futureID, err := event.NewID(time.Now(), rand.Reader)
	if err != nil {
		t.Fatalf("NewID: %v", err)
	}
	future, err := event.New(futureID, "task.teleported", 1, "future/agent", "m-ffff", "t-x", map[string]any{})
	if err != nil {
		t.Fatalf("event.New: %v", err)
	}
	if err := d.store.AppendBatch(store.Batch{Events: []event.Event{future}}); err != nil {
		t.Fatalf("AppendBatch: %v", err)
	}
	if err := d.Refresh(); err == nil {
		t.Fatal("Refresh succeeded, want a fail-safe replay error")
	}

	// Writes: rejected 503 with the upgrade message.
	status, data, err := do(c, "POST", "/v0/notes", "brandon", map[string]any{"task": id, "text": "hello?"})
	if err != nil {
		t.Fatalf("note: %v", err)
	}
	if status != http.StatusServiceUnavailable {
		t.Fatalf("write in fail-safe mode: status %d, want 503; body: %s", status, data)
	}
	if !strings.Contains(string(data), "upgrade tuhdoo") {
		t.Fatalf("503 body should carry the upgrade message: %s", data)
	}

	// Reads: still serving the last good state.
	var st stateResp
	unmarshalInto(t, mustDo(t, c, "GET", "/v0/state", "", nil, http.StatusOK), &st)
	if st.Degraded == "" {
		t.Fatal("state should report degraded mode")
	}
	if len(st.Tasks) != 1 || st.Tasks[0].ID != id {
		t.Fatalf("state tasks = %+v, want the surviving task %s", st.Tasks, id)
	}
	mustDo(t, c, "GET", "/v0/tasks/"+id, "", nil, http.StatusOK)
}

// Clean shutdown removes the socket and discovery file and releases the
// lock so a successor can start.
func TestShutdownCleansUp(t *testing.T) {
	d, c := startDaemon(t)
	mustDo(t, c, "GET", "/v0/state", "", nil, http.StatusOK)

	sock, disc := d.sockPath, d.jsonPath
	d.Shutdown("test: clean shutdown")
	for _, p := range []string{sock, disc} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s still exists after shutdown (stat err: %v)", p, err)
		}
	}

	// The lock is free again: a successor daemon starts.
	d2, err := New(d.root, Options{Log: log.New(io.Discard, "", 0)})
	if err != nil {
		t.Fatalf("successor New: %v", err)
	}
	d2.Shutdown("test cleanup")
}

// Test 7: views ride local writes (T6). After ordinary daemon writes —
// no sync merge anywhere — the data branch carries the four rendered
// views alongside the events. The escalate call is eager, so this also
// proves staged views ride an eager flush, not just the debounce.
func TestViewsRideLocalWrites(t *testing.T) {
	d, c := startDaemon(t)

	id := createOne(t, c, "brandon", map[string]any{"title": "make views ride"})
	mustDo(t, c, "POST", "/v0/escalations", "brandon/a1",
		map[string]any{"task": id, "question": "do views ride?"}, http.StatusOK)

	checks := map[string]string{
		"README.md":           "",
		"backlog.md":          "make views ride",
		"escalations.md":      "do views ride?",
		"tasks/" + id + ".md": "make views ride",
		views.MetaPath:        "",
	}
	for path, want := range checks {
		data, err := d.store.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", path, err)
		}
		if data == nil {
			t.Errorf("%s missing from the data branch after local writes", path)
			continue
		}
		if want != "" && !strings.Contains(string(data), want) {
			t.Errorf("%s does not mention %q:\n%s", path, want, data)
		}
	}
}

// Test 8: highest version wins (B8). Views stamped by a newer generator
// are never overwritten by this daemon's local writes — events flow,
// views stay exactly as the newer peer left them.
func TestViewsGuardRefusesNewerStamp(t *testing.T) {
	d, c := startDaemon(t)

	newer := []byte("{\"format\":99}\n")
	err := d.store.AppendBatch(store.Batch{Files: map[string][]byte{views.MetaPath: newer}})
	if err != nil {
		t.Fatalf("stamping newer format: %v", err)
	}

	createOne(t, c, "brandon", map[string]any{"title": "guarded"})
	if err := d.batcher.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	events, err := d.store.LoadEvents()
	if err != nil {
		t.Fatalf("LoadEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("event count = %d, want 1 (events must still flow)", len(events))
	}
	meta, err := d.store.ReadFile(views.MetaPath)
	if err != nil {
		t.Fatalf("ReadFile(meta): %v", err)
	}
	if !bytes.Equal(meta, newer) {
		t.Errorf("meta = %q, want the newer peer's %q untouched", meta, newer)
	}
	if backlog, _ := d.store.ReadFile("backlog.md"); backlog != nil {
		t.Errorf("backlog.md written despite newer stamp:\n%s", backlog)
	}
}

// The overlay of process-written events carries a write only until a
// refresh sees it on the branch, then trims it — it must not grow for
// the process lifetime (t-01KYRMFV10W1N28TCN5ZZ9Z2C1). State must serve
// the write throughout: from the overlay before the flush, from the
// branch after the trim.
func TestOverlayTrimsAfterFlush(t *testing.T) {
	d, c := startDaemon(t)

	id := createOne(t, c, "brandon", map[string]any{"title": "trim me"})

	// Before the debounced flush lands, the event lives in the overlay
	// and state already serves it.
	d.mu.Lock()
	overlayBefore := len(d.written)
	_, inState := d.state.Tasks[id]
	d.mu.Unlock()
	if overlayBefore != 1 {
		t.Fatalf("overlay holds %d events before flush, want 1", overlayBefore)
	}
	if !inState {
		t.Fatalf("task %s not in state while its event is overlay-only", id)
	}

	// After the flush, the next refresh must trim the overlay to empty
	// while the task keeps being served, now from the branch.
	if err := d.batcher.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if err := d.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	d.mu.Lock()
	overlayAfter := len(d.written)
	_, inState = d.state.Tasks[id]
	d.mu.Unlock()
	if overlayAfter != 0 {
		t.Errorf("overlay holds %d events after flush + refresh, want 0", overlayAfter)
	}
	if !inState {
		t.Errorf("task %s vanished from state after overlay trim", id)
	}
}

// The finish_run guard (t-01KYVMD4PS9NMQVP1K5HQ8769X): a run.finished
// event only lands when the acting principal has an attempt of their
// own to close — the live claim, or their own released/voided claim not
// yet closed by a later run of theirs. Each row builds its own task and
// history through the ops, then attempts one finish.
func TestFinishRunGuard(t *testing.T) {
	d, c := startDaemon(t)

	claim := func(t *testing.T, actor, task string) claimJSON {
		t.Helper()
		h, oe := d.opClaimTask(actor, task)
		if oe != nil {
			t.Fatalf("claim %s as %s: %v", task, actor, oe)
		}
		return *h.Claim
	}
	release := func(t *testing.T, actor, task string) {
		t.Helper()
		if _, oe := d.opReleaseClaim(actor, task, "standing down"); oe != nil {
			t.Fatalf("release %s as %s: %v", task, actor, oe)
		}
	}
	finish := func(t *testing.T, actor, task, outcome string) *opError {
		t.Helper()
		_, oe := d.opFinishRun(actor, finishRunReq{Task: task, Outcome: outcome, Summary: "attempt closed"})
		return oe
	}
	// mintClaim writes a claim.made event directly, the way a peer's
	// claim arrives by sync — replay voids it when someone already holds.
	mintClaim := func(t *testing.T, actor, task string) {
		t.Helper()
		d.mu.Lock()
		ev, err := d.newEventLocked(event.TypeClaimMade, actor, task, event.ClaimMade{})
		if err == nil {
			err = d.commitLocked(false, ev)
		}
		d.mu.Unlock()
		if err != nil {
			t.Fatalf("mint claim by %s on %s: %v", actor, task, err)
		}
	}
	// taskRuns returns the task's runs as (non-synthesized, synthesized)
	// counts from freshly replayed state.
	taskRuns := func(t *testing.T, task string) (real, synth int) {
		t.Helper()
		if err := d.Refresh(); err != nil {
			t.Fatalf("Refresh: %v", err)
		}
		d.mu.Lock()
		defer d.mu.Unlock()
		for i := range d.state.Runs {
			if r := &d.state.Runs[i]; r.Task == task {
				if r.Synthesized {
					synth++
				} else {
					real++
				}
			}
		}
		return real, synth
	}

	tests := []struct {
		name      string
		setup     func(t *testing.T, task string)
		actor     string
		outcome   string
		noTask    bool // attempt against a task ID that does not exist
		wantOK    bool
		wantCode  int    // when !wantOK
		wantErr   string // substring of the rejection
		wantRuns  int    // non-synthesized runs on the task afterwards
		wantSynth int    // synthesized runs on the task afterwards
	}{
		{
			name:     "no claim on the task is rejected",
			actor:    "brandon/a1",
			outcome:  event.OutcomeDone,
			wantCode: http.StatusConflict,
			wantErr:  "no claim by brandon/a1",
		},
		{
			name: "live claim held by another principal is rejected",
			setup: func(t *testing.T, task string) {
				claim(t, "brandon/a1", task)
			},
			actor:    "brandon/a2",
			outcome:  event.OutcomeDone,
			wantCode: http.StatusForbidden,
			wantErr:  "held by brandon/a1",
		},
		{
			name: "the caller's own live claim succeeds",
			setup: func(t *testing.T, task string) {
				claim(t, "brandon/a1", task)
			},
			actor:    "brandon/a1",
			outcome:  event.OutcomeDone,
			wantOK:   true,
			wantRuns: 1,
		},
		{
			name: "blocked protocol: finish after own release succeeds",
			setup: func(t *testing.T, task string) {
				claim(t, "brandon/a1", task)
				release(t, "brandon/a1", task)
			},
			actor:    "brandon/a1",
			outcome:  event.OutcomeBlocked,
			wantOK:   true,
			wantRuns: 1,
		},
		{
			name: "a released attempt closes once, not twice",
			setup: func(t *testing.T, task string) {
				claim(t, "brandon/a1", task)
				release(t, "brandon/a1", task)
				if oe := finish(t, "brandon/a1", task, event.OutcomeBlocked); oe != nil {
					t.Fatalf("first finish after release: %v", oe)
				}
			},
			actor:    "brandon/a1",
			outcome:  event.OutcomeBlocked,
			wantCode: http.StatusConflict,
			wantErr:  "already closed",
			wantRuns: 1,
		},
		{
			name: "a finished attempt cannot be finished again",
			setup: func(t *testing.T, task string) {
				claim(t, "brandon/a1", task)
				if oe := finish(t, "brandon/a1", task, event.OutcomeDone); oe != nil {
					t.Fatalf("first finish: %v", oe)
				}
			},
			actor:    "brandon/a1",
			outcome:  event.OutcomeDone,
			wantCode: http.StatusConflict,
			wantErr:  "already closed",
			wantRuns: 1,
		},
		{
			name: "an expired claim is closed by its synthesized interrupted run",
			setup: func(t *testing.T, task string) {
				cl := claim(t, "brandon/a1", task)
				// A missing lease counts as lapsed (core replay), so
				// deleting it expires the claim deterministically.
				if err := d.store.DeleteLease(cl.ID); err != nil {
					t.Fatalf("DeleteLease: %v", err)
				}
			},
			actor:     "brandon/a1",
			outcome:   event.OutcomeDone,
			wantCode:  http.StatusConflict,
			wantErr:   "already closed",
			wantSynth: 1,
		},
		{
			name: "race loser with a voided claim records superseded while the winner holds",
			setup: func(t *testing.T, task string) {
				claim(t, "brandon/a1", task)
				mintClaim(t, "brandon/a2", task)
			},
			actor:    "brandon/a2",
			outcome:  event.OutcomeSuperseded,
			wantOK:   true,
			wantRuns: 1,
		},
		{
			name:     "a mistyped task ID fabricates nothing",
			actor:    "brandon/a1",
			outcome:  event.OutcomeDone,
			noTask:   true,
			wantCode: http.StatusNotFound,
			wantErr:  "unknown task",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			task := "t-00000000000000000000000000"
			if !tc.noTask {
				task = createOne(t, c, "brandon", map[string]any{"title": tc.name})
			}
			if tc.setup != nil {
				tc.setup(t, task)
			}
			oe := finish(t, tc.actor, task, tc.outcome)
			if tc.wantOK {
				if oe != nil {
					t.Fatalf("finish_run = %v, want success", oe)
				}
			} else {
				if oe == nil {
					t.Fatalf("finish_run succeeded, want rejection %d %q", tc.wantCode, tc.wantErr)
				}
				if oe.code != tc.wantCode || !strings.Contains(oe.msg, tc.wantErr) {
					t.Fatalf("finish_run = %d %q, want %d containing %q", oe.code, oe.msg, tc.wantCode, tc.wantErr)
				}
			}
			if tc.noTask {
				return
			}
			real, synth := taskRuns(t, task)
			if real != tc.wantRuns || synth != tc.wantSynth {
				t.Fatalf("runs after attempt = %d real, %d synthesized; want %d and %d",
					real, synth, tc.wantRuns, tc.wantSynth)
			}
		})
	}
}

// The guard is write-side only: a claimless run.finished already stored
// on a ledger (the dogfood branch carries one) replays exactly as
// before — recorded, tied to no claim, moving neither hold nor task
// status. T3: stored events are never rewritten and never
// retro-invalidated.
func TestReplayAcceptsStoredClaimlessRun(t *testing.T) {
	d, c := startDaemon(t)
	task := createOne(t, c, "brandon", map[string]any{"title": "history happened"})

	// Write the event the way a pre-guard daemon did: directly, with no
	// claim anywhere in sight.
	d.mu.Lock()
	ev, err := d.newEventLocked(event.TypeRunFinished, "brandon/ghost", task, event.RunFinished{
		Outcome: event.OutcomeDone, Summary: "claimless, accepted before the guard existed",
	})
	if err == nil {
		err = d.commitLocked(false, ev)
	}
	degraded := d.degraded
	d.mu.Unlock()
	if err != nil {
		t.Fatalf("write claimless run.finished: %v", err)
	}
	if degraded != nil {
		t.Fatalf("daemon degraded on claimless run.finished: %v", degraded)
	}

	// Land it on the branch and replay from scratch.
	if err := d.batcher.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if err := d.Refresh(); err != nil {
		t.Fatalf("Refresh over claimless run.finished: %v", err)
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	var run *runJSON
	for i := range d.state.Runs {
		if r := &d.state.Runs[i]; r.Task == task && r.ID == ev.ID {
			rj := runJSONOf(r)
			run = &rj
		}
	}
	if run == nil {
		t.Fatal("claimless run.finished not replayed into state")
	}
	if run.Claim != "" || run.Synthesized {
		t.Fatalf("replayed run = %+v, want empty claim ref, not synthesized", run)
	}
	if got := d.state.Tasks[task].Status; got != "open" {
		t.Fatalf("task status = %q, want open (a non-holder run moves nothing)", got)
	}
}
