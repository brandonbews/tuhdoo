package main

// Daemon connection: discovery via <git-dir>/tuhdoo/daemon.json,
// auto-spawn when absent (T4 lazy lifecycle), and an HTTP client that
// speaks over the unix socket.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// daemonStartCeiling bounds how long ensureDaemon waits for a freshly
// spawned daemon's socket to accept connections. The daemon binds its
// socket before it loads the ledger (T4 startup order, 2026-09-10), so
// this is a ceiling on process start — generous, because it is never
// the normal case — not on load time. Its predecessor, a fixed 5 s
// spawn wait, covered the load too, and a measured 7.2 s cold load
// could never beat it: every launch after a daemon death failed with
// "daemon did not come up within 5s".
const daemonStartCeiling = 30 * time.Second

// spawnExitGrace is how long ensureDaemon keeps looking for a socket
// after the daemon it spawned has already exited. Two CLIs racing to
// spawn each start a daemon; the flock loser exits at once, and the
// winner's socket appears a few milliseconds later — the loser's CLI
// must find it rather than report a death.
const spawnExitGrace = 2 * time.Second

// client speaks the daemon's JSON HTTP API over its unix socket.
type client struct {
	hc     *http.Client
	socket string
}

func newClient(socket string) *client {
	return &client{
		socket: socket,
		hc:     &http.Client{Transport: unixTransport(socket)},
	}
}

// unixTransport dials the daemon's socket for every request; the URL
// host is a placeholder. Shared by the JSON API client and the mcp
// shim's streamable client.
func unixTransport(socket string) *http.Transport {
	return &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", socket)
		},
	}
}

// get performs one read request and decodes the JSON response into dst.
// Non-200 responses surface the daemon's error message.
func (c *client) get(path string, dst any) error {
	resp, err := c.hc.Get("http://tuhdoo" + path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := readAPIResponse(resp, "GET", path)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, dst)
}

// readAPIResponse reads a capped response body. Non-200 responses
// surface the daemon's decoded {"error": ...} message when present,
// else "METHOD path: status N".
func readAPIResponse(resp *http.Response, method, path string) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<24))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(body, &e) == nil && e.Error != "" {
			return nil, errors.New(e.Error)
		}
		return nil, fmt.Errorf("%s %s: status %d", method, path, resp.StatusCode)
	}
	return body, nil
}

// write performs one write request stamped with the acting principal
// (X-Tuhdoo-Actor, D7). Non-200 responses surface the daemon's error
// message.
func (c *client) write(method, path, actor string, body any) error {
	return c.writeResp(method, path, actor, body, nil)
}

// writeResp is write, additionally decoding the 200 response body into
// dst when dst is non-nil.
func (c *client) writeResp(method, path, actor string, body, dst any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(method, "http://tuhdoo"+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tuhdoo-Actor", actor)
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, err := readAPIResponse(resp, method, path)
	if err != nil {
		return err
	}
	if dst == nil {
		return nil
	}
	return json.Unmarshal(respBody, dst)
}

// ensureDaemon returns a client for the repo's daemon, spawning one
// when none is serving. A client is returned as soon as the socket
// accepts; the daemon may still be loading the ledger behind it, and
// answers "starting" until it is done (fetchState waits that out).
func ensureDaemon(r *repo) (*client, error) {
	if sock, ok := liveSocket(r); ok {
		return newClient(sock), nil
	}
	exited, err := spawnDaemon(r)
	if err != nil {
		return nil, err
	}
	return awaitDaemon(r, exited, daemonStartCeiling)
}

// awaitDaemon polls for the daemon's socket until it accepts — that is
// the return, however soon it comes — or until ceiling elapses. exited
// closes when the spawned process is gone: a daemon that died before
// binding (git too old, a repository it cannot read) fails fast after
// spawnExitGrace instead of at the ceiling. Either failure names
// daemon.log, where the daemon wrote its reason.
func awaitDaemon(r *repo, exited <-chan struct{}, ceiling time.Duration) (*client, error) {
	logPath := filepath.Join(r.runtimeDir(), "daemon.log")
	deadline := time.Now().Add(ceiling)
	var exitDeadline time.Time
	for {
		if sock, ok := liveSocket(r); ok {
			return newClient(sock), nil
		}
		if exitDeadline.IsZero() {
			select {
			case <-exited:
				exitDeadline = time.Now().Add(spawnExitGrace)
			default:
			}
		}
		now := time.Now()
		if !exitDeadline.IsZero() && now.After(exitDeadline) {
			return nil, fmt.Errorf("daemon exited without serving; see %s", logPath)
		}
		if now.After(deadline) {
			return nil, fmt.Errorf("daemon did not come up within %v; see %s", ceiling, logPath)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// liveSocket reads daemon.json and proves the daemon is actually
// serving by dialing its socket — a stale file from a crash fails the
// dial and we spawn fresh.
func liveSocket(r *repo) (string, bool) {
	b, err := os.ReadFile(filepath.Join(r.runtimeDir(), "daemon.json"))
	if err != nil {
		return "", false
	}
	var disc struct {
		Socket string `json:"socket"`
	}
	if json.Unmarshal(b, &disc) != nil || disc.Socket == "" {
		return "", false
	}
	conn, err := net.DialTimeout("unix", disc.Socket, time.Second)
	if err != nil {
		return "", false
	}
	conn.Close()
	return disc.Socket, true
}

// spawnDaemon re-execs this binary as `tuhdoo daemon`, detached in its
// own session with output going to daemon.log, so it outlives the CLI
// and its terminal. If two CLIs race here, the daemon's flock makes the
// loser exit quietly and both CLIs find the winner's socket. The
// returned channel closes when the daemon process exits, which in the
// normal case is long after this CLI is gone.
func spawnDaemon(r *repo) (<-chan struct{}, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("locate own binary: %w", err)
	}
	if err := os.MkdirAll(r.runtimeDir(), 0o755); err != nil {
		return nil, fmt.Errorf("create runtime dir: %w", err)
	}
	logf, err := os.OpenFile(filepath.Join(r.runtimeDir(), "daemon.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open daemon.log: %w", err)
	}
	defer logf.Close()

	cmd := exec.Command(exe, "daemon")
	cmd.Dir = r.root
	cmd.Stdout = logf
	cmd.Stderr = logf
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("spawn daemon: %w", err)
	}
	exited := make(chan struct{})
	go func() {
		_ = cmd.Wait() // reaps the child if it dies while this CLI is alive
		close(exited)
	}()
	return exited, nil
}
