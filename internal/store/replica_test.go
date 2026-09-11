package store

// The store as live replica (001 D2 / 002 T2, 2026-09-10): head and
// tree in memory, the private index reseeded by the first commit after
// every load and reload (never by a load itself), blobs written only
// when their bytes changed, and the store as the single mover of the
// ref. Real git underneath a recording wrapper
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
// OIDs CatFiles was asked for, and the order of every call), so a test
// can say exactly which subprocesses an operation cost — and that a
// read cost none.
type recordingGit struct {
	gitx.Git
	mu       sync.Mutex
	calls    map[string]int
	seq      []string // method names in call order
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
	r.seq = append(r.seq, method)
}

// reset clears the counts; the wrapped git is untouched.
func (r *recordingGit) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = make(map[string]int)
	r.seq = nil
	r.readTree = nil
	r.catOIDs = nil
}

// sequence copies the ordered call log.
func (r *recordingGit) sequence() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.seq...)
}

// indexOf is the position of method's first call in the log, or -1.
func (r *recordingGit) indexOf(method string) int {
	for i, m := range r.sequence() {
		if m == method {
			return i
		}
	}
	return -1
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

// externalCommit advances the data branch behind every Store's back
// with raw plumbing through a fresh gitx CLI — hash-object, mktree,
// commit-tree, update-ref — the way a foreign process (a `branch -f`,
// another machine's daemon) moves the ref: no Store, and so no touch
// of the private index. Returns the new head.
func externalCommit(t *testing.T, dir string, files map[string][]byte) string {
	t.Helper()
	g, err := gitx.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	head, err := g.ReadRef(DefaultRef)
	if err != nil {
		t.Fatalf("externalCommit: %v", err)
	}
	tree, err := gitx.LsTreeMap(g, head)
	if err != nil {
		t.Fatalf("externalCommit: %v", err)
	}
	for path, data := range files {
		oid, err := g.HashObject(data)
		if err != nil {
			t.Fatalf("externalCommit: %v", err)
		}
		tree[path] = oid
	}
	treeOID, err := gitx.MkTreeFromMap(g, tree)
	if err != nil {
		t.Fatalf("externalCommit: %v", err)
	}
	commit, err := g.CommitTree(treeOID, []string{head}, gitx.Identity{Name: "outsider", Email: "outsider@test.invalid"}, "moved from outside\n")
	if err != nil {
		t.Fatalf("externalCommit: %v", err)
	}
	if err := g.UpdateRef(DefaultRef, commit, head); err != nil {
		t.Fatalf("externalCommit: %v", err)
	}
	return commit
}

// encodedEvent is e's stored bytes at its event path.
func encodedEvent(t *testing.T, e event.Event) (string, []byte) {
	t.Helper()
	path, err := event.Path(e.ID)
	if err != nil {
		t.Fatal(err)
	}
	data, err := event.Encode(e)
	if err != nil {
		t.Fatal(err)
	}
	return path, data
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

// movingGit moves the ref behind the store's back — a raw-plumbing
// commit on the same repository, no Store, so the private index is
// untouched by the mover — just before delegating the first UpdateRef,
// so the store's compare-and-swap loses for real.
type movingGit struct {
	gitx.Git
	t       *testing.T
	dir     string
	theirs  map[string][]byte
	moved   string // the head the mover created
	updates int
}

func (m *movingGit) UpdateRef(ref, newOID, oldOID string) error {
	m.updates++
	if m.updates == 1 {
		m.moved = externalCommit(m.t, m.dir, m.theirs)
	}
	return m.Git.UpdateRef(ref, newOID, oldOID)
}

// A lost compare-and-swap reloads head and tree from git, and the
// retry reseeds the index from the moved head before rebuilding on
// it: the resulting tree carries both the concurrent change and ours.
// This is the trap the design revision names — an index not reseeded
// after the reload would still hold our stale tree and the concurrent
// event would vanish from the commit (write-tree writes whatever the
// index holds). The mover is raw plumbing on purpose: a second Store
// as the mover would reseed the shared index with its own commit and
// mask a missing reseed here.
func TestAppendBatchRetriesOnLostCASAndKeepsBothChanges(t *testing.T) {
	s, rg, dir := newRecordedStore(t)
	if err := s.AppendBatch(Batch{Events: []event.Event{newEvent(t, 0)}}); err != nil {
		t.Fatal(err)
	}

	theirs := newEvent(t, 99)
	theirPath, theirBytes := encodedEvent(t, theirs)
	mg := &movingGit{Git: rg, t: t, dir: dir, theirs: map[string][]byte{theirPath: theirBytes}}
	racy := New(mg, "", testIdent)
	if err := racy.Load(); err != nil {
		t.Fatal(err)
	}
	loaded := racy.Head()
	rg.reset()

	ours := newEvent(t, 1)
	if err := racy.AppendBatch(Batch{Events: []event.Event{ours}}); err != nil {
		t.Fatalf("AppendBatch across a lost CAS: %v", err)
	}
	if mg.updates != 2 {
		t.Fatalf("UpdateRef called %d times, want 2 (loss then retry)", mg.updates)
	}
	got := rg.snapshot()
	if got["ReadRef"] != 1 || got["LsTree"] != 1 {
		t.Fatalf("lost CAS should reload exactly once (ReadRef, LsTree = 1 each): %v", got)
	}
	// Two reseeds: the first commit after Load, and the retry after the
	// reload — each from the head the attempt builds on.
	if len(rg.readTree) != 2 || rg.readTree[0] != loaded || rg.readTree[1] != mg.moved {
		t.Fatalf("ReadTree calls = %v, want [%s %s] (loaded head, then the moved head)", rg.readTree, loaded, mg.moved)
	}

	head := strings.TrimSpace(runGit(t, dir, "rev-parse", DefaultRef))
	tree := rawTree(t, dir, head)
	oursPath, _ := event.Path(ours.ID)
	if _, ok := tree[oursPath]; !ok {
		t.Fatalf("committed tree lacks our event %s: %v", oursPath, tree)
	}
	if _, ok := tree[theirPath]; !ok {
		t.Fatalf("committed tree lacks the mover's event %s: %v", theirPath, tree)
	}
	if n := len(rawEvents(tree)); n != 3 {
		t.Fatalf("committed tree carries %d events, want 3 (seed, concurrent, ours): %v", n, tree)
	}
	// The replica agrees with git after the retry, and its parent is
	// the moved head.
	if racy.Head() != head {
		t.Fatalf("Head() = %s, ref at %s", racy.Head(), head)
	}
	sameTree(t, "replica vs git", tree, racy.Tree())
	if parents := strings.Fields(runGit(t, dir, "log", "-1", "--format=%P", head)); len(parents) != 1 || parents[0] != mg.moved {
		t.Fatalf("retried commit parents = %v, want [%s]", parents, mg.moved)
	}
}

// The retry drops the batch's rendered files: they were computed
// against the state the reload superseded, and landing them could
// overwrite pages a newer generator stamped on the moved head (T6).
// Events and leases ride the retry; views wait for the next render.
func TestAppendBatchRetryDropsRenderedFiles(t *testing.T) {
	s, rg, dir := newRecordedStore(t)
	if err := s.AppendBatch(Batch{Events: []event.Event{newEvent(t, 0)}, Files: map[string][]byte{"backlog.md": []byte("v1")}}); err != nil {
		t.Fatal(err)
	}
	mg := &movingGit{Git: rg, t: t, dir: dir, theirs: map[string][]byte{"backlog.md": []byte("rendered by the mover")}}
	racy := New(mg, "", testIdent)
	if err := racy.Load(); err != nil {
		t.Fatal(err)
	}
	ours := newEvent(t, 1)
	err := racy.AppendBatch(Batch{
		Events: []event.Event{ours},
		Files:  map[string][]byte{"backlog.md": []byte("v2, stale"), "leases/c1.json": encodeLease(time.Now().Add(time.Hour))},
	})
	if err != nil {
		t.Fatalf("AppendBatch across a lost CAS: %v", err)
	}
	head := strings.TrimSpace(runGit(t, dir, "rev-parse", DefaultRef))
	tree := rawTree(t, dir, head)
	oursPath, _ := event.Path(ours.ID)
	if _, ok := tree[oursPath]; !ok {
		t.Fatalf("committed tree lacks our event: %v", tree)
	}
	if _, ok := tree["leases/c1.json"]; !ok {
		t.Fatalf("committed tree lacks our lease: %v", tree)
	}
	if got := runGit(t, dir, "cat-file", "-p", tree["backlog.md"]); got != "rendered by the mover" {
		t.Fatalf("backlog.md after the retry = %q, want the mover's render left alone", got)
	}
	sameTree(t, "replica vs git", tree, racy.Tree())
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

func TestAppendBatchExhaustsCASRetries(t *testing.T) {
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

// A merge-shaped Commit never retries: anchored on the head the caller
// computed against, a stale base is refused before any git runs with
// an error matching ErrRefCASFailed (the syncer's next pass merges
// afresh), and a compare-and-swap lost at the ref reloads the replica
// and reports the loss the same way — one UpdateRef, no second try.
func TestCommitIsAnchoredAndNeverRetries(t *testing.T) {
	s, rg, dir := newRecordedStore(t)
	if err := s.AppendBatch(Batch{Events: []event.Event{newEvent(t, 0)}}); err != nil {
		t.Fatal(err)
	}
	stale := s.Head()
	if err := s.AppendBatch(Batch{Events: []event.Event{newEvent(t, 1)}}); err != nil {
		t.Fatal(err)
	}
	blob, err := s.git.HashObject([]byte("merged\n"))
	if err != nil {
		t.Fatal(err)
	}
	other := strings.TrimSpace(runGit(t, dir, "commit-tree", stale+"^{tree}", "-m", "elsewhere"))

	rg.reset()
	_, err = s.Commit(stale, map[string]string{"merged.md": blob}, []string{other}, "tuhdoo: merge\n")
	if !errors.Is(err, gitx.ErrRefCASFailed) {
		t.Fatalf("Commit on a stale base = %v, want ErrRefCASFailed", err)
	}
	if n := rg.total(); n != 0 {
		t.Fatalf("a refused Commit touched git %d times: %v", n, rg.snapshot())
	}

	// Anchored correctly but the ref moves under it: the loss is
	// reported after one UpdateRef, and the replica already follows
	// the moved ref.
	theirs := newEvent(t, 99)
	theirPath, theirBytes := encodedEvent(t, theirs)
	mg := &movingGit{Git: rg, t: t, dir: dir, theirs: map[string][]byte{theirPath: theirBytes}}
	racy := New(mg, "", testIdent)
	if err := racy.Load(); err != nil {
		t.Fatal(err)
	}
	base := racy.Head()
	_, err = racy.Commit(base, map[string]string{"merged.md": blob}, []string{other}, "tuhdoo: merge\n")
	if !errors.Is(err, gitx.ErrRefCASFailed) {
		t.Fatalf("Commit across a lost CAS = %v, want ErrRefCASFailed", err)
	}
	if mg.updates != 1 {
		t.Fatalf("UpdateRef called %d times, want exactly 1 (no retry)", mg.updates)
	}
	if racy.Head() != mg.moved {
		t.Fatalf("after the lost CAS the replica is at %s, want the moved head %s", racy.Head(), mg.moved)
	}
	if got := strings.TrimSpace(runGit(t, dir, "rev-parse", DefaultRef)); got != mg.moved {
		t.Fatalf("ref at %s after the lost CAS, want the mover's %s untouched", got, mg.moved)
	}
	if _, ok := racy.Tree()["merged.md"]; ok {
		t.Fatal("the refused merge's file reached the replica tree")
	}
}

// The first commit after Load reseeds the private index from the head
// — mandatory, not an optimization: an index pre-populated with
// entries from some other tree (and a stale lock left by a killed git)
// must not leak into the next commit. Load itself leaves the index and
// its lock alone: a read-only Load beside a running daemon (the
// harness's LoadReplayInput) must not rewrite the daemon's index. The
// committed tree equals the in-memory tree map exactly, checked by
// ls-tree.
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
	if got := rg.snapshot(); got["ReadTree"] != 0 || got["UpdateIndex"] != 0 || got["WriteTree"] != 0 {
		t.Fatalf("Load touched the private index: %v; a load must not (only a commit may)", got)
	}
	if _, err := os.Stat(indexPath + ".lock"); err != nil {
		t.Fatalf("Load removed the index.lock (stat err %v); clearing it is the committing process's job", err)
	}

	if err := s.AppendBatch(Batch{Events: []event.Event{newEvent(t, 1)}}); err != nil {
		t.Fatalf("AppendBatch after Load: %v", err)
	}
	if len(rg.readTree) != 1 || rg.readTree[0] != head {
		t.Fatalf("the first commit after Load reseeded with ReadTree %v, want exactly one with the head %s", rg.readTree, head)
	}
	if rt, ui := rg.indexOf("ReadTree"), rg.indexOf("UpdateIndex"); rt < 0 || ui < 0 || rt > ui {
		t.Fatalf("ReadTree (call %d) must precede UpdateIndex (call %d): %v", rt, ui, rg.sequence())
	}
	if _, err := os.Stat(indexPath + ".lock"); !os.IsNotExist(err) {
		t.Fatalf("stale index.lock survived the reseed (stat err %v)", err)
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

// failOnceGit fails the first UpdateRef with an error that is not a
// compare-and-swap loss — the ref is untouched, but the index was
// already fed the attempt's entries.
type failOnceGit struct {
	gitx.Git
	updates int
}

func (g *failOnceGit) UpdateRef(ref, newOID, oldOID string) error {
	g.updates++
	if g.updates == 1 {
		return errors.New("simulated: update-ref failed for a reason that is not a lost CAS")
	}
	return g.Git.UpdateRef(ref, newOID, oldOID)
}

// A commit that fails after the index was touched leaves the index
// holding entries no commit landed: the next commit reseeds from the
// head first, so its tree is exactly the in-memory tree — without the
// failed attempt's file — checked by ls-tree.
func TestFailedCommitReseedsBeforeNextCommit(t *testing.T) {
	s, rg, dir := newRecordedStore(t)
	if err := s.AppendBatch(Batch{Events: []event.Event{newEvent(t, 0)}}); err != nil {
		t.Fatal(err)
	}
	fg := &failOnceGit{Git: rg}
	flaky := New(fg, "", testIdent)
	if err := flaky.Load(); err != nil {
		t.Fatal(err)
	}
	head := flaky.Head()

	lost := newEvent(t, 1)
	err := flaky.AppendBatch(Batch{Events: []event.Event{lost}})
	if err == nil || errors.Is(err, gitx.ErrRefCASFailed) {
		t.Fatalf("AppendBatch under a non-CAS UpdateRef failure = %v, want a plain error", err)
	}
	if flaky.Head() != head || strings.TrimSpace(runGit(t, dir, "rev-parse", DefaultRef)) != head {
		t.Fatalf("a failed commit moved the head: replica %s, ref %s, want %s", flaky.Head(), strings.TrimSpace(runGit(t, dir, "rev-parse", DefaultRef)), head)
	}
	rg.reset()

	kept := newEvent(t, 2)
	if err := flaky.AppendBatch(Batch{Events: []event.Event{kept}}); err != nil {
		t.Fatalf("AppendBatch after a failed commit: %v", err)
	}
	if len(rg.readTree) != 1 || rg.readTree[0] != head {
		t.Fatalf("the commit after a failure reseeded with ReadTree %v, want exactly one with the head %s", rg.readTree, head)
	}
	if rt, ui := rg.indexOf("ReadTree"), rg.indexOf("UpdateIndex"); rt < 0 || ui < 0 || rt > ui {
		t.Fatalf("ReadTree (call %d) must precede UpdateIndex (call %d): %v", rt, ui, rg.sequence())
	}
	newHead := strings.TrimSpace(runGit(t, dir, "rev-parse", DefaultRef))
	committed := rawTree(t, dir, newHead)
	sameTree(t, "committed tree vs in-memory tree", committed, flaky.Tree())
	lostPath, _ := event.Path(lost.ID)
	if _, leaked := committed[lostPath]; leaked {
		t.Fatalf("the failed attempt's event %s leaked from the index into the next commit: %v", lostPath, committed)
	}
	keptPath, _ := event.Path(kept.ID)
	if committed[keptPath] == "" || len(rawEvents(committed)) != 2 {
		t.Fatalf("committed tree = %v, want the seed and %s", committed, keptPath)
	}
}

// An index file that vanished between commits is not recreated empty
// by git behind our back (that tree would drop every file on the
// branch): UpdateIndex refuses it, the commit reseeds from the head
// and retries the attempt once, and the tree matches memory.
func TestCommitReseedsWhenIndexVanishes(t *testing.T) {
	s, rg, dir := newRecordedStore(t)
	if err := s.AppendBatch(Batch{
		Events: []event.Event{newEvent(t, 0)},
		Files:  map[string][]byte{"README.md": []byte("real\n")},
	}); err != nil {
		t.Fatal(err)
	}
	head := s.Head()
	if err := os.Remove(filepath.Join(dir, ".git", "tuhdoo", "index")); err != nil {
		t.Fatal(err)
	}
	rg.reset()

	if err := s.AppendBatch(Batch{Events: []event.Event{newEvent(t, 1)}}); err != nil {
		t.Fatalf("AppendBatch after the index vanished: %v", err)
	}
	got := rg.snapshot()
	if got["UpdateIndex"] != 2 || len(rg.readTree) != 1 || rg.readTree[0] != head || got["WriteTree"] != 1 || got["UpdateRef"] != 1 {
		t.Fatalf("calls = %v, ReadTree %v; want UpdateIndex twice around one ReadTree(%s), one WriteTree, one UpdateRef", got, rg.readTree, head)
	}
	newHead := strings.TrimSpace(runGit(t, dir, "rev-parse", DefaultRef))
	committed := rawTree(t, dir, newHead)
	sameTree(t, "committed tree vs in-memory tree", committed, s.Tree())
	if committed["README.md"] == "" || len(rawEvents(committed)) != 2 {
		t.Fatalf("committed tree lost files to an empty index: %v", committed)
	}
}

// A change set whose result is not a tree — a blob where another path
// needs a directory, or under a path that holds a blob — is refused
// before any git runs. `update-index --index-info` would silently drop
// the entries in the way (a blob at "events" wipes every event).
func TestCommitRejectsFileDirectoryConflicts(t *testing.T) {
	const oid = "0123456789abcdef0123456789abcdef01234567"
	base := map[string]string{"events/a": oid, "views/t": oid}
	cases := []struct {
		name    string
		changes map[string]string
		wantErr []string // substrings: both paths in the conflict
		entries int
	}{
		{"blob where a directory stands", map[string]string{"events": oid}, []string{`"events"`, `"events/a"`}, 0},
		{"blob under a blob", map[string]string{"views/t/x": oid}, []string{`"views/t/x"`, `"views/t"`}, 0},
		{"two added paths conflict", map[string]string{"new": oid, "new/x": oid}, []string{`"new"`, `"new/x"`}, 0},
		{"plain add", map[string]string{"views/u.md": oid}, nil, 1},
		{"delete makes room", map[string]string{"views/t": "", "views/t/x": oid}, nil, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree, entries, err := applyChanges(base, tc.changes)
			if len(tc.wantErr) > 0 {
				if err == nil {
					t.Fatalf("applyChanges accepted %v: tree %v", tc.changes, tree)
				}
				for _, want := range tc.wantErr {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("error %q does not name %s", err, want)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("applyChanges(%v): %v", tc.changes, err)
			}
			if len(entries) != tc.entries {
				t.Fatalf("entries = %v, want %d", entries, tc.entries)
			}
		})
	}

	// Through the real thing: the refused commit calls no git, moves
	// nothing, and leaves the index usable for the next commit.
	s, rg, dir := newRecordedStore(t)
	if err := s.AppendBatch(Batch{Events: []event.Event{newEvent(t, 0)}}); err != nil {
		t.Fatal(err)
	}
	head := s.Head()
	blob, err := s.git.HashObject([]byte("a blob where the events directory stands\n"))
	if err != nil {
		t.Fatal(err)
	}
	rg.reset()
	_, err = s.Commit(head, map[string]string{"events": blob}, nil, "tuhdoo: corrupt\n")
	if err == nil || !strings.Contains(err.Error(), `"events"`) {
		t.Fatalf("Commit of a blob at events/ = %v, want a refusal naming the path", err)
	}
	if n := rg.total(); n != 0 {
		t.Fatalf("the refused commit touched git %d times: %v", n, rg.snapshot())
	}
	if got := strings.TrimSpace(runGit(t, dir, "rev-parse", DefaultRef)); got != head || s.Head() != head {
		t.Fatalf("ref %s / Head() %s after the refusal, want %s unchanged", got, s.Head(), head)
	}
	if err := s.AppendBatch(Batch{Events: []event.Event{newEvent(t, 1)}}); err != nil {
		t.Fatalf("AppendBatch after a refused commit: %v", err)
	}
	committed := rawTree(t, dir, strings.TrimSpace(runGit(t, dir, "rev-parse", DefaultRef)))
	sameTree(t, "committed tree vs in-memory tree", committed, s.Tree())
	if len(rawEvents(committed)) != 2 {
		t.Fatalf("events after the refusal and a good commit = %v, want 2", committed)
	}
}

// The decode caches survive a reload-and-retry: a batch's lease that
// was inserted before a lost compare-and-swap would be pruned by the
// reload (the reloaded tree does not hold it yet), so the caches are
// filled once the commit lands — and the next ReplayInput reads no
// blobs from git.
func TestCachesSurviveReloadAndRetry(t *testing.T) {
	s, rg, _ := newRecordedStore(t)
	if err := s.AppendBatch(Batch{Events: []event.Event{newEvent(t, 0)}}); err != nil {
		t.Fatal(err)
	}
	lo := &casLoseOnceGit{Git: rg}
	racy := New(lo, "", testIdent)
	if err := racy.Load(); err != nil {
		t.Fatal(err)
	}
	expires := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	if err := racy.AppendBatch(Batch{
		Events: []event.Event{newEvent(t, 1)},
		Files:  map[string][]byte{"leases/c1.json": encodeLease(expires)},
	}); err != nil {
		t.Fatalf("AppendBatch across a lost CAS: %v", err)
	}
	if lo.updates != 2 {
		t.Fatalf("UpdateRef called %d times, want 2 (loss then retry)", lo.updates)
	}
	rg.reset()
	events, leases, err := racy.ReplayInput()
	if err != nil {
		t.Fatalf("ReplayInput: %v", err)
	}
	if len(events) != 2 || !leases["c1"].Equal(expires) {
		t.Fatalf("ReplayInput = %d events, leases %v; want 2 events and c1 -> %v", len(events), leases, expires)
	}
	if got := rg.snapshot(); got["CatFiles"] != 0 || got["CatFile"] != 0 {
		t.Fatalf("ReplayInput after a retried commit read blobs from git: %v; the caches must hold the batch", got)
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

	commit, err := s.Commit(s.Head(), map[string]string{
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
	merge, err := s.Commit(local, map[string]string{"merged.md": blob}, []string{other}, "tuhdoo: merge\n")
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
	// Advance the branch behind the replica's back with raw plumbing
	// (no Store, so the private index is untouched), then fast-forward
	// the first onto it.
	theirPath, theirBytes := encodedEvent(t, newEvent(t, 1))
	target := externalCommit(t, dir, map[string][]byte{theirPath: theirBytes})
	// The ref already moved past s.head, so this CAS loses at git,
	// reloads, and reports the loss.
	stale := s.Head()
	err := s.FastForward(stale, target)
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
	if got["UpdateRef"] != 1 || got["ReadTree"] != 0 {
		t.Fatalf("FastForward calls = %v, want one UpdateRef and no ReadTree (the reseed waits for the next commit)", got)
	}
	rg.reset()
	events, err := s.LoadEvents()
	if err != nil || len(events) != 2 {
		t.Fatalf("events after fast-forward = %d, %v; want 2", len(events), err)
	}
	if rg.total() != 0 {
		t.Fatalf("read after fast-forward spawned git: %v", rg.snapshot())
	}
	// The next commit reseeds from the adopted head, so it carries the
	// mover's file as well as its own.
	if err := s.AppendBatch(Batch{Events: []event.Event{newEvent(t, 2)}}); err != nil {
		t.Fatal(err)
	}
	if len(rg.readTree) != 1 || rg.readTree[0] != next {
		t.Fatalf("commit after fast-forward reseeded with ReadTree %v, want [%s]", rg.readTree, next)
	}
	after := rawTree(t, dir, strings.TrimSpace(runGit(t, dir, "rev-parse", DefaultRef)))
	if _, ok := after[theirPath]; !ok || len(rawEvents(after)) != 3 {
		t.Fatalf("commit after fast-forward = %v, want the mover's event and 3 events in all", after)
	}
	sameTree(t, "replica vs git", after, s.Tree())

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
