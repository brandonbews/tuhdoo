package main

// Proof of docs/uninstall.md's walk-away claim: the per-machine steps,
// run exactly as the doc writes them, leave zero trace of tuhdoo on a
// machine. The test does not reimplement the steps — it extracts the
// fenced shell blocks marked `<!-- uninstall-test: run -->` from the doc
// and executes them verbatim, so the doc is the single source of truth
// and cannot drift from reality without this test going red.
//
// The `--global` unset is sandboxed: the scripts run with
// GIT_CONFIG_GLOBAL pointing at a file inside the test's temp dir, so
// the developer's real global config is never touched.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

const uninstallDoc = "../../docs/uninstall.md"
const runMarker = "<!-- uninstall-test: run -->"

// runnableDocBlocks extracts, in order, every fenced code block in the
// doc that is immediately preceded by the run marker.
func runnableDocBlocks(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	lines := strings.Split(string(data), "\n")
	var blocks []string
	for i := 0; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != runMarker {
			continue
		}
		j := i + 1
		for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
			j++
		}
		if j >= len(lines) || !strings.HasPrefix(strings.TrimSpace(lines[j]), "```") {
			t.Fatalf("%s: marker on line %d is not followed by a fenced code block", path, i+1)
		}
		var body []string
		k := j + 1
		for ; k < len(lines); k++ {
			if strings.TrimSpace(lines[k]) == "```" {
				break
			}
			body = append(body, lines[k])
		}
		if k >= len(lines) {
			t.Fatalf("%s: unclosed fence after marker on line %d", path, i+1)
		}
		blocks = append(blocks, strings.Join(body, "\n"))
		i = k
	}
	if len(blocks) == 0 {
		t.Fatalf("%s: no %q blocks found — did the markers get renamed?", path, runMarker)
	}
	return blocks
}

// sandboxEnv is cliEnv with GIT_CONFIG_GLOBAL redirected to a writable
// file inside the test's temp dir, so the doc's `git config --global
// --unset` step runs for real without touching the developer's config.
func sandboxEnv(globalCfg string) []string {
	var env []string
	for _, e := range cliEnv() {
		if strings.HasPrefix(e, "GIT_CONFIG_GLOBAL=") {
			continue
		}
		env = append(env, e)
	}
	return append(env, "GIT_CONFIG_GLOBAL="+globalCfg)
}

