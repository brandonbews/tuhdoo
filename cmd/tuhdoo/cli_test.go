package main

// CLI integration tests: the real binary, real repos, a real
// auto-spawned daemon.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/brandonbews/tuhdoo/internal/event"
)

// binPath is the CLI binary under test, built once in TestMain.
var binPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "tuhdoo-bin")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	binPath = filepath.Join(dir, "tuhdoo")
	out, err := exec.Command("go", "build", "-o", binPath, ".").CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "build: %v\n%s", err, out)
		os.RemoveAll(dir)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// cliEnv is the environment for the CLI and everything it spawns: no
// color, and a hermetic git.
func cliEnv() []string {
	return append(os.Environ(),
		"NO_COLOR=1",
		"GIT_AUTHOR_NAME=Test Bot",
		"GIT_AUTHOR_EMAIL=bot@example.com",
		"GIT_COMMITTER_NAME=Test Bot",
		"GIT_COMMITTER_EMAIL=bot@example.com",
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_CONFIG_SYSTEM="+os.DevNull,
	)
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = cliEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// newRepo builds a fresh remoteless repo and arranges for any daemon
// spawned inside it to be stopped at cleanup.
func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Cleanup(func() { stopDaemon(dir) })
	runGit(t, dir, "init", "--quiet", "-b", "main")
	return dir
}

// stopDaemon terminates the repo's daemon (if one is running) and waits
// for its clean shutdown, so temp dirs delete cleanly.
func stopDaemon(repo string) {
	discPath := filepath.Join(repo, ".git", "tuhdoo", "daemon.json")
	pid, _, err := readDiscovery(discPath)
	if err != nil || pid <= 0 {
		return
	}
	_ = syscall.Kill(pid, syscall.SIGTERM)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(discPath); os.IsNotExist(err) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func readDiscovery(path string) (pid int, socket string, err error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, "", err
	}
	var d struct {
		PID    int    `json:"pid"`
		Socket string `json:"socket"`
	}
	if err := json.Unmarshal(b, &d); err != nil {
		return 0, "", err
	}
	return d.PID, d.Socket, nil
}

// runCLI executes the built binary in repo and returns combined output
// and exit code.
func runCLI(t *testing.T, repo string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	cmd.Dir = repo
	cmd.Env = cliEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return string(out), ee.ExitCode()
		}
		t.Fatalf("run %v: %v\n%s", args, err, out)
	}
	return string(out), 0
}

// mustContain asserts every substring is present.
func mustContain(t *testing.T, out string, subs ...string) {
	t.Helper()
	for _, s := range subs {
		if !strings.Contains(out, s) {
			t.Errorf("output missing %q; output:\n%s", s, out)
		}
	}
}

// ---- seeding helpers: talk to the spawned daemon's HTTP API ----

func apiClient(t *testing.T, repo string) *http.Client {
	t.Helper()
	_, socket, err := readDiscovery(filepath.Join(repo, ".git", "tuhdoo", "daemon.json"))
	if err != nil {
		t.Fatalf("read daemon.json: %v", err)
	}
	return &http.Client{Transport: unixTransport(socket)}
}

// api performs one seeding request and fails the test on a non-2xx.
func api(t *testing.T, hc *http.Client, method, path, actor string, body any) []byte {
	t.Helper()
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, "http://tuhdoo"+path, rd)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if actor != "" {
		req.Header.Set("X-Tuhdoo-Actor", actor)
	}
	resp, err := hc.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s %s: status %d: %s", method, path, resp.StatusCode, data)
	}
	return data
}

func createTask(t *testing.T, hc *http.Client, item map[string]any) string {
	t.Helper()
	var resp struct {
		IDs []string `json:"ids"`
	}
	if err := json.Unmarshal(api(t, hc, "POST", "/v0/tasks", "brandon", []map[string]any{item}), &resp); err != nil || len(resp.IDs) != 1 {
		t.Fatalf("create task: ids=%v err=%v", resp.IDs, err)
	}
	return resp.IDs[0]
}

// ---- tests ----

