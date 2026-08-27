package main

// Every subcommand answers -h/--help the same way: that command's usage
// on stdout, exit 0 (adopter report, 2026-08-26 — `tuhdoo init --help`
// used to be rejected as an unexpected argument). The help text renders
// from the same per-command table as the global usage(), so the two
// cannot drift; this file pins the behavior across the full dispatch
// list, pins the no-drift property against the live global output, and
// pins that help is a pure print: no repo, no daemon, nothing written.

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// helpCommands is the full dispatch list from run()'s switch — every
// live subcommand. The watch/top tombstones stay out: they already
// print their own guidance and exit nonzero. A new subcommand belongs
// here and in commandDocs; TestHelpTableMatchesDispatchList keeps the
// two aligned.
var helpCommands = []string{
	"init", "status", "backlog", "task", "escalations",
	"create", "update", "answer", "mcp", "protocol",
	"daemon", "help", "version",
}

// The usage table and this file's dispatch list name exactly the same
// commands, in the same order — so the loop in TestSubcommandHelpFlags
// provably covers every command the table can answer for.
func TestHelpTableMatchesDispatchList(t *testing.T) {
	if len(commandDocs) != len(helpCommands) {
		t.Fatalf("commandDocs has %d rows, helpCommands lists %d", len(commandDocs), len(helpCommands))
	}
	for i, c := range commandDocs {
		if c.name != helpCommands[i] {
			t.Errorf("row %d: commandDocs %q vs helpCommands %q", i, c.name, helpCommands[i])
		}
	}
}

// runCLISplit executes the built binary with stdout and stderr captured
// separately — the acceptance here is stream-specific (usage on stdout,
// stderr silent), which runCLI's combined capture cannot see.
func runCLISplit(t *testing.T, dir string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	cmd.Dir = dir
	cmd.Env = cliEnv()
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			t.Fatalf("run %v: %v", args, err)
		}
		return outb.String(), errb.String(), ee.ExitCode()
	}
	return outb.String(), errb.String(), 0
}

// Table-driven over the full dispatch list: `tuhdoo <cmd> -h` and
// `tuhdoo <cmd> --help` print that command's usage to stdout and exit
// 0, with stderr silent. Every description line of the per-command help
// also appears verbatim in the global `tuhdoo help` output — the
// behavioral face of the one-source-of-truth table. Runs in a plain
// non-repo directory: help must work before init, outside any repo,
// with no daemon anywhere.
func TestSubcommandHelpFlags(t *testing.T) {
	dir, err := os.MkdirTemp("", "tuhdoo-help")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	global, _, code := runCLISplit(t, dir, "help")
	if code != 0 {
		t.Fatalf("help exit %d; output:\n%s", code, global)
	}

	for _, cmd := range helpCommands {
		for _, flg := range []string{"-h", "--help"} {
			stdout, stderr, code := runCLISplit(t, dir, cmd, flg)
			if code != 0 {
				t.Errorf("tuhdoo %s %s exit %d; stderr:\n%s", cmd, flg, code, stderr)
				continue
			}
			if stderr != "" {
				t.Errorf("tuhdoo %s %s wrote to stderr:\n%s", cmd, flg, stderr)
			}
			if !strings.HasPrefix(stdout, "usage: tuhdoo "+cmd) {
				t.Errorf("tuhdoo %s %s stdout does not open with %q:\n%s",
					cmd, flg, "usage: tuhdoo "+cmd, stdout)
				continue
			}
			// Every description line reappears in the global help: the
			// per-command text is the global entry, not a second copy.
			lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
			for _, line := range lines[1:] {
				if !strings.Contains(global, strings.TrimSpace(line)) {
					t.Errorf("tuhdoo %s %s line %q is absent from the global help — help text drifted",
						cmd, flg, strings.TrimSpace(line))
				}
			}
		}
	}

	// -h/--help only answers for known commands; an unknown command
	// stays a loud error.
	stdout, stderr, code := runCLISplit(t, dir, "bogus", "--help")
	if code == 0 {
		t.Errorf("tuhdoo bogus --help exited 0; stdout:\n%s", stdout)
	}
	if !strings.Contains(stderr, `unknown command "bogus"`) {
		t.Errorf("tuhdoo bogus --help stderr missing the unknown-command error:\n%s", stderr)
	}

	// A pure print leaves no trace: nothing at all in the directory —
	// no .git, no daemon discovery file, no data branch.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("help flags created files in a non-repo dir: %v", names)
	}
}
