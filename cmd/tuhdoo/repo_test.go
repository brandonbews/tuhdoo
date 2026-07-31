package main

// Principal derivation tests (D7, T4): the user.email local-part rule,
// the tuhdoo.principal override, and --as beating both. Unit tests
// call the derivation in-process against real hermetic repos; one
// integration test proves the mcp shim end-to-end.

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// hermeticGit isolates in-process git calls (gitOutput inherits the
// test process environment) from the developer's real global config,
// which could otherwise leak a user.email or tuhdoo.principal into
// derivation tests.
func hermeticGit(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
}

func TestPrincipalDerivation(t *testing.T) {
	hermeticGit(t)
	repo := newRepo(t)

	// No user.email, no override: derivation must fail, not invent.
	if _, err := gitEmailLocalPart(repo); err == nil {
		t.Fatal("derivation with no user.email and no override should error")
	}

	// Override absent: behavior unchanged — the local part of
	// user.email, ugly noreply addresses included.
	runGit(t, repo, "config", "user.email", "4099114+brandonbews@users.noreply.github.com")
	got, err := gitEmailLocalPart(repo)
	if err != nil {
		t.Fatalf("gitEmailLocalPart: %v", err)
	}
	if got != "4099114+brandonbews" {
		t.Fatalf("derived %q, want the email local part 4099114+brandonbews", got)
	}

	// Override set: it wins over the email rule.
	runGit(t, repo, "config", "tuhdoo.principal", "brandon")
	got, err = gitEmailLocalPart(repo)
	if err != nil {
		t.Fatalf("gitEmailLocalPart with override: %v", err)
	}
	if got != "brandon" {
		t.Fatalf("derived %q, want the tuhdoo.principal override brandon", got)
	}

	// Invalid overrides are loud errors naming the config key — never
	// a silent fall-through to the email rule.
	for _, bad := range []string{"brandon/agent-1", "bad actor", "a/b/c", ""} {
		runGit(t, repo, "config", "tuhdoo.principal", bad)
		_, err := gitEmailLocalPart(repo)
		if err == nil {
			t.Fatalf("tuhdoo.principal=%q should be rejected, got no error", bad)
		}
		if !strings.Contains(err.Error(), "tuhdoo.principal") {
			t.Errorf("error for %q should name the config key: %v", bad, err)
		}
	}

	// Unsetting restores the email rule.
	runGit(t, repo, "config", "--unset", "tuhdoo.principal")
	got, err = gitEmailLocalPart(repo)
	if err != nil {
		t.Fatalf("gitEmailLocalPart after unset: %v", err)
	}
	if got != "4099114+brandonbews" {
		t.Fatalf("derived %q after unset, want 4099114+brandonbews", got)
	}
}

// The TUI's steer-mode actor goes through the same derivation, so the
// override reaches bare `tuhdoo` too — and --as still beats it, with
// root-vs-agent validation intact.
func TestTopActorHonorsOverride(t *testing.T) {
	hermeticGit(t)
	repo := newRepo(t)
	t.Chdir(repo) // topActor derives from the current directory
	runGit(t, repo, "config", "user.email", "4099114+brandonbews@users.noreply.github.com")
	runGit(t, repo, "config", "tuhdoo.principal", "brandon")

	got, err := topActor("")
	if err != nil {
		t.Fatalf("topActor(\"\"): %v", err)
	}
	if got != "brandon" {
		t.Fatalf("steer-mode actor = %q, want the override brandon", got)
	}

	// --as beats the override.
	got, err = topActor("steve")
	if err != nil {
		t.Fatalf("topActor(steve): %v", err)
	}
	if got != "steve" {
		t.Fatalf("steer-mode actor = %q, want the explicit --as steve", got)
	}

	// Root-vs-agent rules still apply to an explicit --as...
	if _, err := topActor("steve/agent-1"); err == nil {
		t.Fatal("topActor should reject an agent principal for steer mode")
	}
	// ...and an agent-shaped override is rejected loudly at derivation.
	runGit(t, repo, "config", "tuhdoo.principal", "brandon/agent-1")
	if _, err := topActor(""); err == nil {
		t.Fatal("topActor should reject an agent-shaped tuhdoo.principal")
	}
}

// End-to-end through the shim: with tuhdoo.principal set, a no-flag
// `tuhdoo mcp` session acts as the override (agent half still minted
// by the daemon), and --as continues to beat everything.
func TestMCPShimHonorsPrincipalOverride(t *testing.T) {
	repo := newRepo(t)
	runGit(t, repo, "config", "user.email", "4099114+brandonbews@users.noreply.github.com")
	runGit(t, repo, "config", "tuhdoo.principal", "brandon")

	createdBy := func(t *testing.T, args []string, title string) string {
		t.Helper()
		cmd := exec.Command(binPath, args...)
		cmd.Dir = repo
		cmd.Env = cliEnv()
		client := mcp.NewClient(&mcp.Implementation{Name: "Claude Code", Version: "0"}, nil)
		cs, err := client.Connect(context.Background(), &mcp.CommandTransport{Command: cmd}, nil)
		if err != nil {
			t.Fatalf("connect through shim: %v", err)
		}
		defer cs.Close()
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
			Name:      "create_task",
			Arguments: map[string]any{"tasks": []map[string]any{{"title": title}}},
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
		return h.Task.CreatedBy
	}

	// No flags: the override is the human half, the daemon mints the
	// agent half from clientInfo.name.
	if by := createdBy(t, []string{"mcp"}, "born under the override"); by != "brandon/claude-code-1" {
		t.Fatalf("created_by = %q, want brandon/claude-code-1", by)
	}

	// --as beats the override entirely.
	if by := createdBy(t, []string{"mcp", "--as", "steve/agent-7"}, "born under --as"); by != "steve/agent-7" {
		t.Fatalf("created_by = %q, want steve/agent-7", by)
	}

	// An invalid override kills the shim at connect, loudly — a session
	// with no honest principal must not come up at all.
	runGit(t, repo, "config", "tuhdoo.principal", "not a principal")
	out, code := runCLI(t, repo, "mcp")
	if code == 0 {
		t.Fatalf("mcp with an invalid tuhdoo.principal exited 0; output:\n%s", out)
	}
	mustContain(t, out, "tuhdoo.principal")
}
