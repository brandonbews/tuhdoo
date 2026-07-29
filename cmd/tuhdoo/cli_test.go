package main

// CLI integration tests: the real binary, real repos, a real
// auto-spawned daemon. Repos use os.MkdirTemp, not t.TempDir — the
// daemon binds a unix socket under the repo's .git and macOS caps
// sun_path at 104 bytes, which t.TempDir paths routinely blow past.

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
	"strings"
	"syscall"
	"testing"
	"time"
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

// newRepo builds a fresh remoteless repo in a short-pathed temp dir and
// arranges for any daemon spawned inside it to be stopped at cleanup.
func newRepo(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "tuhdoo")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() {
		stopDaemon(dir)
		os.RemoveAll(dir)
	})
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
	return &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", socket)
		},
	}}
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

// Test 1: init on a fresh remoteless repo creates the branch, prints
// local-only as a normal state, exits 0; re-running is idempotent.
func TestInitRemoteless(t *testing.T) {
	repo := newRepo(t)

	out, code := runCLI(t, repo, "init")
	if code != 0 {
		t.Fatalf("init exit %d; output:\n%s", code, out)
	}
	mustContain(t, out, "local-only", "normal state", branchName(), "branches-ignore")
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

// Test 2: from cold, `tuhdoo status` auto-spawns the daemon and
// reports; a subsequent command reuses the same daemon (single pid).
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

// Test 3: backlog / task / escalations render a seeded state.
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

	api(t, hc, "PATCH", "/v0/tasks/"+chore, "brandon", map[string]any{"status": "done"})
	api(t, hc, "PATCH", "/v0/tasks/"+wrong, "brandon", map[string]any{"status": "cancelled"})
	api(t, hc, "POST", "/v0/claims", "brandon/a1", map[string]any{"task": flake})
	api(t, hc, "POST", "/v0/notes", "brandon/a1", map[string]any{
		"task": flake, "text": "the flake is in the retry loop",
	})
	api(t, hc, "POST", "/v0/escalations", "brandon/a2", map[string]any{
		"task": license, "question": "Which license do we ship under?", "blocking": true,
	})
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

	// backlog: buckets, holders, blocked reasons, done/cancelled counts.
	out, code := runCLI(t, repo, "backlog")
	if code != 0 {
		t.Fatalf("backlog exit %d; output:\n%s", code, out)
	}
	mustContain(t, out,
		"write the parser", "sweep the floor",
		"investigate the flake", "brandon/a1",
		"ship the docs", "depends on "+parser,
		"choose a license", "escalation: Which license do we ship under?",
		"Done 1", "Cancelled 1",
	)
	// Ready is priority-ordered: p5 parser before p1 floor-sweeping.
	if strings.Index(out, "write the parser") > strings.Index(out, "sweep the floor") {
		t.Errorf("ready queue not priority-ordered:\n%s", out)
	}
	// The claimed task must not render as ready.
	if strings.Index(out, "investigate the flake") < strings.Index(out, "In progress") {
		t.Errorf("claimed task rendered before the In progress section:\n%s", out)
	}

	// task <id>: metadata, description, history.
	out, code = runCLI(t, repo, "task", flake)
	if code != 0 {
		t.Fatalf("task exit %d; output:\n%s", code, out)
	}
	mustContain(t, out, flake, "investigate the flake",
		"claimed by brandon/a1", "the flake is in the retry loop")

	out, code = runCLI(t, repo, "task", parser)
	if code != 0 {
		t.Fatalf("task exit %d; output:\n%s", code, out)
	}
	mustContain(t, out, "Parse the thing.", "Acceptance: it parses.",
		"Do we need unicode?", "A (brandon): ASCII first.")

	// task with an unknown id: clear failure.
	out, code = runCLI(t, repo, "task", "t-nope")
	if code == 0 {
		t.Errorf("task t-nope exited 0; output:\n%s", out)
	}
	mustContain(t, out, "unknown task")

	// escalations: open with absolute age and blocking mark, answered
	// compactly.
	out, code = runCLI(t, repo, "escalations")
	if code != 0 {
		t.Fatalf("escalations exit %d; output:\n%s", code, out)
	}
	mustContain(t, out,
		"Which license do we ship under?", "[blocking]",
		"task "+license, "asked by brandon/a2", "raised 20",
		"Do we need unicode?", "brandon: ASCII first.",
	)
	// The open question must render before the Answered section.
	if strings.Index(out, "Which license") > strings.Index(out, "Answered") {
		t.Errorf("open escalation rendered after Answered section:\n%s", out)
	}

	// status: counts line reflects the seeded state.
	out, code = runCLI(t, repo, "status")
	if code != 0 {
		t.Fatalf("status exit %d; output:\n%s", code, out)
	}
	mustContain(t, out, "local-only",
		"2 ready", "1 in progress", "2 blocked", "1 done", "1 cancelled",
		"1 question open", "brandon/a1")
	_ = docs
}
