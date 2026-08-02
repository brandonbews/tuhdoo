package main

// CLI integration tests for the write verbs (create / update / answer):
// the paved path when no MCP session exists. Same harness as
// cli_test.go — real binary, real repo, real auto-spawned daemon.

import (
	"os/exec"
	"strings"
	"testing"
)

// runCLIStdin is runCLI with the given stdin.
func runCLIStdin(t *testing.T, repo, stdin string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	cmd.Dir = repo
	cmd.Env = cliEnv()
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return string(out), ee.ExitCode()
		}
		t.Fatalf("run %v: %v\n%s", args, err, out)
	}
	return string(out), 0
}

// createdID extracts the task ID from `tuhdoo create` output.
func createdID(t *testing.T, out string) string {
	t.Helper()
	fields := strings.Fields(out)
	if len(fields) < 2 || fields[0] != "created" || !strings.HasPrefix(fields[1], "tuh-") {
		t.Fatalf("cannot parse create output:\n%s", out)
	}
	return fields[1]
}

func TestCreateUpdateAnswer(t *testing.T) {
	repo := newRepo(t)
	// Derivation path: no --as, so the principal comes from user.email.
	runGit(t, repo, "config", "user.email", "brandon@example.com")
	if out, code := runCLI(t, repo, "init"); code != 0 {
		t.Fatalf("init exit %d; output:\n%s", code, out)
	}

	// create with all the field flags; actor derived from git identity.
	out, code := runCLI(t, repo, "create", "write the parser",
		"--desc", "Parse the thing.\nAcceptance: it parses.",
		"--priority", "5", "--labels", "go,parser")
	if code != 0 {
		t.Fatalf("create exit %d; output:\n%s", code, out)
	}
	parser := createdID(t, out)
	mustContain(t, out, "write the parser")

	out, code = runCLI(t, repo, "task", parser)
	if code != 0 {
		t.Fatalf("task exit %d; output:\n%s", code, out)
	}
	mustContain(t, out, "write the parser", "Parse the thing.",
		"Acceptance: it parses.", "priority    5", "go, parser",
		"created", "by brandon")

	// create with --desc - (stdin) and an edge given as a short ID.
	out, code = runCLIStdin(t, repo, "Docs need the parser first.\n",
		"create", "ship the docs", "--desc", "-", "--depends-on", shortID(parser))
	if code != 0 {
		t.Fatalf("create --desc - exit %d; output:\n%s", code, out)
	}
	docs := createdID(t, out)
	out, code = runCLI(t, repo, "task", docs)
	if code != 0 {
		t.Fatalf("task exit %d; output:\n%s", code, out)
	}
	mustContain(t, out, "Docs need the parser first.", "depends on  "+parser)

	// update: partial by design — only sent fields change; --as wins
	// over git identity and lands as the recorded actor.
	out, code = runCLI(t, repo, "update", shortID(parser),
		"--priority", "2", "--labels", "go", "--as", "steer")
	if code != 0 {
		t.Fatalf("update exit %d; output:\n%s", code, out)
	}
	mustContain(t, out, "updated "+parser)
	out, code = runCLI(t, repo, "task", parser)
	if code != 0 {
		t.Fatalf("task exit %d; output:\n%s", code, out)
	}
	mustContain(t, out, "priority    2", "Parse the thing.")
	if strings.Contains(out, "parser,") || strings.Contains(out, ", parser") {
		t.Errorf("labels not fully replaced:\n%s", out)
	}

	// update --status is the curation path (done/cancelled). One
	// vocabulary since the status-vocabulary revision (2026-08-01):
	// the flag word is the stored word, and the "archived" input alias
	// is gone.
	out, code = runCLI(t, repo, "update", docs, "--status", "cancelled")
	if code != 0 {
		t.Fatalf("update --status exit %d; output:\n%s", code, out)
	}
	out, _ = runCLI(t, repo, "status")
	mustContain(t, out, "1 cancelled")
	out, _ = runCLI(t, repo, "task", docs)
	mustContain(t, out, "status      cancelled")
	// The machine surface agrees: the API JSON says cancelled.
	hc := apiClient(t, repo)
	if got := string(api(t, hc, "GET", "/v0/tasks/"+docs, "", nil)); !strings.Contains(got, `"status":"cancelled"`) {
		t.Errorf("API status vocabulary changed; body:\n%s", got)
	}

	// update with no field flags is an error, not a silent no-op.
	out, code = runCLI(t, repo, "update", parser)
	if code == 0 {
		t.Fatalf("field-less update exited 0; output:\n%s", out)
	}
	mustContain(t, out, "usage: tuhdoo update")

	// The daemon's validation reaches the user: bad status is loud.
	out, code = runCLI(t, repo, "update", parser, "--status", "bogus")
	if code == 0 {
		t.Fatalf("bad status exited 0; output:\n%s", out)
	}

	// answer, addressed by the blocked task's ID (what `tuhdoo
	// escalations` prints); the answer is the rest of the line.
	api(t, hc, "POST", "/v0/escalations", "brandon/a1", map[string]any{
		"task": parser, "question": "Which encoding?", "blocking": true,
	})
	out, code = runCLI(t, repo, "answer", shortID(parser), "UTF-8", "only.")
	if code != 0 {
		t.Fatalf("answer exit %d; output:\n%s", code, out)
	}
	mustContain(t, out, "answered", "Which encoding?")
	out, code = runCLI(t, repo, "escalations")
	if code != 0 {
		t.Fatalf("escalations exit %d; output:\n%s", code, out)
	}
	answeredRow := ""
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "answered") {
			answeredRow = line
		}
	}
	if !strings.Contains(answeredRow, "brandon") || !strings.Contains(answeredRow, "UTF-8 only.") {
		t.Errorf("answered row missing attribution or answer:\n%s", out)
	}

	// Two open questions on one task: addressing by task is ambiguous,
	// the error lists candidates, and the escalation's own ID still
	// resolves.
	api(t, hc, "POST", "/v0/escalations", "brandon/a1", map[string]any{
		"task": parser, "question": "Streaming or buffered?",
	})
	api(t, hc, "POST", "/v0/escalations", "brandon/a1", map[string]any{
		"task": parser, "question": "Error budget?",
	})
	out, code = runCLI(t, repo, "answer", shortID(parser), "irrelevant")
	if code == 0 {
		t.Fatalf("ambiguous answer exited 0; output:\n%s", out)
	}
	mustContain(t, out, "ambiguous", "Streaming or buffered?", "Error budget?")
	escID := ""
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "Streaming or buffered?") {
			escID = strings.Fields(line)[0]
		}
	}
	if escID == "" {
		t.Fatalf("no escalation ID in ambiguity listing:\n%s", out)
	}
	out, code = runCLI(t, repo, "answer", escID, "Buffered.")
	if code != 0 {
		t.Fatalf("answer by escalation ID exit %d; output:\n%s", code, out)
	}
	mustContain(t, out, "Streaming or buffered?")

	// No open escalation matches: loud, with a pointer.
	out, code = runCLI(t, repo, "answer", "zzzz", "nothing")
	if code == 0 {
		t.Fatalf("unmatched answer exited 0; output:\n%s", out)
	}
	mustContain(t, out, "no open escalation")
}