// init on a fresh remoteless repo creates the branch, prints
// local-only as a normal state, exits 0; re-running is idempotent.
func TestInitRemoteless(t *testing.T) {
	repo := newRepo(t)

	out, code := runCLI(t, repo, "init")
	if code != 0 {
		t.Fatalf("init exit %d; output:\n%s", code, out)
	}
	mustContain(t, out, "local-only", "normal state", branchName(), "branches-ignore")
	// The universal MCP harness snippet (002 T4) rides the same output.
	mustContain(t, out, `"mcpServers"`, `"command": "tuhdoo"`, `"args": ["mcp"]`)
	// The branch-protection line: hosts whose rulesets require PRs on
	// all branches would silently break the daemon's direct pushes.
	mustContain(t, out, "Branch protection", "fast-forward only, never force")
	// The auto-deploy pointer: hosts that autodeploy branches need the
	// data branch silenced, and how differs by host (on Vercel a
	// dashboard ignore rule is not even sufficient) — so the binary
	// carries a pointer to the per-host guidance, not the guidance.
	mustContain(t, out, "Auto-deploys", "preview builder",
		"https://tuhdoo.com/docs/joining")
	// The docs pointer: the least-specific stable URL is a permanent
	// promise baked into every shipped binary — recipes and guides live
	// there, not in the binary, so they can be fixed without a release.
	mustContain(t, out, "Docs & workflow recipes: https://tuhdoo.com/docs")
	// The protocol pointer: init tells the operator how to hand agents
	// the protocol text, but never writes a file into the host repo —
	// the doc stays in the binary, printed on demand.
	mustContain(t, out, "Agent protocol: tuhdoo protocol")
	if strings.Contains(strings.ToLower(out), "error") {
		t.Errorf("init output should not mention errors:\n%s", out)
	}
	head1 := strings.TrimSpace(runGit(t, repo, "rev-parse", "--verify", "refs/heads/tuhdoo"))

	out2, code2 := runCLI(t, repo, "init")
	if code2 != 0 {
		t.Fatalf("second init exit %d; output:\n%s", code2, out2)
	}
	mustContain(t, out2, "local-only")
	head2 := strings.TrimSpace(runGit(t, repo, "rev-parse", "--verify", "refs/heads/tuhdoo"))
	if head1 != head2 {
		t.Errorf("init is not idempotent: head moved %s -> %s", head1, head2)
	}
}

// init takes no flags and no arguments: anything extra is rejected
// loudly instead of silently ignored, and --as gets its own explanation
// — init writes no events, so there is no principal to act as.
func TestInitRejectsArgs(t *testing.T) {
	repo := newRepo(t)

	out, code := runCLI(t, repo, "init", "--force")
	if code == 0 {
		t.Fatalf("init --force exited 0; output:\n%s", out)
	}
	mustContain(t, out, `unexpected argument "--force"`, "usage: tuhdoo init")

	out, code = runCLI(t, repo, "init", "extra")
	if code == 0 {
		t.Fatalf("init extra exited 0; output:\n%s", out)
	}
	mustContain(t, out, `unexpected argument "extra"`, "usage: tuhdoo init")

	for _, args := range [][]string{
		{"init", "--as", "brandon"},
		{"init", "--as=brandon"},
	} {
		out, code = runCLI(t, repo, args...)
		if code == 0 {
			t.Fatalf("%v exited 0; output:\n%s", args, out)
		}
		mustContain(t, out, "init writes no events", "user.email",
			"tuhdoo.principal", "usage: tuhdoo init")
	}

	// Rejection happens before anything touches the repo: no daemon.
	if _, err := os.Stat(filepath.Join(repo, ".git", "tuhdoo", "daemon.json")); !os.IsNotExist(err) {
		t.Errorf("rejected init still spawned a daemon (stat err: %v)", err)
	}
}

// From cold, `tuhdoo status` auto-spawns the daemon and reports; a
// subsequent command reuses the same daemon (single pid).
func TestAutoSpawnSingleDaemon(t *testing.T) {
	repo := newRepo(t)
	discPath := filepath.Join(repo, ".git", "tuhdoo", "daemon.json")
	if _, err := os.Stat(discPath); !os.IsNotExist(err) {
		t.Fatalf("daemon.json exists before any command (stat err: %v)", err)
	}

	out, code := runCLI(t, repo, "status")
	if code != 0 {
		t.Fatalf("status exit %d; output:\n%s", code, out)
	}
	mustContain(t, out, "local-only", "ready", "escalations")
	pid1, _, err := readDiscovery(discPath)
	if err != nil {
		t.Fatalf("no daemon.json after status: %v", err)
	}

	out2, code2 := runCLI(t, repo, "backlog")
	if code2 != 0 {
		t.Fatalf("backlog exit %d; output:\n%s", code2, out2)
	}
	pid2, _, err := readDiscovery(discPath)
	if err != nil {
		t.Fatalf("daemon.json gone after backlog: %v", err)
	}
	if pid1 != pid2 {
		t.Errorf("daemon respawned: pid %d then %d", pid1, pid2)
	}
}

