package main

// ensureDaemon's wait (T4, 2026-09-10) has two phases, each bounded by
// a generous ceiling with failures pointing at daemon.log: the socket
// accepting — the moment it does, however soon — and then the daemon
// reporting its ledger loaded. No fixed spawn wait: the daemon binds
// before it loads, so the load's length is measured by the daemon's
// own answer, not guessed at. Readiness is absorbed here, once, so no
// caller ever sees the "starting" placeholder.

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakeRepo is a runtime dir short enough for a unix socket (macOS caps
// sun_path at 104 bytes; t.TempDir() paths blow past it) with no git
// behind it — the waits only read daemon.json and dial.
func fakeRepo(t *testing.T) *repo {
	t.Helper()
	dir, err := os.MkdirTemp("", "tuhdoo")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	r := &repo{root: dir, gitDir: filepath.Join(dir, ".git")}
	if err := os.MkdirAll(r.runtimeDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	return r
}

// bindFakeDaemon does what a daemon does at startup, in order: bind the
// socket, then write daemon.json pointing at it. Nothing is served —
// a bound unix socket accepts dials on its own, which is all
// liveSocket checks.
func bindFakeDaemon(r *repo) (net.Listener, error) {
	sock := filepath.Join(r.runtimeDir(), "daemon.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		return nil, err
	}
	disc, _ := json.Marshal(map[string]any{"pid": os.Getpid(), "socket": sock})
	if err := os.WriteFile(r.discoveryPath(), disc, 0o644); err != nil {
		ln.Close()
		return nil, err
	}
	return ln, nil
}

// serveFakeDaemon is bindFakeDaemon plus an HTTP server answering
// /v0/state from state, so the loaded phase has something to poll.
// The returned stop tears the daemon down the way a failed first load
// does: socket closed, discovery file removed.
func serveFakeDaemon(t *testing.T, r *repo, state func() stateResp) (stop func()) {
	t.Helper()
	ln, err := bindFakeDaemon(r)
	if err != nil {
		t.Fatalf("bindFakeDaemon: %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v0/state", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(state())
	})
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln)
	stop = func() {
		srv.Close()
		os.Remove(r.discoveryPath())
	}
	t.Cleanup(stop)
	return stop
}

// placeholder is the answer a daemon serves until its first replay
// lands; loadedState what it serves after.
func placeholder() stateResp { return stateResp{Sync: syncJSON{Mode: "starting"}} }
func loadedState() stateResp {
	return stateResp{Loaded: true, Sync: syncJSON{Mode: "local-only"}}
}

// neverExits stands in for a daemon that keeps running.
func neverExits() <-chan int { return make(chan int) }

// exitedWith stands in for a spawned daemon that has already exited
// with code.
func exitedWith(code int) <-chan int {
	ch := make(chan int, 1)
	ch <- code
	return ch
}

func TestAwaitDaemonReturnsAsSoonAsSocketAccepts(t *testing.T) {
	r := fakeRepo(t)
	const comesUpAfter = 300 * time.Millisecond
	bound := make(chan net.Listener, 1)
	go func() {
		time.Sleep(comesUpAfter)
		ln, err := bindFakeDaemon(r)
		if err != nil {
			t.Error(err)
		}
		bound <- ln
	}()

	start := time.Now()
	c, err := awaitDaemon(r, neverExits(), daemonStartCeiling)
	elapsed := time.Since(start)
	if ln := <-bound; ln != nil {
		defer ln.Close()
	}
	if err != nil {
		t.Fatalf("awaitDaemon: %v", err)
	}
	if want := filepath.Join(r.runtimeDir(), "daemon.sock"); c.socket != want {
		t.Fatalf("client socket = %s, want %s", c.socket, want)
	}
	// Returned when the socket appeared — not after any fixed wait
	// (the old spawnWait was 5 s) and nowhere near the ceiling.
	if elapsed < comesUpAfter || elapsed > comesUpAfter+2*time.Second {
		t.Fatalf("awaitDaemon took %v; want just over %v (the socket's arrival), never a fixed wait", elapsed, comesUpAfter)
	}
}

func TestAwaitDaemonCeilingNamesDaemonLog(t *testing.T) {
	if daemonStartCeiling != 30*time.Second {
		t.Fatalf("daemonStartCeiling = %v, want the generous 30s the design names", daemonStartCeiling)
	}
	r := fakeRepo(t)
	const ceiling = 200 * time.Millisecond
	start := time.Now()
	_, err := awaitDaemon(r, neverExits(), ceiling)
	if err == nil {
		t.Fatal("awaitDaemon with no socket ever succeeded")
	}
	if time.Since(start) < ceiling {
		t.Fatalf("gave up before the ceiling: %v < %v", time.Since(start), ceiling)
	}
	logPath := r.logPath()
	if !strings.Contains(err.Error(), logPath) || !strings.Contains(err.Error(), ceiling.String()) {
		t.Fatalf("ceiling error %q should name the ceiling and %s", err, logPath)
	}
}

// A spawned daemon that dies before binding fails the wait at once,
// pointing at daemon.log — no grace period, nowhere near the ceiling.
func TestAwaitDaemonFailsFastWhenTheDaemonExits(t *testing.T) {
	r := fakeRepo(t)
	start := time.Now()
	_, err := awaitDaemon(r, exitedWith(1), daemonStartCeiling)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("awaitDaemon succeeded although the daemon exited and no socket exists")
	}
	if elapsed > time.Second {
		t.Fatalf("took %v to notice the exit; want at once", elapsed)
	}
	if !strings.Contains(err.Error(), "exited") || !strings.Contains(err.Error(), r.logPath()) {
		t.Fatalf("exit error %q should say the daemon exited and name daemon.log", err)
	}
}

