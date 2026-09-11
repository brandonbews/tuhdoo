package daemon

// The versioned replica (001 D2/D3/D6/D9, 002 T4/T6, 2026-09-10): a
// monotonic state version bumped on staged writes, merges, and lease
// transitions; a memo served at an unchanged version; one long-polled
// snapshot that parks until the version moves; views re-rendered on
// every bump and staged only when their bytes change. The lease
// transition tests drive a fake clock and a fake transition timer, so
// "a lease that expires at T" is observed at exactly T.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeClock is the daemon's clock under test control.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// fakeTimer is the one transition timer, firing only when the test
// says so. Armed durations are recorded so a test can see what the
// daemon asked for.
type fakeTimer struct {
	mu    sync.Mutex
	f     func()
	armed bool
	dur   time.Duration
	made  int // afterFunc calls: the daemon must make exactly one timer
}

func (ft *fakeTimer) afterFunc(d time.Duration, f func()) timer {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	ft.f, ft.armed, ft.dur = f, true, d
	ft.made++
	return ft
}

func (ft *fakeTimer) Stop() bool {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	was := ft.armed
	ft.armed = false
	return was
}

func (ft *fakeTimer) Reset(d time.Duration) bool {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	was := ft.armed
	ft.armed, ft.dur = true, d
	return was
}

// fire runs the callback, as the real timer would at its deadline.
// Fails the test when nothing is armed: the daemon forgot to re-arm.
func (ft *fakeTimer) fire(t *testing.T) {
	t.Helper()
	ft.mu.Lock()
	f, armed := ft.f, ft.armed
	ft.armed = false
	ft.mu.Unlock()
	if !armed || f == nil {
		t.Fatal("transition timer fired with nothing armed")
	}
	f()
}

func (ft *fakeTimer) armedFor() (time.Duration, bool) {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	return ft.dur, ft.armed
}

func (ft *fakeTimer) timersMade() int {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	return ft.made
}

// startFakeTimeDaemon starts a daemon on a fake clock (seeded from the
// real one, so ULIDs stay sane) and a fake transition timer, with a
// batcher quiet period long enough that only explicit flushes commit.
func startFakeTimeDaemon(t *testing.T, ttl time.Duration) (*Daemon, *http.Client, *fakeClock, *fakeTimer) {
	t.Helper()
	clock := &fakeClock{t: time.Now().Truncate(time.Second)}
	ft := &fakeTimer{}
	d, c := startDaemonOpts(t, Options{
		Quiet:     time.Hour,
		LeaseTTL:  ttl,
		Log:       log.New(io.Discard, "", 0),
		now:       clock.now,
		afterFunc: ft.afterFunc,
	})
	return d, c, clock, ft
}

func stateVersion(d *Daemon) uint64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.stateVersion
}

func replays(d *Daemon) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.replays
}

// snapshotResult is one long-poll's outcome, from a goroutine.
type snapshotResult struct {
	status  int
	body    []byte
	err     error
	elapsed time.Duration
}

// park issues GET /v0/snapshot?since=&wait= in the background and
// returns the channel its result lands on.
func park(c *http.Client, since uint64, wait string) <-chan snapshotResult {
	out := make(chan snapshotResult, 1)
	go func() {
		start := time.Now()
		status, body, err := do(c, "GET", fmt.Sprintf("/v0/snapshot?since=%d&wait=%s", since, wait), "", nil)
		out <- snapshotResult{status: status, body: body, err: err, elapsed: time.Since(start)}
	}()
	return out
}

// stillParked asserts a long poll has not answered within d.
func stillParked(t *testing.T, res <-chan snapshotResult, d time.Duration) {
	t.Helper()
	select {
	case r := <-res:
		t.Fatalf("snapshot answered while it should be parked: status %d, body %s", r.status, r.body)
	case <-time.After(d):
	}
}

