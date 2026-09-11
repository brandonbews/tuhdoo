package syncer

// The syncer over the live replica (002 T2, 2026-09-10): merges land
// through the store with two parents and the syncer never moves the
// ref itself; merge-time replays read only the blobs the store's cache
// lacks; the cycle's ref check notices a ref moved from outside the
// daemon, remote or not.

import (
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brandonbews/tuhdoo/internal/event"
	"github.com/brandonbews/tuhdoo/internal/gitx"
	"github.com/brandonbews/tuhdoo/internal/store"
	"github.com/brandonbews/tuhdoo/internal/views"
)

// watchingGit wraps a real git: it records every OID requested through
// CatFiles (so a test can say what a merge read) and, when refuseMoves
// is set — the syncer's copy — fails the test on any UpdateRef, since
// the store is the single mover.
type watchingGit struct {
	gitx.Git
	t           *testing.T
	refuseMoves bool
	mu          sync.Mutex
	updates     int
	catOIDs     []string
}

func (w *watchingGit) UpdateRef(ref, newOID, oldOID string) error {
	w.mu.Lock()
	w.updates++
	w.mu.Unlock()
	if w.refuseMoves {
		w.t.Errorf("UpdateRef(%s, %s, %s) called through the syncer's git; only the store moves the ref", ref, newOID, oldOID)
	}
	return w.Git.UpdateRef(ref, newOID, oldOID)
}

func (w *watchingGit) CatFiles(oids []string) (map[string][]byte, error) {
	w.mu.Lock()
	w.catOIDs = append(w.catOIDs, oids...)
	w.mu.Unlock()
	return w.Git.CatFiles(oids)
}

func (w *watchingGit) requested() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := append([]string(nil), w.catOIDs...)
	sort.Strings(out)
	return out
}

// newWatchedPair is newPair with peer A's git wrapped twice: the
// store's copy records what it reads, the syncer's copy refuses to
// move the ref. Returned as (a, b, storeGit, syncGit).
func newWatchedPair(t *testing.T) (peer, peer, *watchingGit, *watchingGit) {
	t.Helper()
	a, b := newPair(t)
	storeGit := &watchingGit{Git: a.git, t: t}
	syncGit := &watchingGit{Git: a.git, t: t, refuseMoves: true}
	id := gitx.Identity{Name: "machine-a", Email: "machine-a@test.invalid"}
	a.store = store.New(storeGit, "", id)
	a.sync = New(syncGit, a.store, Options{Ident: id})
	return a, b, storeGit, syncGit
}

// A union merge is committed through the store — two parents, the
// local head first — and moves the ref and the replica together; the
// syncer's own git never sees an UpdateRef. A fast-forward lands the
// same way.
func TestMergeCommitsThroughStoreWithTwoParents(t *testing.T) {
	a, b, _, syncGit := newWatchedPair(t)

	if err := a.store.AppendBatch(store.Batch{Events: []event.Event{
		evt(t, 1, event.TypeTaskCreated, "brandon", "m-a", "t1", event.TaskCreated{Title: "born on A"}),
	}}); err != nil {
		t.Fatal(err)
	}
	cycle(t, a)
	cycle(t, b) // B fast-forwards onto A's history
	if err := b.store.AppendBatch(store.Batch{Events: []event.Event{
		evt(t, 2, event.TypeNoteAdded, "sarah", "m-b", "t1", event.NoteAdded{Text: "from B"}),
	}}); err != nil {
		t.Fatal(err)
	}
	cycle(t, b)
	// A diverges, then merges.
	if err := a.store.AppendBatch(store.Batch{Events: []event.Event{
		evt(t, 3, event.TypeNoteAdded, "brandon", "m-a", "t1", event.NoteAdded{Text: "from A"}),
	}}); err != nil {
		t.Fatal(err)
	}
	localBefore := a.store.Head()
	remoteBefore := strings.TrimSpace(runGit(t, b.dir, "rev-parse", store.DefaultRef))
	cycle(t, a)

	ref := strings.TrimSpace(runGit(t, a.dir, "rev-parse", store.DefaultRef))
	if a.store.Head() != ref {
		t.Fatalf("after the merge the replica is at %s, the ref at %s", a.store.Head(), ref)
	}
	parents := strings.Fields(runGit(t, a.dir, "log", "-1", "--format=%P", ref))
	if len(parents) != 2 || parents[0] != localBefore || parents[1] != remoteBefore {
		t.Fatalf("merge commit parents = %v, want [%s %s] (local head first, remote head second)", parents, localBefore, remoteBefore)
	}
	if a.sync.Status().Merges != 1 {
		t.Fatalf("Merges = %d, want 1", a.sync.Status().Merges)
	}
	// The replica serves the merged state without reloading: both
	// notes, one task.
	events, err := a.store.LoadEvents()
	if err != nil || len(events) != 3 {
		t.Fatalf("events after merge = %d, %v; want 3", len(events), err)
	}
	cycle(t, b) // B fast-forwards to the merge, also through its store
	sameTrees(t, a, b)
	if b.store.Head() != strings.TrimSpace(runGit(t, b.dir, "rev-parse", store.DefaultRef)) {
		t.Fatalf("B's replica did not follow its fast-forward")
	}
	if syncGit.updates != 0 {
		t.Fatalf("the syncer's git saw %d UpdateRef calls, want 0", syncGit.updates)
	}
}