// The flock loser of two CLIs racing to spawn exits with
// exitAlreadyRunning; its CLI keeps looking for the winner's socket,
// which arrives a moment later — a death report here would be wrong.
// With no winner ever binding, the loser's exit still ends at the
// ceiling, not as a death.
func TestAwaitDaemonKeepsWaitingAfterTheFlockLoserExits(t *testing.T) {
	r := fakeRepo(t)
	const winnerBindsAfter = 300 * time.Millisecond
	bound := make(chan net.Listener, 1)
	go func() {
		time.Sleep(winnerBindsAfter)
		ln, err := bindFakeDaemon(r)
		if err != nil {
			t.Error(err)
		}
		bound <- ln
	}()

	start := time.Now()
	c, err := awaitDaemon(r, exitedWith(exitAlreadyRunning), daemonStartCeiling)
	elapsed := time.Since(start)
	if ln := <-bound; ln != nil {
		defer ln.Close()
	}
	if err != nil {
		t.Fatalf("awaitDaemon reported the flock loser's exit as a failure: %v", err)
	}
	if c == nil || elapsed < winnerBindsAfter {
		t.Fatalf("awaitDaemon returned %v after %v; want the winner's socket after %v", c, elapsed, winnerBindsAfter)
	}

	// No winner: the ceiling, not a death.
	r2 := fakeRepo(t)
	const ceiling = 200 * time.Millisecond
	_, err = awaitDaemon(r2, exitedWith(exitAlreadyRunning), ceiling)
	if err == nil || strings.Contains(err.Error(), "exited") || !strings.Contains(err.Error(), ceiling.String()) {
		t.Fatalf("flock-loser exit with no winner: err %v; want the ceiling error", err)
	}
}

// The loaded phase returns the moment the daemon reports loaded, and a
// placeholder answer is polled through, never returned.
func TestAwaitLoadedWaitsForTheDaemonsLoad(t *testing.T) {
	r := fakeRepo(t)
	var polls atomic.Int32
	const loadsAfterPolls = 4
	serveFakeDaemon(t, r, func() stateResp {
		if polls.Add(1) < loadsAfterPolls {
			return placeholder()
		}
		return loadedState()
	})

	c, err := ensureDaemon(r) // the live-socket path: no spawn, straight to the loaded phase
	if err != nil {
		t.Fatalf("ensureDaemon: %v", err)
	}
	if c == nil {
		t.Fatal("ensureDaemon returned no client")
	}
	if got := polls.Load(); got != loadsAfterPolls {
		t.Fatalf("ensureDaemon returned after %d state polls; want exactly %d (the first loaded answer)", got, loadsAfterPolls)
	}
	// What the caller reads next is real state, never the placeholder.
	st, err := fetchState(c)
	if err != nil {
		t.Fatalf("fetchState after ensureDaemon: %v", err)
	}
	if !st.Loaded {
		t.Fatal("fetchState handed back the placeholder")
	}
}