// Capture and the shelves over the CLI (2026-07-31): title-only
// create --status inbox, park with --status held, promote/pause/resume
// with update --status — the same round trips the MCP surface has.
func TestCreateCaptureAndPromote(t *testing.T) {
	repo := newRepo(t)
	runGit(t, repo, "config", "user.email", "brandon@example.com")
	if out, code := runCLI(t, repo, "init"); code != 0 {
		t.Fatalf("init exit %d; output:\n%s", code, out)
	}

	// Title-only capture: no description, no priority — legitimate for
	// inbox, and the daemon accepts it as-is.
	out, code := runCLI(t, repo, "create", "idea: dark mode", "--status", "inbox")
	if code != 0 {
		t.Fatalf("capture exit %d; output:\n%s", code, out)
	}
	idea := createdID(t, out)
	out, code = runCLI(t, repo, "create", "polish docs", "--status", "held", "--priority", "3")
	if code != 0 {
		t.Fatalf("held create exit %d; output:\n%s", code, out)
	}
	parked := createdID(t, out)

	// Born-terminal or garbage statuses are loud.
	out, code = runCLI(t, repo, "create", "stillborn", "--status", "done")
	if code == 0 {
		t.Fatalf("create --status done exited 0; output:\n%s", out)
	}
	mustContain(t, out, "invalid status")

	// The shelves render: backlog rows carry the on-hold / inbox STATE
	// values (serialized form, T7 2026-07-31; on-hold above inbox),
	// status counts them, and the task biography keeps its status line.
	out, _ = runCLI(t, repo, "backlog")
	mustContain(t, out, "on-hold", parked, "inbox", idea, "idea: dark mode")
	if strings.Index(out, parked) > strings.Index(out, idea) {
		t.Errorf("the on-hold row must render above the inbox row:\n%s", out)
	}
	out, _ = runCLI(t, repo, "status")
	mustContain(t, out, "0 ready", "1 on hold", "1 inbox")
	out, _ = runCLI(t, repo, "task", idea)
	mustContain(t, out, "status      inbox")

	// Promote the capture (with the description promotion deserves);
	// pause and resume the other. All plain update --status.
	out, code = runCLI(t, repo, "update", idea, "--status", "open",
		"--desc", "Context, ask, acceptance.")
	if code != 0 {
		t.Fatalf("promotion exit %d; output:\n%s", code, out)
	}
	out, code = runCLI(t, repo, "update", parked, "--status", "open")
	if code != 0 {
		t.Fatalf("resume exit %d; output:\n%s", code, out)
	}
	out, code = runCLI(t, repo, "update", parked, "--status", "held")
	if code != 0 {
		t.Fatalf("pause exit %d; output:\n%s", code, out)
	}
	out, _ = runCLI(t, repo, "status")
	mustContain(t, out, "1 ready", "1 on hold", "0 inbox")
	out, _ = runCLI(t, repo, "task", idea)
	mustContain(t, out, "status      open", "Context, ask, acceptance.")
}

