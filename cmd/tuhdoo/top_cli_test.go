package main

// Integration test for tuhdoo top: the real model driven by key
// messages against a real auto-spawned daemon (cli_test.go harness) —
// everything except the terminal itself. Covers the acceptance loop:
// a blocking escalation answered from the TUI returns the task to the
// ready pool; a reprioritize is visible to the next claim; a cancel
// lands on the data branch stamped with the acting human principal.

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// refreshTop performs one poll cycle by hand: run fetchCmd, feed the
// snapshot through Update.
func refreshTop(t *testing.T, m topModel) topModel {
	t.Helper()
	msg := fetchCmd(m.c)()
	sm, ok := msg.(snapMsg)
	if !ok {
		t.Fatalf("fetchCmd produced %T", msg)
	}
	if sm.err != nil {
		t.Fatalf("fetch: %v", sm.err)
	}
	mm, _ := m.Update(sm)
	return mm.(topModel)
}

// moveTo walks the cursor to the row with the given id using j/k, as a
// user would.
func moveTo(t *testing.T, m topModel, id string) topModel {
	t.Helper()
	target := -1
	for i, r := range m.rows {
		if r.id() == id {
			target = i
			break
		}
	}
	if target < 0 {
		t.Fatalf("no row for %s; rows: %+v", id, m.rows)
	}
	key := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	if target < m.cursor {
		key = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
	}
	for m.cursor != target {
		before := m.cursor
		m, _ = press(t, m, key)
		if m.cursor == before {
			t.Fatalf("cursor stuck at %d walking to %s", m.cursor, id)
		}
	}
	return m
}

// act runs the command a submit produced and feeds the result back
// through Update, failing on an action error.
func act(t *testing.T, m topModel, cmd tea.Cmd) topModel {
	t.Helper()
	if cmd == nil {
		t.Fatal("no command produced")
	}
	am, ok := cmd().(actionMsg)
	if !ok {
		t.Fatalf("command produced %T, want actionMsg", cmd())
	}
	if am.err != nil {
		t.Fatalf("action failed: %v", am.err)
	}
	mm, _ := m.Update(am)
	return mm.(topModel)
}

// grepDataBranch searches committed event files on the data branch.
func grepDataBranch(repo, pattern string) (string, bool) {
	cmd := exec.Command("git", "grep", "-h", pattern, "refs/heads/tuhdoo", "--", "events")
	cmd.Dir = repo
	cmd.Env = cliEnv()
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	return string(out), true
}

func TestTopSteersRealDaemon(t *testing.T) {
	repo := newRepo(t)
	if out, code := runCLI(t, repo, "init"); code != 0 {
		t.Fatalf("init exit %d; output:\n%s", code, out)
	}
	hc := apiClient(t, repo)

	lic := createTask(t, hc, map[string]any{"title": "choose a license"})
	parser := createTask(t, hc, map[string]any{"title": "write the parser"})
	wrong := createTask(t, hc, map[string]any{"title": "wrong idea"})
	api(t, hc, "POST", "/v0/escalations", "brandon/a2", map[string]any{
		"task": lic, "question": "Which license do we ship under?", "blocking": true,
	})

	_, socket, err := readDiscovery(filepath.Join(repo, ".git", "tuhdoo", "daemon.json"))
	if err != nil {
		t.Fatalf("read daemon.json: %v", err)
	}
	c := newClient(socket)
	m := topModel{c: c, api: httpSteering{c: c, actor: "brandon"}, actor: "brandon"}
	m = refreshTop(t, m)

	// ---- answer the blocking escalation from the TUI ----
	if r, ok := m.selected(); !ok || r.kind != rowEscalation {
		t.Fatalf("first row is not the escalation: %+v", m.rows)
	}
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m, cmd := press(t, m, append(runes("MIT."), keyOf(tea.KeyEnter))...)
	m = act(t, m, cmd)

	// The task returned to the ready pool: a direct claim now succeeds
	// (it would 409 while the blocking escalation was open).
	api(t, hc, "POST", "/v0/claims", "brandon/a9", map[string]any{"task": lic})

	// ---- reprioritize from the TUI ----
	m = refreshTop(t, m)
	m = moveTo(t, m, parser)
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m, cmd = press(t, m, append(runes("7"), keyOf(tea.KeyEnter))...)
	m = act(t, m, cmd)

	// Visible to the next claimant: claim-next serves the same
	// priority-ordered ready pool as get_backlog, so it must now hand
	// out parser (p7) ahead of wrong (p0, earlier ULID).
	var next struct {
		Task taskJSON `json:"task"`
	}
	if err := json.Unmarshal(api(t, hc, "POST", "/v0/claims", "brandon/a8",
		map[string]any{"next": true}), &next); err != nil {
		t.Fatalf("claim next: %v", err)
	}
	if next.Task.ID != parser {
		t.Fatalf("claim_next served %s (%q), want reprioritized %s", next.Task.ID, next.Task.Title, parser)
	}
	if next.Task.Priority != 7 {
		t.Errorf("priority = %d, want 7", next.Task.Priority)
	}

	// ---- cancel from the TUI ----
	m = refreshTop(t, m)
	m = moveTo(t, m, wrong)
	m, cmd = press(t, m,
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = act(t, m, cmd)

	var h hydratedTask
	if err := c.get("/v0/tasks/"+wrong, &h); err != nil {
		t.Fatalf("get %s: %v", wrong, err)
	}
	if h.Task.Status != "cancelled" {
		t.Fatalf("status = %q, want cancelled", h.Task.Status)
	}

	// The cancel event lands on the data branch stamped with the acting
	// human principal. Non-eager writes ride the 2s debounce, so poll.
	deadline := time.Now().Add(8 * time.Second)
	var line string
	for {
		if out, ok := grepDataBranch(repo, `"status":"cancelled"`); ok {
			line = out
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("cancel event never committed to the data branch")
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !strings.Contains(line, `"actor":"brandon"`) {
		t.Errorf("cancel event not stamped with the acting human; event:\n%s", line)
	}
	if !strings.Contains(line, `"task":"`+wrong+`"`) {
		t.Errorf("cancel event not on task %s; event:\n%s", wrong, line)
	}

	// The interactive session ends cleanly from nav mode.
	_, quit := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if quit == nil {
		t.Fatal("q should quit")
	}
	if _, ok := quit().(tea.QuitMsg); !ok {
		t.Errorf("q produced %T, want tea.QuitMsg", quit())
	}
}