// A merge replay after a fetch reads only the OIDs the store's cache
// lacks — the other machine's new events and leases (and a view stamp
// the replica never decoded) — never the shared history it already
// holds, and each of them exactly once.
func TestMergeReplayFetchesOnlyUnseenBlobs(t *testing.T) {
	a, b, storeGit, syncGit := newWatchedPair(t)
	alive := time.Now().Add(time.Hour)

	// Shared history, written on A (so A's cache holds it from the
	// write) and adopted by B.
	if err := a.store.AppendBatch(store.Batch{Events: []event.Event{
		evt(t, 1, event.TypeTaskCreated, "brandon", "m-a", "t1", event.TaskCreated{Title: "fix login"}),
		evt(t, 2, event.TypeTaskCreated, "brandon", "m-a", "t2", event.TaskCreated{Title: "fix logout"}),
		evt(t, 3, event.TypeClaimMade, "brandon/impl-1", "m-a", "t1", event.ClaimMade{}),
	}}); err != nil {
		t.Fatal(err)
	}
	if err := a.store.WriteLease(tick(t, 3), alive); err != nil {
		t.Fatal(err)
	}
	cycle(t, a)
	cycle(t, b)

	// B's new work: two events and a lease A has never seen.
	if err := b.store.AppendBatch(store.Batch{Events: []event.Event{
		evt(t, 4, event.TypeClaimMade, "sarah/impl-9", "m-b", "t2", event.ClaimMade{}),
		evt(t, 5, event.TypeNoteAdded, "sarah/impl-9", "m-b", "t2", event.NoteAdded{Text: "on it"}),
	}}); err != nil {
		t.Fatal(err)
	}
	// A distinct expiry: the same bytes as A's lease would be the same
	// blob, which A already holds.
	if err := b.store.WriteLease(tick(t, 4), alive.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	cycle(t, b)
	// A diverges so the cycle must merge, not fast-forward.
	if err := a.store.AppendBatch(store.Batch{Events: []event.Event{
		evt(t, 6, event.TypeNoteAdded, "brandon/impl-1", "m-a", "t1", event.NoteAdded{Text: "me too"}),
	}}); err != nil {
		t.Fatal(err)
	}

	// Fetch first so their objects are in A's database for the
	// accounting below; the cycle's own fetch is then a no-op.
	ourTree := a.store.Tree()
	remoteHead, err := a.sync.fetch()
	if err != nil {
		t.Fatal(err)
	}
	theirTree, err := treeMap(a.git, remoteHead)
	if err != nil {
		t.Fatal(err)
	}
	// What the merge must read: every event and lease blob in their
	// tree that ours lacks. What it may additionally read: the two view
	// stamps (views are written, never read back, so even our own stamp
	// may be undecoded). Nothing else — rendered pages included.
	ourOIDs := make(map[string]bool, len(ourTree))
	for _, oid := range ourTree {
		ourOIDs[oid] = true
	}
	must := make(map[string]bool)
	for path, oid := range theirTree {
		if ourOIDs[oid] {
			continue
		}
		if strings.HasPrefix(path, "events/") || strings.HasPrefix(path, "leases/") {
			must[oid] = true
		}
	}
	if len(must) != 3 {
		t.Fatalf("scenario expects B's 2 events and 1 lease to be new to A, found %d", len(must))
	}
	may := map[string]bool{ourTree[views.MetaPath]: true, theirTree[views.MetaPath]: true}

	storeGit.catOIDs = nil
	cycle(t, a)
	if a.sync.Status().Merges != 1 {
		t.Fatalf("expected a merge, Merges = %d", a.sync.Status().Merges)
	}
	if n := len(syncGit.requested()); n != 0 {
		t.Fatalf("the syncer read %d blobs through its own git, want 0 — replays go through the store", n)
	}
	seen := make(map[string]int)
	for _, oid := range storeGit.requested() {
		seen[oid]++
	}
	for oid, n := range seen {
		if !must[oid] && !may[oid] {
			t.Errorf("merge read %s, a blob the replica already held or never needs", oid)
		}
		if n != 1 {
			t.Errorf("merge read %s %d times, want once", oid, n)
		}
	}
	for oid := range must {
		if seen[oid] == 0 {
			t.Errorf("merge never read the other side's new blob %s", oid)
		}
	}
	// And the replica serves the merged history with no further reads.
	storeGit.catOIDs = nil
	events, err := a.store.LoadEvents()
	if err != nil || len(events) != 6 {
		t.Fatalf("events after merge = %d, %v; want 6", len(events), err)
	}
	if n := len(storeGit.requested()); n != 0 {
		t.Fatalf("reading the merged replica fetched %d blobs, want 0", n)
	}
}

// The cycle's ref check runs before the remote check: a remoteless
// daemon whose ref was moved from outside (a second writer on the same
// repository, a `branch -f`) reloads its replica within one cycle and
// tells the daemon (OnMerged). A ref the store itself moved is not
// mistaken for an external move.
func TestCycleNoticesExternalMoveWithoutRemote(t *testing.T) {
	dir, g := mkRepo(t, "solo", "")
	st := store.New(g, "", ident("solo"))
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	refreshed := 0
	sy := New(g, st, Options{Ident: ident("solo"), OnMerged: func() { refreshed++ }})
	if err := st.AppendBatch(store.Batch{Events: []event.Event{
		evt(t, 1, event.TypeTaskCreated, "brandon", "m-a", "t1", event.TaskCreated{Title: "mine"}),
	}}); err != nil {
		t.Fatal(err)
	}
	if err := sy.Cycle(); err != nil {
		t.Fatalf("cycle: %v", err)
	}
	if refreshed != 0 {
		t.Fatalf("a ref the store moved itself triggered %d refreshes, want 0", refreshed)
	}
	if sy.Status().Mode != "local-only" {
		t.Fatalf("mode = %q, want local-only", sy.Status().Mode)
	}

	// Another writer moves the ref behind the replica's back.
	other := store.New(g, "", ident("other"))
	if err := other.AppendBatch(store.Batch{Events: []event.Event{
		evt(t, 2, event.TypeTaskCreated, "sarah", "m-b", "t2", event.TaskCreated{Title: "theirs"}),
	}}); err != nil {
		t.Fatal(err)
	}
	ref := strings.TrimSpace(runGit(t, dir, "rev-parse", store.DefaultRef))
	if st.Head() == ref {
		t.Fatal("setup: the replica should not know about the external move yet")
	}
	if err := sy.Cycle(); err != nil {
		t.Fatalf("cycle after external move: %v", err)
	}
	if st.Head() != ref {
		t.Fatalf("replica head = %s after the cycle, ref at %s", st.Head(), ref)
	}
	if refreshed != 1 {
		t.Fatalf("external move triggered %d refreshes, want 1", refreshed)
	}
	events, err := st.LoadEvents()
	if err != nil || len(events) != 2 {
		t.Fatalf("events after reload = %d, %v; want both writers' events", len(events), err)
	}
	// The reload reseeded the index: the replica's next commit carries
	// the other writer's file, not just its own.
	if err := st.AppendBatch(store.Batch{Events: []event.Event{
		evt(t, 3, event.TypeNoteAdded, "brandon", "m-a", "t1", event.NoteAdded{Text: "after"}),
	}}); err != nil {
		t.Fatal(err)
	}
	tree, err := treeMap(g, store.DefaultRef)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []int{1, 2, 3} {
		if _, ok := tree[eventPath(t, n)]; !ok {
			t.Fatalf("committed tree after the reload lacks event %d: %v", n, tree)
		}
	}
}