// answered waits up to limit for the long poll to answer and decodes
// the snapshot.
func answered(t *testing.T, res <-chan snapshotResult, limit time.Duration) snapshotResp {
	t.Helper()
	select {
	case r := <-res:
		if r.err != nil {
			t.Fatalf("snapshot: %v", r.err)
		}
		if r.status != http.StatusOK {
			t.Fatalf("snapshot: status %d, body %s", r.status, r.body)
		}
		var snap snapshotResp
		unmarshalInto(t, r.body, &snap)
		return snap
	case <-time.After(limit):
		t.Fatalf("parked snapshot did not answer within %v", limit)
		return snapshotResp{}
	}
}

func claimVia(t *testing.T, c *http.Client, actor, task string) hydratedTask {
	t.Helper()
	var h hydratedTask
	unmarshalInto(t, mustDo(t, c, "POST", "/v0/claims", actor, map[string]any{"task": task}, http.StatusOK), &h)
	if h.Claim == nil {
		t.Fatalf("claim of %s carried no claim", task)
	}
	return h
}

// The version moves exactly once per staged write, once per merge
// callback (the syncer's OnMerged is Refresh), and once per lease
// transition — and a claim, which writes a lease and an event, is one
// bump too.
func TestVersionBumpsOncePerWriteMergeAndTransition(t *testing.T) {
	d, c, clock, ft := startFakeTimeDaemon(t, 15*time.Minute)
	v := stateVersion(d)
	if v == 0 {
		t.Fatal("version is 0 after the first load, want the load counted as the first bump")
	}

	task := createOne(t, c, "brandon", map[string]any{"title": "count my bumps"})
	if got := stateVersion(d); got != v+1 {
		t.Fatalf("version after one staged write = %d, want %d", got, v+1)
	}
	v++

	if err := d.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if got := stateVersion(d); got != v+1 {
		t.Fatalf("version after one merge callback = %d, want %d", got, v+1)
	}
	v++

	claimVia(t, c, "brandon/a1", task)
	if got := stateVersion(d); got != v+1 {
		t.Fatalf("version after one claim = %d, want %d (lease + event is one bump)", got, v+1)
	}
	v++
	if dur, armed := ft.armedFor(); !armed || dur != 15*time.Minute {
		t.Fatalf("transition timer armed for %v (armed %v), want the lease TTL 15m", dur, armed)
	}
	if made := ft.timersMade(); made != 1 {
		t.Fatalf("daemon made %d timers, want exactly one, re-armed", made)
	}

	// The lease lapses at T: the timer fires at T, one replay, one bump,
	// the task back in the pool with the attempt on record.
	clock.advance(15 * time.Minute)
	ft.fire(t)
	if got := stateVersion(d); got != v+1 {
		t.Fatalf("version after one lease transition = %d, want %d", got, v+1)
	}
	row := snapshotTaskOf(t, c, task)
	if row.Claim != nil || row.Situation != "ready" || len(row.Runs) != 1 || !row.Runs[0].Synthesized {
		t.Fatalf("after the lapse: claim %+v situation %q runs %+v; want no claim, ready, one synthesized run",
			row.Claim, row.Situation, row.Runs)
	}
	if _, armed := ft.armedFor(); armed {
		t.Fatal("timer still armed with no live lease to lapse")
	}
}

// since equal to the current version parks; a bump wakes the parked
// request at once with the new version and the whole snapshot.
func TestSnapshotParksUntilBump(t *testing.T) {
	d, c := startDaemon(t)
	v := stateVersion(d)

	res := park(c, v, "10s")
	stillParked(t, res, 100*time.Millisecond)

	start := time.Now()
	id := createOne(t, c, "brandon", map[string]any{"title": "wake the pane"})
	snap := answered(t, res, 2*time.Second)
	if woke := time.Since(start); woke > 500*time.Millisecond {
		t.Fatalf("parked request woke %v after the bump, want at once", woke)
	}
	if snap.Version != v+1 || snap.Unchanged || !snap.Loaded {
		t.Fatalf("woken snapshot = version %d unchanged %v loaded %v; want version %d, a full snapshot", snap.Version, snap.Unchanged, snap.Loaded, v+1)
	}
	if len(snap.Tasks) != 1 || snap.Tasks[0].Task.ID != id {
		t.Fatalf("woken snapshot tasks = %+v, want the one task %s", snap.Tasks, id)
	}
}

