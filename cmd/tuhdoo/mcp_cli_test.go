package main

// Shim integration test: the real binary run as `tuhdoo mcp --as ...`
// over stdio, auto-spawning a real daemon, bridging to its /mcp
// endpoint. The SDK's CommandTransport is exactly how a harness would
// launch the shim.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mcpTools is the T5 surface the shim must mirror: exactly these ten.
var mcpTools = []string{
	"add_note", "claim_next", "claim_task", "create_task", "escalate",
	"finish_run", "get_backlog", "get_task", "release_claim", "update_task",
}

func TestMCPShimBridgesStdio(t *testing.T) {
	repo := newRepo(t)

	cmd := exec.Command(binPath, "mcp", "--as", "test/agent")
	cmd.Dir = repo
	cmd.Env = cliEnv()
	client := mcp.NewClient(&mcp.Implementation{Name: "harness", Version: "0"}, nil)
	cs, err := client.Connect(context.Background(), &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("connect through shim: %v", err)
	}
	defer cs.Close()

	// The mirrored surface is exactly the ten T5 verbs, and sessions
	// carry the daemon's orientation instructions.
	var names []string
	for tool, err := range cs.Tools(context.Background(), nil) {
		if err != nil {
			t.Fatalf("list tools: %v", err)
		}
		names = append(names, tool.Name)
	}
	if fmt.Sprint(names) != fmt.Sprint(mcpTools) {
		t.Fatalf("tool surface = %v, want %v", names, mcpTools)
	}
	if instr := cs.InitializeResult().Instructions; !strings.Contains(instr, "claim_next") {
		t.Errorf("instructions should describe the loop, got %q", instr)
	}

	// Round-trip a write and a read through the bridge.
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "create_task",
		Arguments: map[string]any{"tasks": []map[string]any{{"title": "born through the shim"}}},
	})
	if err != nil {
		t.Fatalf("create_task: %v", err)
	}
	if res.IsError {
		t.Fatalf("create_task returned a tool error: %+v", res.Content)
	}
	var created struct {
		IDs []string `json:"ids"`
	}
	decodeStructured(t, res, &created)
	if len(created.IDs) != 1 {
		t.Fatalf("create_task ids = %v, want one", created.IDs)
	}

	res, err = cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "get_task",
		Arguments: map[string]any{"task": created.IDs[0]},
	})
	if err != nil {
		t.Fatalf("get_task: %v", err)
	}
	if res.IsError {
		t.Fatalf("get_task returned a tool error: %+v", res.Content)
	}
	var h hydratedTask
	decodeStructured(t, res, &h)
	if h.Task.Title != "born through the shim" {
		t.Fatalf("get_task title = %q", h.Task.Title)
	}
	// The --as principal is the actor of record.
	if h.Task.CreatedBy != "test/agent" {
		t.Fatalf("created_by = %q, want the --as principal test/agent", h.Task.CreatedBy)
	}

	// Tool errors cross the bridge as tool errors, not protocol errors.
	res, err = cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "get_task",
		Arguments: map[string]any{"task": "t-nope"},
	})
	if err != nil {
		t.Fatalf("get_task(t-nope): protocol error %v, want a tool error", err)
	}
	if !res.IsError {
		t.Fatal("get_task(t-nope) should be a tool error")
	}
}

// The crash contract (mcp_cmd.go): if the daemon session dies while the
// harness is still attached, the shim exits 1 loudly — silently serving
// a dead session would let the harness think its leases still renew.
func TestMCPShimExitsWhenDaemonDies(t *testing.T) {
	repo := newRepo(t)

	// IOTransport over pipes we own, not CommandTransport: its Close
	// calls cmd.Wait itself, and the exit status is what this test is
	// about.
	cmd := exec.Command(binPath, "mcp", "--as", "test/agent")
	cmd.Dir = repo
	cmd.Env = cliEnv()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start shim: %v", err)
	}
	exited := false
	t.Cleanup(func() {
		if !exited { // don't leak the shim if an assertion fails first
			cmd.Process.Kill()
			cmd.Wait()
		}
	})

	client := mcp.NewClient(&mcp.Implementation{Name: "harness", Version: "0"}, nil)
	cs, err := client.Connect(context.Background(), &mcp.IOTransport{Reader: stdout, Writer: stdin}, nil)
	if err != nil {
		t.Fatalf("connect through shim: %v", err)
	}
	defer cs.Close()

	// The handshake auto-spawned a daemon; kill it out from under the
	// live session, with no chance to clean up.
	discPath := filepath.Join(repo, ".git", "tuhdoo", "daemon.json")
	pid, _, err := readDiscovery(discPath)
	if err != nil {
		t.Fatalf("read daemon.json: %v", err)
	}
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
		t.Fatalf("kill daemon %d: %v", pid, err)
	}

	// The shim's client retries the broken connection briefly before
	// declaring the session dead, so give it room.
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
		exited = true
	case <-time.After(30 * time.Second):
		t.Fatal("shim still running 30s after the daemon died; want exit 1")
	}
	if code := cmd.ProcessState.ExitCode(); code != 1 {
		t.Fatalf("shim exit code = %d, want 1; stderr:\n%s", code, stderr.String())
	}
	mustContain(t, stderr.String(), "daemon session ended")

	// SIGKILL left the discovery file behind; remove it so newRepo's
	// stopDaemon cleanup doesn't wait out its shutdown deadline.
	os.Remove(discPath)
}