func TestAwaitLoadedCeilingNamesDaemonLog(t *testing.T) {
	r := fakeRepo(t)
	serveFakeDaemon(t, r, placeholder)
	sock, ok := liveSocket(r)
	if !ok {
		t.Fatal("fake daemon's socket does not accept")
	}
	const ceiling = 200 * time.Millisecond
	start := time.Now()
	_, err := awaitLoaded(r, newClient(sock), ceiling)
	if err == nil {
		t.Fatal("awaitLoaded succeeded against a daemon that never loads")
	}
	if time.Since(start) < ceiling {
		t.Fatalf("gave up before the ceiling: %v < %v", time.Since(start), ceiling)
	}
	logPath := r.logPath()
	if !strings.Contains(err.Error(), "loading") || !strings.Contains(err.Error(), logPath) {
		t.Fatalf("ceiling error %q should say the daemon is still loading and name %s", err, logPath)
	}
}

// A daemon whose first load fails tears its socket and discovery file
// down; the loaded phase reports that death rather than handing back a
// client to nothing.
func TestAwaitLoadedReportsADaemonThatDiedLoading(t *testing.T) {
	r := fakeRepo(t)
	var polls atomic.Int32
	stop := serveFakeDaemon(t, r, func() stateResp {
		polls.Add(1)
		return placeholder()
	})
	sock, ok := liveSocket(r)
	if !ok {
		t.Fatal("fake daemon's socket does not accept")
	}
	go func() {
		for polls.Load() < 2 {
			time.Sleep(10 * time.Millisecond)
		}
		stop() // dies mid-load, after answering the placeholder
	}()

	start := time.Now()
	c, err := awaitLoaded(r, newClient(sock), daemonStartCeiling)
	if err == nil || c != nil {
		t.Fatalf("awaitLoaded = client %v, err %v; want an error and no client", c, err)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatalf("took %v to notice the death; want at once", time.Since(start))
	}
	if !strings.Contains(err.Error(), "exited while starting") || !strings.Contains(err.Error(), r.logPath()) {
		t.Fatalf("death error %q should say the daemon exited while starting and name daemon.log", err)
	}
}

// fetchState never hands a placeholder to a caller: at its budget a
// daemon still loading is an error (the TUI renders it as retrying and
// keeps its board), while a loaded daemon whose sync loop has not yet
// decided its mode is returned as is.
func TestFetchStateNeverReturnsThePlaceholder(t *testing.T) {
	r := fakeRepo(t)
	serveFakeDaemon(t, r, placeholder)
	sock, ok := liveSocket(r)
	if !ok {
		t.Fatal("fake daemon's socket does not accept")
	}
	_, err := fetchState(newClient(sock))
	if err == nil || !strings.Contains(err.Error(), "loading") {
		t.Fatalf("fetchState against a placeholder: err %v; want an error saying the daemon is still loading", err)
	}

	r2 := fakeRepo(t)
	serveFakeDaemon(t, r2, func() stateResp {
		return stateResp{Loaded: true, Sync: syncJSON{Mode: "starting"}}
	})
	sock2, ok := liveSocket(r2)
	if !ok {
		t.Fatal("fake daemon's socket does not accept")
	}
	st, err := fetchState(newClient(sock2))
	if err != nil {
		t.Fatalf("fetchState against a loaded daemon with an undecided sync mode: %v", err)
	}
	if !st.Loaded || st.Sync.Mode != "starting" {
		t.Fatalf("fetchState = loaded %v, mode %q; want the state as served (true, starting)", st.Loaded, st.Sync.Mode)
	}
}