// A lease that expires at T wakes a parked request at T: the fake
// clock reaches T, the transition timer fires, and the pane redraws
// with the task back in the pool (D6 clause 5's accepted consequence).
func TestSnapshotWakesAtLeaseExpiry(t *testing.T) {
	d, c, clock, ft := startFakeTimeDaemon(t, 15*time.Minute)
	task := createOne(t, c, "brandon", map[string]any{"title": "the pane sees the lapse"})
	claimVia(t, c, "brandon/a1", task)
	v := stateVersion(d)

	res := park(c, v, "10s")
	stillParked(t, res, 100*time.Millisecond)

	// Short of T nothing happens, even if the timer fires early.
	clock.advance(15*time.Minute - time.Second)
	ft.fire(t)
	stillParked(t, res, 100*time.Millisecond)
	if dur, armed := ft.armedFor(); !armed || dur != time.Second {
		t.Fatalf("early fire re-armed for %v (armed %v), want 1s to T", dur, armed)
	}

	clock.advance(time.Second)
	ft.fire(t)
	snap := answered(t, res, 2*time.Second)
	if snap.Version != v+1 {
		t.Fatalf("woken at T with version %d, want %d", snap.Version, v+1)
	}
	if len(snap.Tasks) != 1 || snap.Tasks[0].Claim != nil || snap.Tasks[0].Situation != "ready" {
		t.Fatalf("woken snapshot tasks = %+v, want the task unclaimed and ready", snap.Tasks)
	}
}

// since semantics: behind the current version answers at once (and so
// does ahead of it — a restarted daemon counts from 1 again); equal
// parks; a wait that elapses answers the unchanged version alone.
func TestSnapshotSinceAndWait(t *testing.T) {
	d, c := startDaemon(t)
	createOne(t, c, "brandon", map[string]any{"title": "already here"})
	v := stateVersion(d)

	for _, since := range []uint64{0, v - 1, v + 100} {
		snap := answered(t, park(c, since, "10s"), time.Second)
		if snap.Version != v || snap.Unchanged || len(snap.Tasks) != 1 {
			t.Fatalf("since=%d: version %d unchanged %v tasks %d; want an immediate full snapshot at %d", since, snap.Version, snap.Unchanged, len(snap.Tasks), v)
		}
	}

	// Equal parks; the wait elapses; the unchanged version comes back
	// with nothing else, and the version did not move.
	res := park(c, v, "300ms")
	stillParked(t, res, 100*time.Millisecond)
	r := <-res
	if r.err != nil || r.status != http.StatusOK {
		t.Fatalf("timed-out poll: status %d err %v body %s", r.status, r.err, r.body)
	}
	if r.elapsed < 300*time.Millisecond {
		t.Fatalf("poll answered after %v, want the full 300ms wait", r.elapsed)
	}
	var out map[string]any
	unmarshalInto(t, r.body, &out)
	if out["version"] != float64(v) || out["unchanged"] != true || len(out) != 2 {
		t.Fatalf("timed-out poll body = %s, want exactly {version: %d, unchanged: true}", r.body, v)
	}

	// No wait at all answers at once, even at the current version.
	snap := answered(t, park(c, v, "0s"), time.Second)
	if snap.Version != v || snap.Unchanged {
		t.Fatalf("wait=0 at the current version = %+v, want an immediate full snapshot", snap)
	}

	// wait is capped, not refused; garbage is refused.
	if status, _, _ := do(c, "GET", "/v0/snapshot?wait=5h&since="+fmt.Sprint(v-1), "", nil); status != http.StatusOK {
		t.Fatalf("wait above the cap: status %d, want 200 (capped)", status)
	}
	for _, q := range []string{"since=abc", "wait=soon", "since=-1"} {
		if status, _, _ := do(c, "GET", "/v0/snapshot?"+q, "", nil); status != http.StatusBadRequest {
			t.Errorf("GET /v0/snapshot?%s: status %d, want 400", q, status)
		}
	}
}

