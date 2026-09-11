package daemon

// The daemon over the live replica (001 D2, 2026-09-10): once loaded,
// reads never spawn git — every GET handler and every MCP read tool
// answers from the store's memory — and a ref moved from outside the
// daemon is noticed by the next sync cycle, remote or not.

import (
	"crypto/rand"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brandonbews/tuhdoo/internal/event"
	"github.com/brandonbews/tuhdoo/internal/gitx"
	"github.com/brandonbews/tuhdoo/internal/store"
)

// countingGit wraps the real git and counts every call by method.
type countingGit struct {
	gitx.Git
	mu    sync.Mutex
	calls map[string]int
}

func newCountingGit(g gitx.Git) *countingGit {
	return &countingGit{Git: g, calls: make(map[string]int)}
}

func (c *countingGit) count(method string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls[method]++
}

// snapshot copies the per-method counts so far.
func (c *countingGit) snapshot() map[string]int {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]int, len(c.calls))
	for k, v := range c.calls {
		out[k] = v
	}
	return out
}

func (c *countingGit) total() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, v := range c.calls {
		n += v
	}
	return n
}

func (c *countingGit) HashObject(data []byte) (string, error) {
	c.count("HashObject")
	return c.Git.HashObject(data)
}
func (c *countingGit) MkTree(entries []gitx.TreeEntry) (string, error) {
	c.count("MkTree")
	return c.Git.MkTree(entries)
}
func (c *countingGit) CommitTree(tree string, parents []string, ident gitx.Identity, msg string) (string, error) {
	c.count("CommitTree")
	return c.Git.CommitTree(tree, parents, ident, msg)
}
func (c *countingGit) UpdateRef(ref, newOID, oldOID string) error {
	c.count("UpdateRef")
	return c.Git.UpdateRef(ref, newOID, oldOID)
}
func (c *countingGit) ReadRef(ref string) (string, error) {
	c.count("ReadRef")
	return c.Git.ReadRef(ref)
}
func (c *countingGit) CatFile(oid string) ([]byte, error) {
	c.count("CatFile")
	return c.Git.CatFile(oid)
}
func (c *countingGit) CatFiles(oids []string) (map[string][]byte, error) {
	c.count("CatFiles")
	return c.Git.CatFiles(oids)
}
func (c *countingGit) LsTree(rev string) ([]gitx.TreeEntry, error) {
	c.count("LsTree")
	return c.Git.LsTree(rev)
}
func (c *countingGit) ReadTree(tree string) error {
	c.count("ReadTree")
	return c.Git.ReadTree(tree)
}
func (c *countingGit) UpdateIndex(entries []gitx.TreeEntry) error {
	c.count("UpdateIndex")
	return c.Git.UpdateIndex(entries)
}
func (c *countingGit) WriteTree() (string, error) {
	c.count("WriteTree")
	return c.Git.WriteTree()
}
func (c *countingGit) IsAncestor(a, b string) (bool, error) {
	c.count("IsAncestor")
	return c.Git.IsAncestor(a, b)
}
func (c *countingGit) Fetch(remote, refspec string) error {
	c.count("Fetch")
	return c.Git.Fetch(remote, refspec)
}
func (c *countingGit) FetchTimeout(remote, refspec string, timeout time.Duration) error {
	c.count("FetchTimeout")
	return c.Git.FetchTimeout(remote, refspec, timeout)
}
func (c *countingGit) Push(remote, refspec string) error {
	c.count("Push")
	return c.Git.Push(remote, refspec)
}
func (c *countingGit) RemoteURL(remote string) (string, error) {
	c.count("RemoteURL")
	return c.Git.RemoteURL(remote)
}

// newCountedRepo builds a fresh repository and a counting wrapper over
// its real git, for daemons started with the Options.git seam.
func newCountedRepo(t *testing.T) (string, *countingGit) {
	t.Helper()
	setGitEnv(t)
	root := shortTempDir(t)
	runGit(t, root, "init", "--quiet", "-b", "main")
	real, err := gitx.New(root)
	if err != nil {
		t.Fatalf("gitx.New: %v", err)
	}
	return root, newCountingGit(real)
}