func TestMCPShimRejectsBadPrincipal(t *testing.T) {
	repo := newRepo(t)
	out, code := runCLI(t, repo, "mcp", "--as", "a/b/c")
	if code == 0 {
		t.Fatalf("mcp --as a/b/c exited 0; output:\n%s", out)
	}
	mustContain(t, out, "invalid actor")

	// No --as means auto-derive — and this hermetic repo has no
	// user.email, so the shim must fail loudly at connect, not start a
	// session under a made-up name.
	out, code = runCLI(t, repo, "mcp")
	if code == 0 {
		t.Fatalf("mcp with no --as and no user.email exited 0; output:\n%s", out)
	}
	mustContain(t, out, "cannot derive a principal", "--as")

	out, code = runCLI(t, repo, "mcp", "--bogus")
	if code == 0 {
		t.Fatalf("mcp --bogus exited 0; output:\n%s", out)
	}
	mustContain(t, out, "usage")
}

// Zero-config identity (D7): with no --as, the human half derives from
// git user.email's local part and the daemon mints the agent half from
// the harness's clientInfo.name at session bind.
func TestMCPShimAutoDerivesPrincipal(t *testing.T) {
	repo := newRepo(t)
	runGit(t, repo, "config", "user.email", "brandon@example.com")

	cmd := exec.Command(binPath, "mcp")
	cmd.Dir = repo
	cmd.Env = cliEnv()
	client := mcp.NewClient(&mcp.Implementation{Name: "Claude Code", Version: "0"}, nil)
	cs, err := client.Connect(context.Background(), &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("connect through shim: %v", err)
	}
	defer cs.Close()

	// The bridge still carries the daemon's instructions and full
	// tool surface (they now flow through the initialize middleware).
	if instr := cs.InitializeResult().Instructions; !strings.Contains(instr, "claim_next") {
		t.Errorf("instructions should describe the loop, got %q", instr)
	}
	var names []string
	for tool, err := range cs.Tools(context.Background(), nil) {
		if err != nil {
			t.Fatalf("list tools: %v", err)
		}
		names = append(names, tool.Name)
	}
	if fmt.Sprint(names) != fmt.Sprint(mcpTools) {
		t.Fatalf("tool surface = %v, want %v", names, mcpTools)
	}

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "create_task",
		Arguments: map[string]any{"tasks": []map[string]any{{"title": "born without --as"}}},
	})
	if err != nil {
		t.Fatalf("create_task: %v", err)
	}
	if res.IsError {
		t.Fatalf("create_task returned a tool error: %+v", res.Content)
	}
	var created struct {
		IDs []string `json:"ids"`
	}
	decodeStructured(t, res, &created)
	res, err = cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "get_task",
		Arguments: map[string]any{"task": created.IDs[0]},
	})
	if err != nil {
		t.Fatalf("get_task: %v", err)
	}
	var h hydratedTask
	decodeStructured(t, res, &h)
	if h.Task.CreatedBy != "brandon/claude-code-1" {
		t.Fatalf("created_by = %q, want the auto-derived brandon/claude-code-1", h.Task.CreatedBy)
	}
}

func decodeStructured(t *testing.T, res *mcp.CallToolResult, dst any) {
	t.Helper()
	b, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	if err := json.Unmarshal(b, dst); err != nil {
		t.Fatalf("decode structured content %s: %v", b, err)
	}
}