// With a remote configured, status reports the live sync state (B7)
// instead of local-only.
func TestStatusWithRemote(t *testing.T) {
	repo := newRepo(t)
	bare, err := os.MkdirTemp("", "tuhdoo-remote")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(bare) })
	runGit(t, bare, "init", "--quiet", "--bare")
	runGit(t, repo, "remote", "add", "origin", bare)

	out, code := runCLI(t, repo, "status")
	if code != 0 {
		t.Fatalf("status exit %d; output:\n%s", code, out)
	}
	mustContain(t, out, `syncing with "origin"`)
	if strings.Contains(out, "local-only") {
		t.Errorf("status shows local-only despite a configured remote:\n%s", out)
	}
}

// backlog / task / escalations render a seeded state.
func TestReadCommandsRenderSeededState(t *testing.T) {
	repo := newRepo(t)
	if out, code := runCLI(t, repo, "init"); code != 0 {
		t.Fatalf("init exit %d; output:\n%s", code, out)
	}
	hc := apiClient(t, repo)

	parser := createTask(t, hc, map[string]any{
		"title": "write the parser", "priority": 5,
		"description": "Parse the thing.\nAcceptance: it parses.",
		"labels":      []string{"go"},
	})
	docs := createTask(t, hc, map[string]any{
		"title": "ship the docs", "depends_on": []string{parser},
	})
	flake := createTask(t, hc, map[string]any{"title": "investigate the flake"})
	license := createTask(t, hc, map[string]any{"title": "choose a license"})
	chore := createTask(t, hc, map[string]any{"title": "old chore"})
	wrong := createTask(t, hc, map[string]any{"title": "wrong idea"})
	createTask(t, hc, map[string]any{"title": "sweep the floor", "priority": 1})
	createTask(t, hc, map[string]any{"title": "polish the manual", "status": "held", "priority": 2})
	createTask(t, hc, map[string]any{"title": "idea: dark mode", "status": "inbox"})

	api(t, hc, "PATCH", "/v0/tasks/"+chore, "brandon", map[string]any{"status": "done"})
	api(t, hc, "PATCH", "/v0/tasks/"+wrong, "brandon", map[string]any{"status": "cancelled"})
	api(t, hc, "POST", "/v0/claims", "brandon/a1", map[string]any{"task": flake})
	api(t, hc, "POST", "/v0/notes", "brandon/a1", map[string]any{
		"task": flake, "text": "the flake is in the retry loop",
	})
	api(t, hc, "POST", "/v0/notes", "brandon/a1", map[string]any{
		"task": flake, "text": "confirmed: the retry loop swallows the race",
	})
	var lic struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(api(t, hc, "POST", "/v0/escalations", "brandon/a2", map[string]any{
		"task": license, "question": "Which license do we ship under?", "blocking": true,
	}), &lic); err != nil {
		t.Fatalf("unmarshal escalation: %v", err)
	}
	var esc struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(api(t, hc, "POST", "/v0/escalations", "brandon/a3", map[string]any{
		"task": parser, "question": "Do we need unicode?", "blocking": false,
	}), &esc); err != nil {
		t.Fatalf("unmarshal escalation: %v", err)
	}
	api(t, hc, "POST", "/v0/escalations/answer", "brandon", map[string]any{
		"escalation": esc.ID, "answer": "ASCII first.",
	})

	// grepState mimics `out | grep <state>`: the acceptance contract is
	// that a state name selects exactly that state's rows.
	grepState := func(out, state string) []string {
		var m []string
		for _, l := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
			if strings.Contains(l, state) {
				m = append(m, l)
			}
		}
		return m
	}

	// backlog: serialized column output (T7, 2026-07-31) — one row per
	// task, STATE column, dep:/esc: waiting IDs, no ANSI ever. Status
	// words are the stored words (2026-08-01); held stays the on-hold
	// token, one word for grep/awk.
	out, code := runCLI(t, repo, "backlog")
	if code != 0 {
		t.Fatalf("backlog exit %d; output:\n%s", code, out)
	}
	if strings.Contains(out, "\x1b") {
		t.Errorf("serialized backlog contains ANSI escapes:\n%q", out)
	}
	mustContain(t, out, "ID", "STATE", "PRI", "HOLDER", "LABELS", "WAITING", "TITLE")
	for state, wantRows := range map[string][]string{
		"ready":       {"sweep the floor", "write the parser"},
		"in-progress": {"investigate the flake"},
		"blocked":     {"ship the docs", "choose a license"},
		"on-hold":     {"polish the manual"},
		"inbox":       {"idea: dark mode"},
		"done":        {"old chore"},
		"cancelled":   {"wrong idea"},
	} {
		lines := grepState(out, state)
		if len(lines) != len(wantRows) {
			t.Errorf("grep %q selects %d lines, want %d:\n%s", state, len(lines), len(wantRows), out)
			continue
		}
		for i, sub := range wantRows {
			if !strings.Contains(lines[i], sub) {
				t.Errorf("grep %q row %d missing %q: %q", state, i, sub, lines[i])
			}
		}
	}
	// Ready is priority-ordered — P0-highest (2026-08-21): p1
	// floor-sweeping before p5 parser, checked by row order above;
	// waiting reasons are IDs, not prose; the holder is attributed on
	// the in-progress row; labels ride the parser's row as a plain
	// comma cell.
	mustContain(t, out, "dep:"+parser, "esc:"+lic.ID, "brandon/a1")
	if ready := grepState(out, "ready"); len(ready) > 1 && !strings.Contains(ready[1], "go") {
		t.Errorf("parser row lost its label cell: %q", ready[1])
	}
	if strings.Contains(out, "depends on ") || strings.Contains(out, "escalation:") {
		t.Errorf("backlog still renders prose waiting-reasons:\n%s", out)
	}

	// task <id>: metadata, description, history.
	out, code = runCLI(t, repo, "task", flake)
	if code != 0 {
		t.Fatalf("task exit %d; output:\n%s", code, out)
	}
	mustContain(t, out, flake, "investigate the flake",
		"claimed by brandon/a1", "the flake is in the retry loop",
		"confirmed: the retry loop swallows the race")
	// Consecutive history entries are separated by one blank line
	// (entry formatting, 2026-08-03): the first note's body, a blank,
	// then the second note's header.
	if !strings.Contains(out, "the flake is in the retry loop\n\n  ") {
		t.Errorf("no blank separator between history entries:\n%s", out)
	}

	out, code = runCLI(t, repo, "task", parser)
	if code != 0 {
		t.Fatalf("task exit %d; output:\n%s", code, out)
	}
	mustContain(t, out, "Parse the thing.", "Acceptance: it parses.",
		"Do we need unicode?", "A (brandon): ASCII first.",
		// The docs task depends on the parser, so the parser's one-shot
		// view carries the reverse edge as a needed-by row — full ID,
		// status word, title (edge rows, 2026-08-11).
		"needed by   "+docs+"  open  ship the docs")

	// task with an unknown id: clear failure.
	out, code = runCLI(t, repo, "task", "t-nope")
	if code == 0 {
		t.Errorf("task t-nope exited 0; output:\n%s", out)
	}
	mustContain(t, out, "unknown task")

	// task <short-form>: resolves to the same task — the short form is
	// the human input contract (T7); output still carries the full ID.
	out, code = runCLI(t, repo, "task", event.ShortID(flake))
	if code != 0 {
		t.Fatalf("task %s exit %d; output:\n%s", event.ShortID(flake), code, out)
	}
	mustContain(t, out, flake, "investigate the flake", "claimed by brandon/a1")

	// task with an ambiguous fragment (every ULID starts with 0): an
	// error listing the candidates, never a guess. The old-era prefix on
	// the same fragment matches nothing here — these tasks are all
	// tuh-minted, and a fragment's prefix is literal (T7, 2026-07-31).
	out, code = runCLI(t, repo, "task", "tuh-0")
	if code == 0 {
		t.Errorf("ambiguous fragment exited 0; output:\n%s", out)
	}
	mustContain(t, out, "ambiguous", event.ShortID(parser), event.ShortID(flake))
	out, code = runCLI(t, repo, "task", "t-0")
	if code == 0 {
		t.Errorf("old-era fragment matched tuh- tasks; output:\n%s", out)
	}
	mustContain(t, out, "unknown task")

	// escalations: serialized column output — one row per escalation,
	// open before answered, the blocking flag its own column, task/actor
	// attribution and a compact UTC timestamp per row, no ANSI.
	out, code = runCLI(t, repo, "escalations")
	if code != 0 {
		t.Fatalf("escalations exit %d; output:\n%s", code, out)
	}
	if strings.Contains(out, "\x1b") {
		t.Errorf("serialized escalations contains ANSI escapes:\n%q", out)
	}
	mustContain(t, out, "BLOCKING", "ASKED-BY", "RAISED", "QUESTION",
		lic.ID, esc.ID, license, parser, "brandon/a2", "brandon/a3",
		"Which license do we ship under?", "Do we need unicode?", "ASCII first.")
	if open := grepState(out, "open"); len(open) != 1 ||
		!strings.Contains(open[0], "blocking") || !strings.Contains(open[0], lic.ID) {
		t.Errorf("grep open must select exactly the blocking license row:\n%s", out)
	}
	if ans := grepState(out, "answered"); len(ans) != 1 ||
		!strings.Contains(ans[0], "brandon") || !strings.Contains(ans[0], "ASCII first.") {
		t.Errorf("grep answered must select exactly the answered row, with attribution:\n%s", out)
	}
	// The open question renders before the answered one.
	if strings.Index(out, "Which license") > strings.Index(out, "ASCII first.") {
		t.Errorf("open escalation rendered after the answered one:\n%s", out)
	}
	// The RAISED cell is one token: compact UTC, no interior spaces.
	if !strings.Contains(out, "Z ") && !strings.HasSuffix(out, "Z\n") {
		t.Errorf("no compact UTC timestamp in escalations output:\n%s", out)
	}

	// status: counts line reflects the seeded state.
	out, code = runCLI(t, repo, "status")
	if code != 0 {
		t.Fatalf("status exit %d; output:\n%s", code, out)
	}
	mustContain(t, out, "local-only",
		"2 ready", "1 in progress", "2 blocked", "1 on hold", "1 inbox",
		"1 done", "1 cancelled", "1 question open", "brandon/a1")
}