// The write commands are the human portal: they act as a root human
// principal, never an agent, and tuhdoo.principal overrides the email
// derivation exactly as in the TUI.
func TestWriteActorRules(t *testing.T) {
	repo := newRepo(t)
	runGit(t, repo, "config", "user.email", "brandon@example.com")
	runGit(t, repo, "config", "tuhdoo.principal", "steerer")
	if out, code := runCLI(t, repo, "init"); code != 0 {
		t.Fatalf("init exit %d; output:\n%s", code, out)
	}

	out, code := runCLI(t, repo, "create", "override check")
	if code != 0 {
		t.Fatalf("create exit %d; output:\n%s", code, out)
	}
	out, code = runCLI(t, repo, "task", createdID(t, out))
	if code != 0 {
		t.Fatalf("task exit %d; output:\n%s", code, out)
	}
	mustContain(t, out, "by steerer")

	out, code = runCLI(t, repo, "create", "agent check", "--as", "brandon/a1")
	if code == 0 {
		t.Fatalf("agent principal accepted; output:\n%s", out)
	}
	mustContain(t, out, "human root principal")
}

// The claim lifecycle is deliberately MCP-only; the help text owns that
// decision instead of leaving it implied.
func TestHelpDocumentsWriteVerbs(t *testing.T) {
	repo := newRepo(t)
	out, code := runCLI(t, repo, "help")
	if code != 0 {
		t.Fatalf("help exit %d; output:\n%s", code, out)
	}
	mustContain(t, out, "create <t>", "update <id>", "answer <id>",
		"deliberately not here", "tuhdoo mcp")
}