// Shutdown wakes a parked request before the listener closes: the
// request answers, and Shutdown itself does not sit out the drain.
func TestShutdownWakesParkedSnapshot(t *testing.T) {
	d, c := startDaemon(t)
	v := stateVersion(d)
	res := park(c, v, "30s")
	stillParked(t, res, 100*time.Millisecond)

	start := time.Now()
	d.Shutdown("test: wake the parked pane")
	took := time.Since(start)
	r := <-res
	if r.err != nil || r.status != http.StatusOK {
		t.Fatalf("parked request at shutdown: status %d err %v body %s; want a 200 answer", r.status, r.err, r.body)
	}
	if took > 2*time.Second {
		t.Fatalf("Shutdown took %v with a parked request, want it woken before the drain (3s)", took)
	}
	var snap snapshotResp
	unmarshalInto(t, r.body, &snap)
	if snap.Version != v {
		t.Fatalf("shutdown answered version %d, want the unchanged %d", snap.Version, v)
	}
}

// Reads at an unchanged version are served from the memo — no replay —
// and a read arriving past the memo's validUntil replays exactly once.
func TestMemoServedAtUnchangedVersion(t *testing.T) {
	d, c, clock, _ := startFakeTimeDaemon(t, 15*time.Minute)
	task := createOne(t, c, "brandon", map[string]any{"title": "memoized"})
	claimVia(t, c, "brandon/a1", task)

	before := replays(d)
	for i := 0; i < 3; i++ {
		snapshotNow(t, c)
		if _, oe := d.opGetTask(task); oe != nil {
			t.Fatalf("get task: %v", oe)
		}
		if _, oe := d.opBacklog(nil); oe != nil {
			t.Fatalf("backlog: %v", oe)
		}
	}
	if got := replays(d); got != before {
		t.Fatalf("reads at an unchanged version replayed %d times, want 0", got-before)
	}

	// Past validUntil with the timer not yet fired (a machine waking
	// from sleep): the first read replays once, the next serves the
	// new memo.
	clock.advance(15 * time.Minute)
	row := snapshotTaskOf(t, c, task)
	if row.Claim != nil {
		t.Fatalf("read past the lease expiry served the lapsed claim as live: %+v", row.Claim)
	}
	if got := replays(d); got != before+1 {
		t.Fatalf("read past validUntil replayed %d times, want exactly 1", got-before)
	}
	snapshotNow(t, c)
	if got := replays(d); got != before+1 {
		t.Fatalf("second read after the transition replayed again (%d total), want the memo", got-before)
	}
}

// backlogSection returns the "## ..." heading under which backlog.md
// lists the task, or "" when it is under none.
func backlogSection(md []byte, task string) string {
	section := ""
	for _, line := range strings.Split(string(md), "\n") {
		if strings.HasPrefix(line, "## ") {
			section = strings.TrimPrefix(line, "## ")
		}
		if strings.Contains(line, "tasks/"+task+".md") {
			return section
		}
	}
	return ""
}

func headSubject(t *testing.T, d *Daemon) string {
	t.Helper()
	return strings.TrimSpace(runGit(t, d.root, "log", "-1", "--format=%s", d.store.Head()))
}

