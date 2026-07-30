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
		if !strings.HasPrefix(id, "t-") {
			t.Fatalf("tmp %q resolved to %q, want a t- id", name, id)
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
		"README.md":          "",
		"backlog.md":         "make views ride",
		"escalations.md":     "do views ride?",
		"tasks/" + id + ".md": "make views ride",
		views.MetaPath:       "",
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
