package store

// The store as live replica (001 D2 / 002 T2, 2026-09-10): head and
// tree in memory, the private index reseeded at every load and reload,
// blobs written only when their bytes changed, and the store as the
// single mover of the ref. Real git underneath a recording wrapper
// that counts every call by method, so each test pins the subprocess
// shape of an operation, not just its outcome.

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brandonbews/tuhdoo/internal/event"
	"github.com/brandonbews/tuhdoo/internal/gitx"
)

// recordingGit wraps a real git and counts calls per method (plus the
// OIDs CatFiles was asked for), so a test can say exactly which
// subprocesses an operation cost — and that a read cost none.
type recordingGit struct {
	gitx.Git
	mu       sync.Mutex
	calls    map[string]int
	readTree []string // ReadTree arguments, in order
	catOIDs  []string // every OID requested through CatFiles
}

func newRecordingGit(g gitx.Git) *recordingGit {
	return &recordingGit{Git: g, calls: make(map[string]int)}
}

func (r *recordingGit) count(method string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls[method]++
}

// reset clears the counts; the wrapped git is untouched.
func (r *recordingGit) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = make(map[string]int)
	r.readTree = nil
	r.catOIDs = nil
}

// total is the number of git calls of any kind since the last reset.
func (r *recordingGit) total() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, c := range r.calls {
		n += c
	}
	return n
}

// snapshot copies the per-method counts.
func (r *recordingGit) snapshot() map[string]int {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]int, len(r.calls))
	for k, v := range r.calls {
		out[k] = v
	}
	return out
}

func (r *recordingGit) HashObject(data []byte) (string, error) {
	r.count("HashObject")
	return r.Git.HashObject(data)
}
func (r *recordingGit) MkTree(entries []gitx.TreeEntry) (string, error) {
	r.count("MkTree")
	return r.Git.MkTree(entries)
}
func (r *recordingGit) CommitTree(tree string, parents []string, ident gitx.Identity, msg string) (string, error) {
	r.count("CommitTree")
	return r.Git.CommitTree(tree, parents, ident, msg)
}
func (r *recordingGit) UpdateRef(ref, newOID, oldOID string) error {
	r.count("UpdateRef")
	return r.Git.UpdateRef(ref, newOID, oldOID)
}
func (r *recordingGit) ReadRef(ref string) (string, error) {
	r.count("ReadRef")
	return r.Git.ReadRef(ref)
}
func (r *recordingGit) CatFile(oid string) ([]byte, error) {
	r.count("CatFile")
	return r.Git.CatFile(oid)
}
func (r *recordingGit) CatFiles(oids []string) (map[string][]byte, error) {
	r.count("CatFiles")
	r.mu.Lock()
	r.catOIDs = append(r.catOIDs, oids...)
	r.mu.Unlock()
	return r.Git.CatFiles(oids)
}
func (r *recordingGit) LsTree(rev string) ([]gitx.TreeEntry, error) {
	r.count("LsTree")
	return r.Git.LsTree(rev)
}
func (r *recordingGit) ReadTree(tree string) error {
	r.count("ReadTree")
	r.mu.Lock()
	r.readTree = append(r.readTree, tree)
	r.mu.Unlock()
	return r.Git.ReadTree(tree)
}
func (r *recordingGit) UpdateIndex(entries []gitx.TreeEntry) error {
	r.count("UpdateIndex")
	return r.Git.UpdateIndex(entries)
}
func (r *recordingGit) WriteTree() (string, error) {
	r.count("WriteTree")
	return r.Git.WriteTree()
}
func (r *recordingGit) IsAncestor(a, b string) (bool, error) {
	r.count("IsAncestor")
	return r.Git.IsAncestor(a, b)
}
func (r *recordingGit) Fetch(remote, refspec string) error {
	r.count("Fetch")
	return r.Git.Fetch(remote, refspec)
}
func (r *recordingGit) FetchTimeout(remote, refspec string, timeout time.Duration) error {
	r.count("FetchTimeout")
	return r.Git.FetchTimeout(remote, refspec, timeout)
}
func (r *recordingGit) Push(remote, refspec string) error {
	r.count("Push")
	return r.Git.Push(remote, refspec)
}
func (r *recordingGit) RemoteURL(remote string) (string, error) {
	r.count("RemoteURL")
	return r.Git.RemoteURL(remote)
}