// Every GET handler and every MCP read tool issues zero git calls once
// the daemon is loaded: the state snapshot, a task hydration, the
// backlog tool, and the task tool are all answered from the replica.
func TestReadsSpawnNoGitOnceLoaded(t *testing.T) {
	root, cg := newCountedRepo(t)
	d, c := startDaemonAt(t, root, Options{
		Quiet:        50 * time.Millisecond,
		SyncInterval: time.Hour, // the first cycle runs at once; no second one during the test
		Log:          log.New(io.Discard, "", 0),
		git:          cg,
	})
	cs := mcpConnect(t, d, "brandon/impl-1", nil)

	// Some state to read, landed on the branch, and the first sync
	// cycle finished (it sets the mode as its last act).
	id := createOne(t, c, "brandon", map[string]any{"title": "read me without git"})
	if err := d.batcher.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for d.sync.Status().Mode != "local-only" {
		if time.Now().After(deadline) {
			t.Fatalf("sync mode = %q after 5s, want local-only", d.sync.Status().Mode)
		}
		time.Sleep(10 * time.Millisecond)
	}
	before := cg.total()

	for i := 0; i < 3; i++ {
		var st stateResp
		unmarshalInto(t, mustDo(t, c, "GET", "/v0/state", "", nil, http.StatusOK), &st)
		if !st.Loaded || len(st.Tasks) != 1 {
			t.Fatalf("state = loaded %v, %d tasks; want loaded with 1 task", st.Loaded, len(st.Tasks))
		}
		var h hydratedTask
		unmarshalInto(t, mustDo(t, c, "GET", "/v0/tasks/"+id, "", nil, http.StatusOK), &h)
		if h.Task.ID != id {
			t.Fatalf("GET task = %+v, want %s", h.Task, id)
		}
		var backlog backlogResult
		mustToolOK(t, cs, "get_backlog", map[string]any{}, &backlog)
		if len(backlog.Ready) != 1 || backlog.Ready[0].ID != id {
			t.Fatalf("get_backlog = %+v, want the one task", backlog.Ready)
		}
		var got hydratedTask
		mustToolOK(t, cs, "get_task", map[string]any{"task": id}, &got)
		if got.Task.ID != id {
			t.Fatalf("get_task = %+v, want %s", got.Task, id)
		}
	}
	if after := cg.total(); after != before {
		t.Fatalf("reads issued %d git calls: %v", after-before, cg.snapshot())
	}
}

// A ref moved from outside the daemon — here a second writer on the
// same repository, the shape of a `branch -f` or a manual fetch — is
// noticed by the next sync cycle with no remote configured: the store
// reloads (reseeding its index) and the daemon's state reflects the
// moved ref.
func TestSyncCycleNoticesExternalRefMove(t *testing.T) {
	root, cg := newCountedRepo(t)
	d, err := New(root, Options{Quiet: 50 * time.Millisecond, Log: log.New(io.Discard, "", 0), git: cg})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { d.Shutdown("test cleanup") })
	loadOnly(t, d) // no Run: cycles are driven by hand

	ids, _, oe := d.opCreateTasks("brandon", []createTaskItem{{Title: "mine"}})
	if oe != nil {
		t.Fatalf("create: %v", oe)
	}
	if err := d.batcher.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if err := d.sync.Cycle(); err != nil {
		t.Fatalf("cycle: %v", err)
	}

	// Another writer advances the branch behind the daemon's back.
	real, err := gitx.New(root)
	if err != nil {
		t.Fatal(err)
	}
	other := store.New(real, "", testIdent)
	eid, err := event.NewID(time.Now(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := event.New(eid, event.TypeTaskCreated, event.Versions[event.TypeTaskCreated],
		"sarah", "m-elsewhere", "t-external", event.TaskCreated{Title: "moved in from outside"})
	if err != nil {
		t.Fatal(err)
	}
	if err := other.AppendBatch(store.Batch{Events: []event.Event{foreign}}); err != nil {
		t.Fatalf("external AppendBatch: %v", err)
	}
	ref := strings.TrimSpace(runGit(t, root, "rev-parse", "refs/heads/tuhdoo"))
	if d.store.Head() == ref {
		t.Fatal("setup: the daemon's replica must not yet know the moved ref")
	}
	d.mu.Lock()
	_, known := d.state.Tasks["t-external"]
	d.mu.Unlock()
	if known {
		t.Fatal("setup: the daemon's state must not yet carry the foreign task")
	}

	if err := d.sync.Cycle(); err != nil {
		t.Fatalf("cycle after external move: %v", err)
	}
	if d.store.Head() != ref {
		t.Fatalf("replica head = %s after the cycle, ref at %s", d.store.Head(), ref)
	}
	d.mu.Lock()
	_, known = d.state.Tasks["t-external"]
	_, mine := d.state.Tasks[ids[0]]
	d.mu.Unlock()
	if !known || !mine {
		t.Fatalf("daemon state after the cycle: foreign task known %v, own task known %v; want both", known, mine)
	}
	// The daemon's next write commits on the moved head with the
	// foreign event still in the tree — the reload reseeded the index.
	if _, _, oe := d.opCreateTasks("brandon", []createTaskItem{{Title: "after"}}); oe != nil {
		t.Fatalf("create after reload: %v", oe)
	}
	if err := d.batcher.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	events, err := d.store.LoadEvents()
	if err != nil {
		t.Fatal(err)
	}
	var tasks []string
	for _, e := range events {
		tasks = append(tasks, e.Task)
	}
	if len(events) != 3 || !strings.Contains(strings.Join(tasks, " "), "t-external") {
		t.Fatalf("events after the write = %v, want 3 including t-external", tasks)
	}
}