// The watch and top verbs died in Cycle 4 (002 T7): the TUI is bare
// `tuhdoo`, watch is its --watch mode. The tombstones must say so.
func TestDeadVerbTombstones(t *testing.T) {
	repo := newRepo(t)
	out, code := runCLI(t, repo, "watch")
	if code == 0 {
		t.Fatalf("watch should exit nonzero; output:\n%s", out)
	}
	mustContain(t, out, "tuhdoo --watch")

	out, code = runCLI(t, repo, "top")
	if code == 0 {
		t.Fatalf("top should exit nonzero; output:\n%s", out)
	}
	mustContain(t, out, "bare tuhdoo")
}

// Verb-less dispatch (Cycle 4): help is a real command on stdout; bare
// invocation without a terminal falls back to usage (the guarded
// launch) instead of half-starting a TUI; unknown commands and bad
// flags stay loud.
func TestVerblessDispatch(t *testing.T) {
	repo := newRepo(t)

	out, code := runCLI(t, repo, "help")
	if code != 0 {
		t.Fatalf("help exit %d; output:\n%s", code, out)
	}
	mustContain(t, out, "usage: tuhdoo", "--watch", "mcp")
	if strings.Contains(out, "top ") {
		t.Errorf("help still lists the dead top verb:\n%s", out)
	}

	out, code = runCLI(t, repo, "bogus")
	if code == 0 {
		t.Fatalf("unknown command should exit nonzero; output:\n%s", out)
	}
	mustContain(t, out, `unknown command "bogus"`, "usage: tuhdoo")

	// runCLI pipes output, so stdout is not a TTY: the guarded launch
	// must print usage rather than start Bubble Tea.
	out, code = runCLI(t, repo)
	if code == 0 {
		t.Fatalf("bare tuhdoo without a TTY should exit nonzero; output:\n%s", out)
	}
	mustContain(t, out, "usage: tuhdoo")

	out, code = runCLI(t, repo, "--verbose")
	if code == 0 {
		t.Fatalf("unknown flag should exit nonzero; output:\n%s", out)
	}
	mustContain(t, out, "usage: tuhdoo")

	out, code = runCLI(t, repo, "--watch", "--as", "brandon")
	if code == 0 {
		t.Fatalf("--watch with --as should be rejected; output:\n%s", out)
	}
	mustContain(t, out, "watch mode")
}
