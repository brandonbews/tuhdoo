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

// daemonStartCeiling bounds each phase of ensureDaemon's wait: first
// for a freshly spawned daemon's socket to accept connections, then for
// the daemon to report its ledger loaded. The daemon binds its socket
// before it loads the ledger (T4 startup order, 2026-09-10), so the
// first phase is a ceiling on process start — generous, because it is
// never the normal case — and the second on the load itself (a
// measured 7.2 s cold start; the ceiling leaves room for a bigger
// ledger and a slower disk). Its predecessor, a fixed 5 s spawn wait,
// covered both phases together, and the cold load alone could never
// beat it: every launch after a daemon death failed with "daemon did
// not come up within 5s".
const daemonStartCeiling = 30 * time.Second

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
// when none is serving. It returns only once the daemon reports its
// ledger loaded: the daemon serves before it loads (T4 startup order,
// 2026-09-10) and answers a placeholder until the first replay lands,
// and readiness is absorbed here, once, rather than by every caller —
// whichever way the socket was found, the loaded phase follows.
func ensureDaemon(r *repo) (*client, error) {
	var c *client
	if sock, ok := liveSocket(r); ok {
		c = newClient(sock)
	} else {
		exited, err := spawnDaemon(r)
		if err != nil {
			return nil, err
		}
		c, err = awaitDaemon(r, exited, daemonStartCeiling)
		if err != nil {
			return nil, err
		}
	}
	return awaitLoaded(r, c, daemonStartCeiling)
}

// awaitDaemon polls for the daemon's socket until it accepts — that is
// the return, however soon it comes — or until ceiling elapses. exited
// delivers the spawned process's exit status when it is gone. An exit
// with exitAlreadyRunning is the flock loser of two CLIs racing to
// spawn: the winner's socket is coming, so the poll goes on. Any other
// exit is a daemon that died before binding (git too old, a repository
// it cannot read) and fails at once rather than at the ceiling. Either
// failure names daemon.log, where the daemon wrote its reason.
func awaitDaemon(r *repo, exited <-chan int, ceiling time.Duration) (*client, error) {
	logPath := filepath.Join(r.runtimeDir(), "daemon.log")
	deadline := time.Now().Add(ceiling)
	for {
		if sock, ok := liveSocket(r); ok {
			return newClient(sock), nil
		}
		select {
		case code := <-exited:
			if code != exitAlreadyRunning {
				return nil, fmt.Errorf("daemon exited without serving; see %s", logPath)
			}
		default:
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("daemon did not come up within %v; see %s", ceiling, logPath)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// awaitLoaded polls /v0/state on an accepting socket until the daemon
// reports its ledger loaded, bounded by ceiling. A daemon whose first
// load fails ends itself — socket and discovery file torn down — so a
// poll that can no longer reach the socket is that death, reported as
// such rather than as a dead client handed back; any other failed poll
// is the daemon's own answer, surfaced as is. Both failures and the
// ceiling name daemon.log.
func awaitLoaded(r *repo, c *client, ceiling time.Duration) (*client, error) {
	logPath := filepath.Join(r.runtimeDir(), "daemon.log")
	deadline := time.Now().Add(ceiling)
	for {
		var st stateResp
		if err := c.get("/v0/state", &st); err != nil {
			if _, ok := liveSocket(r); !ok {
				return nil, fmt.Errorf("daemon exited while starting; see %s", logPath)
			}
			return nil, fmt.Errorf("daemon not answering: %w", err)
		}
		if st.Loaded {
			return c, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("daemon is still loading the ledger after %v; see %s", ceiling, logPath)
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
// loser exit quietly (with exitAlreadyRunning) and both CLIs find the
// winner's socket. The returned channel delivers the daemon's exit
// status if it exits while this CLI is alive — in the normal case it
// never does.
func spawnDaemon(r *repo) (<-chan int, error) {
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
	exited := make(chan int, 1)
	go func() {
		_ = cmd.Wait() // reaps the child if it dies while this CLI is alive
		code := -1     // killed by a signal, or the wait itself failed
		if cmd.ProcessState != nil {
			code = cmd.ProcessState.ExitCode()
		}
		exited <- code
	}()
	return exited, nil
}
