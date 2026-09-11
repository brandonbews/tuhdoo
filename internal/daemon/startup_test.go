package daemon

// Startup order (T4, 2026-09-10: lock, socket, discovery file, then
// load). The socket serves while the first replay is still in flight —
// reads answer "starting", writes a retryable 503, MCP session setup
// succeeds and a tool call gets the same retryable error — and a first
// load that fails ends the daemon with its socket and discovery file
// torn down.

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brandonbews/tuhdoo/internal/gitx"
)

// gatedGit wraps the real git and holds every ReadRef until release is
// closed, so a test can look at the daemon while its first load is
// still in flight and decide exactly when it lands. ReadRef is the
// load's first git call — AdoptRemoteBranch's, made outside d.mu —
// which is what makes the gate safe: the load parks there holding no
// lock, so a request that reaches the mutex meanwhile is answered
// (with the placeholder), never deadlocked. Gating a call made under
// d.mu (LsTree, inside refreshLocked) would park the load holding the
// mutex, and the first request to want it would hang the test.
type gatedGit struct {
	gitx.Git
	release chan struct{}
}

func (g *gatedGit) ReadRef(ref string) (string, error) {
	<-g.release
	return g.Git.ReadRef(ref)
}

// brokenGit fails every LsTree: a first load the daemon cannot complete.
type brokenGit struct{ gitx.Git }

func (brokenGit) LsTree(string) ([]gitx.TreeEntry, error) {
	return nil, errors.New("simulated: object store unreadable")
}

func TestServesStartingUntilFirstReplay(t *testing.T) {
	setGitEnv(t)
	root := shortTempDir(t)
	runGit(t, root, "init", "--quiet", "-b", "main")
	real, err := gitx.New(root)
	if err != nil {
		t.Fatalf("gitx.New: %v", err)
	}
	release := make(chan struct{})
	var releaseOnce sync.Once
	letLoadFinish := func() { releaseOnce.Do(func() { close(release) }) }

	d, err := New(root, Options{
		Quiet: 50 * time.Millisecond,
		Log:   log.New(io.Discard, "", 0),
		git:   &gatedGit{Git: real, release: release},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	go d.Run()
	// Cleanups run last-registered first: Shutdown joins the load, so
	// the gate must be released before it — and before any Fatalf
	// below, so a failed assertion cannot hang the test on the gate.
	t.Cleanup(func() { d.Shutdown("test cleanup") })
	t.Cleanup(letLoadFinish)
	c := socketClient(d)

	// The socket accepts and /v0/state answers the placeholder — loaded
	// false, sync mode "starting", no tasks — while the load is held
	// open.
	var st stateResp
	unmarshalInto(t, mustDo(t, c, "GET", "/v0/state", "", nil, http.StatusOK), &st)
	if st.Loaded || st.Sync.Mode != "starting" {
		t.Fatalf("state during load: loaded %v, sync mode %q; want the placeholder (false, starting)", st.Loaded, st.Sync.Mode)
	}
	if len(st.Tasks) != 0 || st.Degraded != "" {
		t.Fatalf("state during load carries tasks/degraded: %+v", st)
	}

	// A write in the window: 503, body naming starting.
	status, body, err := do(c, "POST", "/v0/tasks", "brandon", []map[string]any{{"title": "too early"}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if status != http.StatusServiceUnavailable || !strings.Contains(string(body), "starting") {
		t.Fatalf("write during load: status %d body %s; want 503 naming starting", status, body)
	}
	// A hydration read too: there is no state to hydrate from.
	status, body, err = do(c, "GET", "/v0/tasks/tuh-nothing", "", nil)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if status != http.StatusServiceUnavailable || !strings.Contains(string(body), "starting") {
		t.Fatalf("task read during load: status %d body %s; want 503 naming starting", status, body)
	}

	// MCP: session setup needs no state and succeeds inside the window
	// (initialize, tools/list); a tool call is a retryable tool error
	// that names starting, not a protocol failure.
	cs := mcpConnect(t, d, "brandon/impl-1", nil)
	tools, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("tools/list during load: %v", err)
	}
	if len(tools.Tools) != 12 {
		t.Fatalf("tools/list during load returned %d tools, want 12", len(tools.Tools))
	}
	res := callTool(t, cs, "get_backlog", map[string]any{})
	if !res.IsError || !strings.Contains(contentText(res), "starting") {
		t.Fatalf("tool call during load = isError %v, %q; want a tool error naming starting", res.IsError, contentText(res))
	}

	// Let the load land: the same calls now succeed.
	letLoadFinish()
	waitLoaded(t, d)
	deadline := time.Now().Add(5 * time.Second)
	for {
		unmarshalInto(t, mustDo(t, c, "GET", "/v0/state", "", nil, http.StatusOK), &st)
		if st.Sync.Mode != "starting" || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !st.Loaded || st.Sync.Mode != "local-only" {
		t.Fatalf("state after load: loaded %v, sync mode %q; want true, local-only", st.Loaded, st.Sync.Mode)
	}
	id := createOne(t, c, "brandon", map[string]any{"title": "right on time"})
	mustDo(t, c, "GET", "/v0/tasks/"+id, "", nil, http.StatusOK)
	var backlog backlogResult
	mustToolOK(t, cs, "get_backlog", map[string]any{}, &backlog)
	if len(backlog.Ready) != 1 || backlog.Ready[0].ID != id {
		t.Fatalf("backlog after load = %+v, want the one task %s", backlog.Ready, id)
	}
}

// New writes daemon.json before Run loads anything (bind before load),
// and a first load that fails ends the daemon: Run returns the error,
// the reason is in the log, socket and daemon.json are gone, and the
// lock is free for a successor — so no client loops on "starting"
// against a daemon that will never load.
func TestFailedFirstLoadEndsTheDaemon(t *testing.T) {
	setGitEnv(t)
	root := shortTempDir(t)
	runGit(t, root, "init", "--quiet", "-b", "main")
	real, err := gitx.New(root)
	if err != nil {
		t.Fatalf("gitx.New: %v", err)
	}
	var logBuf bytes.Buffer
	d, err := New(root, Options{Log: log.New(&logBuf, "", 0), git: brokenGit{real}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	sock, disc := d.sockPath, d.jsonPath
	if _, err := os.Stat(disc); err != nil {
		t.Fatalf("daemon.json should exist before the load runs: %v", err)
	}

	runErr := d.Run()
	if runErr == nil {
		t.Fatal("Run returned nil after a failed first load, want the error")
	}
	if !strings.Contains(runErr.Error(), "initial load") || !strings.Contains(runErr.Error(), "unreadable") {
		t.Fatalf("Run error %q should say the initial load failed and why", runErr)
	}
	if !strings.Contains(logBuf.String(), "unreadable") || !strings.Contains(logBuf.String(), "exiting") {
		t.Fatalf("log should carry the exit reason; got:\n%s", logBuf.String())
	}
	for _, p := range []string{sock, disc} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s still exists after the failed load (stat err: %v)", p, err)
		}
	}
	d2, err := New(root, Options{Log: log.New(io.Discard, "", 0)})
	if err != nil {
		t.Fatalf("successor New after a failed load: %v", err)
	}
	go d2.Run() // Shutdown joins the load Run starts
	d2.Shutdown("test cleanup")
}