// newRecordedStore is newStore over a recording wrapper: a fresh
// repository, an initialized and loaded Store, counts reset.
func newRecordedStore(t *testing.T) (*Store, *recordingGit, string) {
	t.Helper()
	setGitEnv(t)
	dir := t.TempDir()
	runGit(t, dir, "init", "--quiet", "-b", "main")
	real, err := gitx.New(dir)
	if err != nil {
		t.Fatalf("gitx.New: %v", err)
	}
	rg := newRecordingGit(real)
	s := New(rg, "", testIdent)
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	rg.reset()
	return s, rg, dir
}

// rawTree is `git ls-tree` of rev as path → OID, read by git directly
// so the store's memory is checked against the object database.
func rawTree(t *testing.T, dir, rev string) map[string]string {
	t.Helper()
	out := runGit(t, dir, "ls-tree", "-r", "--full-tree", rev)
	m := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		meta, path, _ := strings.Cut(line, "\t")
		m[path] = strings.Fields(meta)[2]
	}
	return m
}

func sameTree(t *testing.T, what string, got, want map[string]string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: %d entries, want %d\ngot  %v\nwant %v", what, len(got), len(want), got, want)
	}
	for path, oid := range want {
		if got[path] != oid {
			t.Fatalf("%s: %s = %s, want %s", what, path, got[path], oid)
		}
	}
}

// renderedFiles is a stand-in for a full view render: n pages whose
// bytes depend on version, so bumping version for a subset changes
// exactly that subset.
func renderedFiles(n int, version map[string]int) map[string][]byte {
	files := make(map[string][]byte, n)
	for i := 0; i < n; i++ {
		path := fmt.Sprintf("tasks/t-%03d.md", i)
		files[path] = []byte(fmt.Sprintf("# task %d, render %d\n", i, version[path]))
	}
	return files
}

// A commit costs one hash-object per genuinely new blob plus the four
// fixed processes: one new event and 166 rendered files of which 5
// changed is six HashObject calls, one UpdateIndex, one WriteTree, one
// CommitTree, one UpdateRef — and no LsTree, ReadRef, ReadTree, or
// CatFiles on the success path (T2, 2026-09-10).
func TestCommitWritesOnlyChangedBlobs(t *testing.T) {
	s, rg, dir := newRecordedStore(t)

	const pages = 166
	version := make(map[string]int)
	if err := s.AppendBatch(Batch{Events: []event.Event{newEvent(t, 0)}, Files: renderedFiles(pages, version)}); err != nil {
		t.Fatalf("seed AppendBatch: %v", err)
	}
	rg.reset()

	for i := 0; i < 5; i++ {
		version[fmt.Sprintf("tasks/t-%03d.md", i*30)]++
	}
	if err := s.AppendBatch(Batch{Events: []event.Event{newEvent(t, 1)}, Files: renderedFiles(pages, version)}); err != nil {
		t.Fatalf("AppendBatch: %v", err)
	}

	want := map[string]int{"HashObject": 6, "UpdateIndex": 1, "WriteTree": 1, "CommitTree": 1, "UpdateRef": 1}
	got := rg.snapshot()
	for method, n := range want {
		if got[method] != n {
			t.Errorf("%s called %d times, want %d (all calls: %v)", method, got[method], n, got)
		}
	}
	for _, method := range []string{"LsTree", "ReadRef", "ReadTree", "CatFiles", "CatFile", "MkTree"} {
		if got[method] != 0 {
			t.Errorf("%s called %d times on the success path, want 0 (all calls: %v)", method, got[method], got)
		}
	}
	if rg.total() != 10 {
		t.Errorf("commit cost %d git calls, want exactly 10: %v", rg.total(), got)
	}

	// Memory and git agree on the result, and the ref is where the
	// replica says.
	head := strings.TrimSpace(runGit(t, dir, "rev-parse", DefaultRef))
	if s.Head() != head {
		t.Fatalf("Head() = %s, ref at %s", s.Head(), head)
	}
	sameTree(t, "tree after commit", rawTree(t, dir, head), s.Tree())
	assertNoWorktreeFiles(t, dir)
}