// Views follow the version: a claim's eager commit carries backlog.md
// with the task under In progress; a release moves it back under
// Ready; a lease expiry is a view-only commit ("0 events, N files")
// that renders the task ready again; renewals stage nothing; the
// claim path pokes the syncer.
func TestViewsFollowTheVersion(t *testing.T) {
	d, c, clock, ft := startFakeTimeDaemon(t, 15*time.Minute)
	// Pokes are counted under d.mu: commitLocked calls the seam with
	// the mutex held, and pokesSoFar reads it the same way.
	pokes := 0
	d.mu.Lock()
	d.poke = func() { pokes++ }
	d.mu.Unlock()
	pokesSoFar := func() int {
		d.mu.Lock()
		defer d.mu.Unlock()
		return pokes
	}

	task := createOne(t, c, "brandon", map[string]any{"title": "rendered live"})
	if err := d.batcher.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	backlog, _ := d.store.ReadFile("backlog.md")
	if got := backlogSection(backlog, task); got != "Ready" {
		t.Fatalf("before the claim, task under %q, want Ready:\n%s", got, backlog)
	}

	// Claim: the eager commit itself carries the re-rendered views,
	// and the syncer is poked.
	held := claimVia(t, c, "brandon/a1", task)
	if got := pokesSoFar(); got != 1 {
		t.Fatalf("claim poked the syncer %d times, want 1", got)
	}
	if subj := headSubject(t, d); !strings.HasPrefix(subj, "tuhdoo: 1 events, ") || strings.Contains(subj, " 0 files") {
		t.Fatalf("claim commit subject %q, want the claim event with its views", subj)
	}
	backlog, _ = d.store.ReadFile("backlog.md")
	if got := backlogSection(backlog, task); got != "In progress" {
		t.Fatalf("after the claim, task under %q, want In progress:\n%s", got, backlog)
	}
	page, _ := d.store.ReadFile("tasks/" + task + ".md")
	if !bytes.Contains(page, []byte("claimed by `brandon/a1`")) {
		t.Fatalf("task page after the claim does not show the holder:\n%s", page)
	}

	// Three renewal ticks stage zero files: the render carries no
	// expiry, so nothing differs from the tree — a flush after them is
	// a no-op. Renewal is the holding session's tick (T8), so the test
	// plays the session.
	s := &mcpSession{actor: "brandon/a1", stop: make(chan struct{}), claims: make(map[string]string)}
	s.track(task, held.Claim.ID)
	for i := 0; i < 3; i++ {
		clock.advance(time.Minute)
		d.renewOnce(s)
	}
	head := d.store.Head()
	if err := d.batcher.Flush(); err != nil {
		t.Fatalf("flush after renewals: %v", err)
	}
	if d.store.Head() != head {
		t.Fatalf("renewals staged views: flush moved the head from %s to %s (%q)", head, d.store.Head(), headSubject(t, d))
	}

	// Release: the eager commit renders the task back under Ready and
	// pokes again.
	mustDo(t, c, "DELETE", "/v0/claims", "brandon/a1", map[string]any{"task": task, "reason": "done for now"}, http.StatusOK)
	if got := pokesSoFar(); got != 2 {
		t.Fatalf("release poked the syncer %d times in total, want 2", got)
	}
	backlog, _ = d.store.ReadFile("backlog.md")
	if got := backlogSection(backlog, task); got != "Ready" {
		t.Fatalf("after the release, task under %q, want Ready:\n%s", got, backlog)
	}

	// Expiry: claim again, let the lease lapse — the bump stages the
	// views with no event, and the batch commits as view-only.
	claimVia(t, c, "brandon/a2", task)
	clock.advance(15 * time.Minute)
	ft.fire(t)
	head = d.store.Head()
	if err := d.batcher.Flush(); err != nil {
		t.Fatalf("flush after expiry: %v", err)
	}
	if d.store.Head() == head {
		t.Fatal("lease expiry staged nothing: the branch still renders the lapsed holder")
	}
	if subj := headSubject(t, d); !strings.HasPrefix(subj, "tuhdoo: 0 events, ") || strings.HasSuffix(subj, " 0 files") {
		t.Fatalf("expiry commit subject %q, want a view-only \"0 events, N files\"", subj)
	}
	backlog, _ = d.store.ReadFile("backlog.md")
	if got := backlogSection(backlog, task); got != "Ready" {
		t.Fatalf("after the lapse, task under %q, want Ready:\n%s", got, backlog)
	}
	page, _ = d.store.ReadFile("tasks/" + task + ".md")
	if !bytes.Contains(page, []byte("interrupted")) {
		t.Fatalf("task page after the lapse does not record the interrupted attempt:\n%s", page)
	}
}

