package main

// Daemon connection: discovery via <git-dir>/tuhdoo/daemon.json,
// auto-spawn when absent (T4 lazy lifecycle), and an HTTP client that
// speaks over the unix socket.

import (
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

// spawnWait bounds how long we wait for a freshly spawned daemon to
// come up.
const spawnWait = 5 * time.Second

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
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<24))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(body, &e) == nil && e.Error != "" {
			return errors.New(e.Error)
		}
		return fmt.Errorf("GET %s: status %d", path, resp.StatusCode)
	}
	return json.Unmarshal(body, dst)
}

// ensureDaemon returns a client for the repo's daemon, spawning one
// when none is serving.
func ensureDaemon(r *repo) (*client, error) {
	if sock, ok := liveSocket(r); ok {
		return newClient(sock), nil
	}
	if err := spawnDaemon(r); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(spawnWait)
	for time.Now().Before(deadline) {
		if sock, ok := liveSocket(r); ok {
			return newClient(sock), nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil, fmt.Errorf("daemon did not come up within %v; see %s",
		spawnWait, filepath.Join(r.runtimeDir(), "daemon.log"))
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
// loser exit quietly and both CLIs find the winner's socket.
func spawnDaemon(r *repo) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate own binary: %w", err)
	}
	if err := os.MkdirAll(r.runtimeDir(), 0o755); err != nil {
		return fmt.Errorf("create runtime dir: %w", err)
	}
	logf, err := os.OpenFile(filepath.Join(r.runtimeDir(), "daemon.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open daemon.log: %w", err)
	}
	defer logf.Close()

	cmd := exec.Command(exe, "daemon")
	cmd.Dir = r.root
	cmd.Stdout = logf
	cmd.Stderr = logf
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("spawn daemon: %w", err)
	}
	return cmd.Process.Release()
}
