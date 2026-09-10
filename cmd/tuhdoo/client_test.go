package main

// ensureDaemon's wait (T4, 2026-09-10): a client is returned the moment
// the daemon's socket accepts — however soon that is — bounded by a
// generous ceiling on the socket appearing, with failures pointing at
// daemon.log. No fixed spawn wait: the daemon binds before it loads,
// so the load's length is not the client's problem.

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeRepo is a runtime dir short enough for a unix socket (macOS caps
// sun_path at 104 bytes; t.TempDir() paths blow past it) with no git
// behind it — awaitDaemon only reads daemon.json and dials.
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
	if err := os.WriteFile(filepath.Join(r.runtimeDir(), "daemon.json"), disc, 0o644); err != nil {
		ln.Close()
		return nil, err
	}
	return ln, nil
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
	c, err := awaitDaemon(r, make(chan struct{}), daemonStartCeiling)
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
	_, err := awaitDaemon(r, make(chan struct{}), ceiling)
	if err == nil {
		t.Fatal("awaitDaemon with no socket ever succeeded")
	}
	if time.Since(start) < ceiling {
		t.Fatalf("gave up before the ceiling: %v < %v", time.Since(start), ceiling)
	}
	logPath := filepath.Join(r.runtimeDir(), "daemon.log")
	if !strings.Contains(err.Error(), logPath) || !strings.Contains(err.Error(), ceiling.String()) {
		t.Fatalf("ceiling error %q should name the ceiling and %s", err, logPath)
	}
}

// A spawned daemon that dies before binding fails the wait after a
// short grace, pointing at daemon.log — not after the 30 s ceiling.
func TestAwaitDaemonFailsFastWhenTheDaemonExits(t *testing.T) {
	r := fakeRepo(t)
	exited := make(chan struct{})
	close(exited)
	start := time.Now()
	_, err := awaitDaemon(r, exited, daemonStartCeiling)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("awaitDaemon succeeded although the daemon exited and no socket exists")
	}
	if elapsed > spawnExitGrace+2*time.Second {
		t.Fatalf("took %v to notice the exit; want about %v", elapsed, spawnExitGrace)
	}
	if !strings.Contains(err.Error(), "exited") || !strings.Contains(err.Error(), filepath.Join(r.runtimeDir(), "daemon.log")) {
		t.Fatalf("exit error %q should say the daemon exited and name daemon.log", err)
	}
}