// Reads never spawn git once loaded: ReplayInput, ReadFile of a blob
// read once before, Head, and Tree all answer from memory.
func TestReadsSpawnNoGitOnceLoaded(t *testing.T) {
	s, rg, _ := newRecordedStore(t)
	if err := s.AppendBatch(Batch{
		Events: []event.Event{newEvent(t, 0), newEvent(t, 1)},
		Files:  map[string][]byte{"meta.json": []byte(`{"format":1}`)},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteLease("c1", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	// One read of a file the store never decoded costs one batch read;
	// every later read is memory.
	if _, err := s.ReadFile("meta.json"); err != nil {
		t.Fatal(err)
	}
	rg.reset()

	for i := 0; i < 3; i++ {
		events, leases, err := s.ReplayInput()
		if err != nil {
			t.Fatalf("ReplayInput: %v", err)
		}
		if len(events) != 2 || len(leases) != 1 {
			t.Fatalf("ReplayInput = %d events, %d leases; want 2 and 1", len(events), len(leases))
		}
		if data, err := s.ReadFile("meta.json"); err != nil || string(data) != `{"format":1}` {
			t.Fatalf("ReadFile = %q, %v", data, err)
		}
		if data, err := s.ReadFile("absent"); err != nil || data != nil {
			t.Fatalf("ReadFile(absent) = %q, %v; want nil, nil", data, err)
		}
		_ = s.Head()
		_ = s.Tree()
	}
	if n := rg.total(); n != 0 {
		t.Fatalf("reads issued %d git calls, want 0: %v", n, rg.snapshot())
	}
}

// movingGit moves the ref behind the store's back — a second store's
// commit on the same repository — just before delegating the first
// UpdateRef, so the store's compare-and-swap loses for real.
type movingGit struct {
	gitx.Git
	t       *testing.T
	other   *Store
	updates int
}

func (m *movingGit) UpdateRef(ref, newOID, oldOID string) error {
	m.updates++
	if m.updates == 1 {
		if err := m.other.AppendBatch(Batch{Events: []event.Event{newEvent(m.t, 99)}}); err != nil {
			m.t.Fatalf("concurrent AppendBatch: %v", err)
		}
	}
	return m.Git.UpdateRef(ref, newOID, oldOID)
}

// A lost compare-and-swap reloads head and tree from git, reseeds the
// index, rebuilds on the new head, and retries: the resulting tree
// carries both the concurrent change and ours. This is the trap the
// design revision names — an index not reseeded after the reload would
// still hold our stale tree and the concurrent event would vanish from
// the commit (write-tree writes whatever the index holds).
func TestCommitRetriesOnLostCASAndKeepsBothChanges(t *testing.T) {
	s, rg, dir := newRecordedStore(t)
	if err := s.AppendBatch(Batch{Events: []event.Event{newEvent(t, 0)}}); err != nil {
		t.Fatal(err)
	}

	real, err := gitx.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	other := New(real, "", testIdent) // the concurrent writer, plain git
	mg := &movingGit{Git: rg, t: t, other: other}
	racy := New(mg, "", testIdent)
	if err := racy.Load(); err != nil {
		t.Fatal(err)
	}
	rg.reset()

	ours := newEvent(t, 1)
	if err := racy.AppendBatch(Batch{Events: []event.Event{ours}}); err != nil {
		t.Fatalf("AppendBatch across a lost CAS: %v", err)
	}
	if mg.updates != 2 {
		t.Fatalf("UpdateRef called %d times, want 2 (loss then retry)", mg.updates)
	}
	got := rg.snapshot()
	if got["ReadRef"] != 1 || got["LsTree"] != 1 || got["ReadTree"] != 1 {
		t.Fatalf("lost CAS should reload exactly once (ReadRef, LsTree, ReadTree = 1 each): %v", got)
	}

	head := strings.TrimSpace(runGit(t, dir, "rev-parse", DefaultRef))
	tree := rawTree(t, dir, head)
	oursPath, _ := event.Path(ours.ID)
	if _, ok := tree[oursPath]; !ok {
		t.Fatalf("committed tree lacks our event %s: %v", oursPath, tree)
	}
	if n := len(rawEvents(tree)); n != 3 {
		t.Fatalf("committed tree carries %d events, want 3 (seed, concurrent, ours): %v", n, tree)
	}
	// The replica agrees with git after the retry.
	if racy.Head() != head {
		t.Fatalf("Head() = %s, ref at %s", racy.Head(), head)
	}
	sameTree(t, "replica vs git", tree, racy.Tree())
	// And the concurrent writer's stale replica notices nothing until it
	// reloads: its next commit reloads on its own lost CAS.
	if err := other.AppendBatch(Batch{Events: []event.Event{newEvent(t, 2)}}); err != nil {
		t.Fatalf("other's AppendBatch after being overtaken: %v", err)
	}
	if n := len(rawEvents(rawTree(t, dir, strings.TrimSpace(runGit(t, dir, "rev-parse", DefaultRef))))); n != 4 {
		t.Fatalf("after both writers: %d events, want 4", n)
	}
}

func rawEvents(tree map[string]string) []string {
	var out []string
	for path := range tree {
		if strings.HasPrefix(path, "events/") {
			out = append(out, path)
		}
	}
	return out
}

// alwaysLosesGit fails every UpdateRef as a CAS loss: exhaustion.
type alwaysLosesGit struct {
	gitx.Git
	updates int
}

func (g *alwaysLosesGit) UpdateRef(ref, newOID, oldOID string) error {
	g.updates++
	return fmt.Errorf("simulated: %w", gitx.ErrRefCASFailed)
}

func TestCommitExhaustsCASRetries(t *testing.T) {
	s, _ := newStore(t)
	lg := &alwaysLosesGit{Git: s.git}
	racy := New(lg, "", testIdent)
	err := racy.AppendBatch(Batch{Events: []event.Event{newEvent(t, 0)}})
	if err == nil || !strings.Contains(err.Error(), "kept moving") || !errors.Is(err, gitx.ErrRefCASFailed) {
		t.Fatalf("AppendBatch under permanent CAS loss = %v, want the kept-moving error wrapping ErrRefCASFailed", err)
	}
	if lg.updates != maxCASRetries {
		t.Fatalf("UpdateRef called %d times, want maxCASRetries = %d", lg.updates, maxCASRetries)
	}
}

// Load reseeds the private index from the head — mandatory, not an
// optimization: an index pre-populated with entries from some other
// tree (and a stale lock left by a killed git) must not leak into the
// next commit. The committed tree equals the in-memory tree map
// exactly, checked by ls-tree.
func TestLoadReseedsIndexSoCommitsMatchMemory(t *testing.T) {
	setGitEnv(t)
	dir := t.TempDir()
	runGit(t, dir, "init", "--quiet", "-b", "main")
	real, err := gitx.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	seed := New(real, "", testIdent)
	if err := seed.Init(); err != nil {
		t.Fatal(err)
	}
	if err := seed.AppendBatch(Batch{
		Events: []event.Event{newEvent(t, 0)},
		Files:  map[string][]byte{"README.md": []byte("real\n")},
	}); err != nil {
		t.Fatal(err)
	}
	head := strings.TrimSpace(runGit(t, dir, "rev-parse", DefaultRef))

	// Garbage the index: entries from a foreign tree that is not the
	// head, plus a stale lock.
	indexPath := filepath.Join(dir, ".git", "tuhdoo", "index")
	foreign, err := real.HashObject([]byte("not on the branch\n"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "update-index", "--add", "--cacheinfo", "100644,"+foreign+",garbage/entry.md")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+indexPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("planting a garbage index entry: %v\n%s", err, out)
	}
	if err := os.WriteFile(indexPath+".lock", nil, 0o644); err != nil {
		t.Fatal(err)
	}

	rg := newRecordingGit(real)
	s := New(rg, "", testIdent)
	if err := s.Load(); err != nil {
		t.Fatalf("Load over a garbage index and stale lock: %v", err)
	}
	if rg.calls["ReadTree"] != 1 || len(rg.readTree) != 1 || rg.readTree[0] != head {
		t.Fatalf("Load reseeded with ReadTree %v (%d calls), want exactly one with the head %s", rg.readTree, rg.calls["ReadTree"], head)
	}
	if _, err := os.Stat(indexPath + ".lock"); !os.IsNotExist(err) {
		t.Fatalf("stale index.lock survived Load (stat err %v)", err)
	}

	if err := s.AppendBatch(Batch{Events: []event.Event{newEvent(t, 1)}}); err != nil {
		t.Fatalf("AppendBatch after reseed: %v", err)
	}
	newHead := strings.TrimSpace(runGit(t, dir, "rev-parse", DefaultRef))
	committed := rawTree(t, dir, newHead)
	if _, leaked := committed["garbage/entry.md"]; leaked {
		t.Fatalf("garbage index entry leaked into the commit: %v", committed)
	}
	sameTree(t, "committed tree vs in-memory tree", committed, s.Tree())
	if committed["README.md"] == "" || len(rawEvents(committed)) != 2 {
		t.Fatalf("committed tree lost real files: %v", committed)
	}
}

// A mode-0 removal through the index drops the path from the
// committed tree (the shape D9 compaction will use); the replica and
// git agree afterwards.
func TestCommitRemovesPathsThroughIndex(t *testing.T) {
	s, rg, dir := newRecordedStore(t)
	if err := s.AppendBatch(Batch{
		Events: []event.Event{newEvent(t, 0)},
		Files:  map[string][]byte{"views/a.md": []byte("a"), "views/b.md": []byte("b"), "keep.md": []byte("k")},
	}); err != nil {
		t.Fatal(err)
	}
	rg.reset()

	commit, err := s.Commit(map[string]string{
		"views/a.md":  "",
		"views/b.md":  "",
		"never/there": "", // deleting an absent path is a no-op, not an error
	}, nil, "tuhdoo: compaction rehearsal\n")
	if err != nil {
		t.Fatalf("Commit with removals: %v", err)
	}
	got := rg.snapshot()
	if got["HashObject"] != 0 || got["UpdateIndex"] != 1 || got["WriteTree"] != 1 || got["CommitTree"] != 1 || got["UpdateRef"] != 1 {
		t.Fatalf("removal commit calls = %v, want no HashObject and one each of UpdateIndex/WriteTree/CommitTree/UpdateRef", got)
	}
	tree := rawTree(t, dir, commit)
	if _, ok := tree["views/a.md"]; ok {
		t.Fatalf("views/a.md survived the removal: %v", tree)
	}
	if _, ok := tree["views/b.md"]; ok {
		t.Fatalf("views/b.md survived the removal: %v", tree)
	}
	if tree["keep.md"] == "" || len(rawEvents(tree)) != 1 {
		t.Fatalf("removal commit lost unrelated files: %v", tree)
	}
	sameTree(t, "replica vs git", tree, s.Tree())
	if head := strings.TrimSpace(runGit(t, dir, "rev-parse", DefaultRef)); head != commit || s.Head() != commit {
		t.Fatalf("ref %s / Head() %s, want the removal commit %s", head, s.Head(), commit)
	}
}

// Commit with extra parents writes a merge commit — the syncer's
// union merge — whose first parent is the head and whose second is
// the given one, and moves the ref and the replica together.
func TestCommitWithExtraParentWritesMerge(t *testing.T) {
	s, _, dir := newRecordedStore(t)
	if err := s.AppendBatch(Batch{Events: []event.Event{newEvent(t, 0)}}); err != nil {
		t.Fatal(err)
	}
	local := s.Head()
	// A commit no ref points at, standing in for a fetched remote head.
	other := strings.TrimSpace(runGit(t, dir, "commit-tree", local+"^{tree}", "-m", "elsewhere"))

	blob, err := s.git.HashObject([]byte("merged\n"))
	if err != nil {
		t.Fatal(err)
	}
	merge, err := s.Commit(map[string]string{"merged.md": blob}, []string{other}, "tuhdoo: merge\n")
	if err != nil {
		t.Fatalf("merge Commit: %v", err)
	}
	parents := strings.Fields(runGit(t, dir, "log", "-1", "--format=%P", merge))
	if len(parents) != 2 || parents[0] != local || parents[1] != other {
		t.Fatalf("merge parents = %v, want [%s %s]", parents, local, other)
	}
	if s.Head() != merge || strings.TrimSpace(runGit(t, dir, "rev-parse", DefaultRef)) != merge {
		t.Fatalf("ref and replica must both be at the merge %s: Head() %s", merge, s.Head())
	}
	if s.Tree()["merged.md"] != blob {
		t.Fatalf("replica tree lacks the merged file: %v", s.Tree())
	}
}

// FastForward moves the ref from the replica's head to the given
// commit and reloads: the replica serves the adopted commit's events
// with no further git on read. From an unloaded store (head "") it is
// the must-not-exist create that clone-join adoption uses.
func TestFastForwardMovesRefAndReloads(t *testing.T) {
	s, rg, dir := newRecordedStore(t)
	if err := s.AppendBatch(Batch{Events: []event.Event{newEvent(t, 0)}}); err != nil {
		t.Fatal(err)
	}
	// Advance the branch behind the replica's back with a plain second
	// store, then fast-forward the first onto it.
	real, err := gitx.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	other := New(real, "", testIdent)
	if err := other.AppendBatch(Batch{Events: []event.Event{newEvent(t, 1)}}); err != nil {
		t.Fatal(err)
	}
	// The ref already moved past s.head, so this CAS loses at git,
	// reloads, and reports the loss.
	target := other.Head()
	stale := s.Head()
	err = s.FastForward(stale, target)
	if !errors.Is(err, gitx.ErrRefCASFailed) {
		t.Fatalf("FastForward from a stale head = %v, want ErrRefCASFailed", err)
	}
	if s.Head() != target {
		t.Fatalf("after the lost CAS the replica should have reloaded to %s, has %s", target, s.Head())
	}
	// A caller whose anchor is older than the replica's own head — a
	// local commit landed since it looked — is refused before git is
	// touched: moving the ref from the newer head would orphan that
	// commit.
	rg.reset()
	if err := s.FastForward(stale, target); !errors.Is(err, gitx.ErrRefCASFailed) {
		t.Fatalf("FastForward anchored on an overtaken head = %v, want ErrRefCASFailed", err)
	}
	if n := rg.total(); n != 0 {
		t.Fatalf("a refused fast-forward touched git %d times: %v", n, rg.snapshot())
	}

	// A genuine fast-forward: a commit built on the head, no ref yet.
	tree := strings.TrimSpace(runGit(t, dir, "rev-parse", target+"^{tree}"))
	next := strings.TrimSpace(runGit(t, dir, "commit-tree", tree, "-p", target, "-m", "fetched"))
	rg.reset()
	if err := s.FastForward(target, next); err != nil {
		t.Fatalf("FastForward: %v", err)
	}
	if s.Head() != next || strings.TrimSpace(runGit(t, dir, "rev-parse", DefaultRef)) != next {
		t.Fatalf("FastForward left ref/replica at %s / %s, want %s", strings.TrimSpace(runGit(t, dir, "rev-parse", DefaultRef)), s.Head(), next)
	}
	got := rg.snapshot()
	if got["UpdateRef"] != 1 || got["ReadTree"] != 1 {
		t.Fatalf("FastForward calls = %v, want one UpdateRef and one ReadTree (the reseed)", got)
	}
	rg.reset()
	events, err := s.LoadEvents()
	if err != nil || len(events) != 2 {
		t.Fatalf("events after fast-forward = %d, %v; want 2", len(events), err)
	}
	if rg.total() != 0 {
		t.Fatalf("read after fast-forward spawned git: %v", rg.snapshot())
	}

	// From nothing: a fresh repository, a store that never loaded.
	dir2 := t.TempDir()
	runGit(t, dir2, "init", "--quiet", "-b", "main")
	g2, err := gitx.New(dir2)
	if err != nil {
		t.Fatal(err)
	}
	runGit(t, dir2, "fetch", "--quiet", dir, DefaultRef+":refs/tuhdoo/remote")
	fresh := New(g2, "", testIdent)
	if err := fresh.FastForward("", next); err != nil {
		t.Fatalf("FastForward on an unloaded store (adopt): %v", err)
	}
	if fresh.Head() != next {
		t.Fatalf("adopted head = %s, want %s", fresh.Head(), next)
	}
	if events, err := fresh.LoadEvents(); err != nil || len(events) != 2 {
		t.Fatalf("adopted events = %d, %v; want 2", len(events), err)
	}
}

// Init goes through the same ref mover: the created root is loaded,
// and a lost creation race leaves the replica on the winner's root.
func TestInitLoadsTheRootItCreates(t *testing.T) {
	s, dir := newStore(t)
	if s.Head() == "" || s.Head() != strings.TrimSpace(runGit(t, dir, "rev-parse", DefaultRef)) {
		t.Fatalf("Init left Head() = %q, ref at %s", s.Head(), strings.TrimSpace(runGit(t, dir, "rev-parse", DefaultRef)))
	}
	if len(s.Tree()) != 0 {
		t.Fatalf("root tree = %v, want empty", s.Tree())
	}
}
