package main

// Shim integration test: the real binary run as `tuhdoo mcp --as ...`
// over stdio, auto-spawning a real daemon, bridging to its /mcp
// endpoint. The SDK's CommandTransport is exactly how a harness would
// launch the shim.

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"testing"

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

func TestMCPShimRejectsBadPrincipal(t *testing.T) {
	repo := newRepo(t)
	out, code := runCLI(t, repo, "mcp", "--as", "a/b/c")
	if code == 0 {
		t.Fatalf("mcp --as a/b/c exited 0; output:\n%s", out)
	}
	mustContain(t, out, "invalid actor")

	out, code = runCLI(t, repo, "mcp")
	if code == 0 {
		t.Fatalf("mcp with no --as exited 0; output:\n%s", out)
	}
	mustContain(t, out, "usage")
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