// /v0/state and GET /v0/tasks/{id} are gone: 404, naming the snapshot.
func TestDeletedReadEndpointsAre404(t *testing.T) {
	_, c := startDaemon(t)
	id := createOne(t, c, "brandon", map[string]any{"title": "unreachable the old way"})
	for _, path := range []string{"/v0/state", "/v0/tasks/" + id} {
		status, body, err := do(c, "GET", path, "", nil)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		if status != http.StatusNotFound || !strings.Contains(string(body), "/v0/snapshot") {
			t.Errorf("GET %s: status %d body %s; want 404 pointing at /v0/snapshot", path, status, body)
		}
	}
	// The write on the same pattern still lives.
	mustDo(t, c, "PATCH", "/v0/tasks/"+id, "brandon", map[string]any{"title": "still writable"}, http.StatusOK)
}

// Every snapshot task entry is the hydration get_task serves, field
// for field: the entry decodes into a hydratedTask that round-trips to
// the same bytes as the op's own, and every key of the hydration is
// present in the entry with the same value.
func TestSnapshotTaskMatchesGetTaskShape(t *testing.T) {
	d, c := startDaemon(t)
	dep := createOne(t, c, "brandon", map[string]any{"title": "the dep"})
	id := createOne(t, c, "brandon", map[string]any{
		"title": "everything attached", "priority": 1, "labels": []string{"go"}, "depends_on": []string{dep}})
	mustDo(t, c, "PATCH", "/v0/tasks/"+id, "brandon", map[string]any{"title": "everything attached, retitled"}, http.StatusOK)
	mustDo(t, c, "POST", "/v0/notes", "brandon/a1", map[string]any{"task": id, "text": "a checkpoint"}, http.StatusOK)
	mustDo(t, c, "POST", "/v0/escalations", "brandon/a1", map[string]any{"task": id, "question": "which way?"}, http.StatusOK)
	claimVia(t, c, "brandon/a1", dep)
	mustDo(t, c, "POST", "/v0/runs", "brandon/a1", map[string]any{"task": dep, "outcome": "done"}, http.StatusOK)
	claimVia(t, c, "brandon/a2", id)

	for _, task := range []string{dep, id} {
		entry := snapshotTaskOf(t, c, task)
		h, oe := d.opGetTask(task)
		if oe != nil {
			t.Fatalf("get task %s: %v", task, oe)
		}
		wantJSON, err := json.Marshal(h)
		if err != nil {
			t.Fatal(err)
		}
		// The entry, re-encoded, decodes into a hydratedTask equal to
		// the op's — and the entry's raw keys are a superset carrying
		// every hydration key at the same value.
		entryJSON, err := json.Marshal(entry)
		if err != nil {
			t.Fatal(err)
		}
		var decoded hydratedTask
		unmarshalInto(t, entryJSON, &decoded)
		gotJSON, err := json.Marshal(decoded)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(gotJSON, wantJSON) {
			t.Fatalf("snapshot entry for %s decodes to\n%s\nwant get_task's\n%s", task, gotJSON, wantJSON)
		}
		var entryKeys, hydrationKeys map[string]any
		unmarshalInto(t, entryJSON, &entryKeys)
		unmarshalInto(t, wantJSON, &hydrationKeys)
		for k, v := range hydrationKeys {
			if !reflect.DeepEqual(entryKeys[k], v) {
				t.Errorf("%s: snapshot entry key %q = %v, get_task has %v", task, k, entryKeys[k], v)
			}
		}
		for _, k := range []string{"situation"} {
			if _, ok := entryKeys[k]; !ok {
				t.Errorf("%s: snapshot entry lacks %q", task, k)
			}
		}
	}
}