// sandboxGit runs git in dir with the sandboxed env, returning combined
// output and any error — callers assert on either.
func sandboxGit(t *testing.T, dir string, env []string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestUninstallDocStepsLeaveZeroTrace(t *testing.T) {
	base := t.TempDir()

	// A bare origin, seeded through a first machine so it carries a real
	// daemon-minted data branch (never a fabricated one).
	bare := filepath.Join(base, "origin.git")
	runGit(t, base, "init", "--quiet", "--bare", "-b", "main", bare)
	seed := filepath.Join(base, "seed")
	if err := os.MkdirAll(seed, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stopDaemon(seed) })
	runGit(t, seed, "init", "--quiet", "-b", "main")
	runGit(t, seed, "remote", "add", "origin", bare)
	runGit(t, seed, "commit", "--quiet", "--allow-empty", "-m", "project code")
	runGit(t, seed, "push", "--quiet", "origin", "main")
	if out, code := runCLI(t, seed, "init"); code != 0 {
		t.Fatalf("seed init exit %d; output:\n%s", code, out)
	}
	// A clean shutdown's final sync publishes the branch deterministically.
	stopDaemon(seed)
	runGit(t, bare, "rev-parse", "--verify", "refs/heads/tuhdoo")

	// The machine that will walk away: a full clone (so the ordinary
	// remote-tracking ref exists) with a live, auto-spawned daemon.
	leaver := filepath.Join(base, "leaver")
	runGit(t, base, "clone", "--quiet", bare, leaver)
	t.Cleanup(func() { stopDaemon(leaver) })
	if out, code := runCLI(t, leaver, "init"); code != 0 {
		t.Fatalf("leaver init exit %d; output:\n%s", code, out)
	}

	globalCfg := filepath.Join(base, "gitconfig-global")
	env := sandboxEnv(globalCfg)

	// The syncer's tracking ref appears on the daemon's first cycle,
	// which runs concurrently with init returning — wait for it.
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := sandboxGit(t, leaver, env, "rev-parse", "--verify", "--quiet", "refs/tuhdoo/remote"); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("daemon never created refs/tuhdoo/remote — no first sync cycle?")
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Plant the principal both places the doc says to unset it.
	if out, err := sandboxGit(t, leaver, env, "config", "tuhdoo.principal", "leaver"); err != nil {
		t.Fatalf("set local principal: %v\n%s", err, out)
	}
	if out, err := sandboxGit(t, leaver, env, "config", "--global", "tuhdoo.principal", "leaver-global"); err != nil {
		t.Fatalf("set global principal: %v\n%s", err, out)
	}

	// Pre-flight: the full documented footprint must exist, or the test
	// proves nothing.
	for _, ref := range []string{"refs/heads/tuhdoo", "refs/remotes/origin/tuhdoo", "refs/tuhdoo/remote"} {
		if out, err := sandboxGit(t, leaver, env, "rev-parse", "--verify", "--quiet", ref); err != nil {
			t.Fatalf("footprint incomplete before uninstall: %s missing: %v\n%s", ref, err, out)
		}
	}
	pid, _, err := readDiscovery(filepath.Join(leaver, ".git", "tuhdoo", "daemon.json"))
	if err != nil || pid <= 0 {
		t.Fatalf("no live daemon.json before uninstall: pid=%d err=%v", pid, err)
	}
	if err := syscall.Kill(pid, 0); err != nil {
		t.Fatalf("daemon pid %d not alive before uninstall: %v", pid, err)
	}

	// Execute the doc's own per-machine steps, verbatim and in order.
	for i, block := range runnableDocBlocks(t, uninstallDoc) {
		cmd := exec.Command("sh", "-e", "-c", block)
		cmd.Dir = leaver
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("doc block %d failed: %v\nblock:\n%s\noutput:\n%s", i+1, err, block, out)
		}
	}

	// Zero trace. Process gone (allow a moment for the reaper):
	deadline = time.Now().Add(3 * time.Second)
	for syscall.Kill(pid, 0) == nil {
		if time.Now().After(deadline) {
			t.Fatalf("daemon pid %d still alive after the documented stop step", pid)
		}
		time.Sleep(20 * time.Millisecond)
	}
	// No tuhdoo refs of any kind:
	if out, err := sandboxGit(t, leaver, env, "for-each-ref",
		"refs/heads/tuhdoo", "refs/remotes/origin/tuhdoo", "refs/tuhdoo/"); err != nil || strings.TrimSpace(out) != "" {
		t.Errorf("tuhdoo refs survived uninstall (err=%v):\n%s", err, out)
	}
	// Runtime dir gone:
	if _, err := os.Stat(filepath.Join(leaver, ".git", "tuhdoo")); !os.IsNotExist(err) {
		t.Errorf(".git/tuhdoo survived uninstall (stat err: %v)", err)
	}
	// Principal unset everywhere:
	if out, err := sandboxGit(t, leaver, env, "config", "--get", "tuhdoo.principal"); err == nil {
		t.Errorf("tuhdoo.principal still set after uninstall: %q", strings.TrimSpace(out))
	}
	if b, err := os.ReadFile(globalCfg); err == nil && strings.Contains(string(b), "principal") {
		t.Errorf("global config still mentions the principal:\n%s", b)
	}
	// Worktree untouched — clean status, as if tuhdoo had never run:
	if out := runGit(t, leaver, "status", "--porcelain"); strings.TrimSpace(out) != "" {
		t.Errorf("worktree dirty after uninstall:\n%s", out)
	}
	// And the team's ledger on the remote is unharmed:
	runGit(t, bare, "rev-parse", "--verify", "refs/heads/tuhdoo")
}
