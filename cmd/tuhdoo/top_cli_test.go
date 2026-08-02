package main

// Integration test for tuhdoo top: the real model driven by key
// messages against a real auto-spawned daemon (cli_test.go harness) —
// everything except the terminal itself. Covers the acceptance loop:
// a blocking escalation answered from the TUI returns the task to the
// ready pool; a reprioritize is visible to the next claim; a cancel
// lands on the data branch (as the task.cancelled event) stamped with
// the acting human principal.

import (
	"bytes"
	"encoding/json"
	"net/http"
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
	m := topModel{c: c, api: httpSteering{c: c, actor: "brandon"}, actor: "brandon", armed: true}
	m = refreshTop(t, m)

	// ---- answer the blocking escalation from the TUI ----
	// Enter on the Needs Input row routes into the task view with the
	// escalation preselected (task-view rework, 2026-08-01); enter there
	// opens answer entry.
	if r, ok := m.selected(); !ok || r.kind != rowEscalation {
		t.Fatalf("first row is not the escalation: %+v", m.rows)
	}
	m, _ = press(t, m, keyOf(tea.KeyEnter))
	if m.mode != modeDetail || m.detailID != lic {
		t.Fatalf("enter on the escalation row: mode %d detail %q, want the task view of %s", m.mode, m.detailID, lic)
	}
	m, _ = press(t, m, keyOf(tea.KeyEnter))
	if m.mode != modeAnswer {
		t.Fatalf("enter on the selected question: mode %d, want modeAnswer", m.mode)
	}
	m, cmd := press(t, m, append(runes("MIT."), keyOf(tea.KeyEnter))...)
	m = act(t, m, cmd)
	// Back out of the task view to the list for the steps below.
	m, _ = press(t, m, keyOf(tea.KeyEsc))

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

	// ---- edit title and description from the task view ----
	// e edits the title (single-line, prefilled), E the description
	// (multi-line, ctrl+s submits): the same PATCH — and task.updated
	// ledger event — as `tuhdoo update`, so a restart replays the edits.
	m = refreshTop(t, m)
	m = moveTo(t, m, wrong)
	m, _ = press(t, m, keyOf(tea.KeyEnter))
	if m.mode != modeDetail || m.detailID != wrong {
		t.Fatalf("enter on the task row: mode %d detail %q, want the task view of %s", m.mode, m.detailID, wrong)
	}
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if m.mode != modeEditTitle || m.input.String() != "wrong idea" {
		t.Fatalf("e: mode %d input %q, want the prefilled title editor", m.mode, m.input)
	}
	m, _ = press(t, m, keyOf(tea.KeyCtrlU)) // cursor at the end: clears the prefill
	m, cmd = press(t, m, append(runes("right idea"), keyOf(tea.KeyEnter))...)
	m = act(t, m, cmd)
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'E'}})
	if m.mode != modeEditDesc || !m.input.multiline {
		t.Fatalf("E: mode %d multiline %v, want the multi-line description editor", m.mode, m.input.multiline)
	}
	m, _ = press(t, m, runes("Line one.")...)
	m, _ = press(t, m, keyOf(tea.KeyEnter)) // a newline, never a submit
	m, cmd = press(t, m, append(runes("Line two."), keyOf(tea.KeyCtrlS))...)
	m = act(t, m, cmd)

	// The edits are daemon state now, and the next poll re-renders the
	// still-open task view with the new content.
	var edited hydratedTask
	if err := c.get("/v0/tasks/"+wrong, &edited); err != nil {
		t.Fatalf("get %s: %v", wrong, err)
	}
	if edited.Task.Title != "right idea" || edited.Task.Description != "Line one.\nLine two." {
		t.Fatalf("edited task = %q / %q, want the new title and description",
			edited.Task.Title, edited.Task.Description)
	}
	m = refreshTop(t, m)
	if v := m.View(); !strings.Contains(v, "right idea") || !strings.Contains(v, "Line two.") {
		t.Errorf("edits not rendered after refresh; view:\n%s", v)
	}
	m, _ = press(t, m, keyOf(tea.KeyEsc))

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

	// The task.cancelled event — stored bytes never change (T3) — lands
	// on the data branch stamped with the acting human principal.
	// Non-eager writes ride the 2s debounce, so poll.
	deadline := time.Now().Add(8 * time.Second)
	var line string
	for {
		if out, ok := grepDataBranch(repo, `"status":"cancelled"`); ok {
			line = out
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("task.cancelled event never committed to the data branch")
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !strings.Contains(line, `"actor":"brandon"`) {
		t.Errorf("task.cancelled event not stamped with the acting human; event:\n%s", line)
	}
	if !strings.Contains(line, `"task":"`+wrong+`"`) {
		t.Errorf("task.cancelled event not on task %s; event:\n%s", wrong, line)
	}

	// The edits landed on the branch too — the log is append-only, so
	// their earlier task.updated events committed with (or before) the
	// cancelled event just found — stamped with the acting human.
	if out, ok := grepDataBranch(repo, `"title":"right idea"`); !ok {
		t.Error("task.updated event for the title edit never committed to the data branch")
	} else if !strings.Contains(out, `"actor":"brandon"`) {
		t.Errorf("title edit event not stamped with the acting human; event:\n%s", out)
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

// Quick capture against the real daemon (2026-07-31): i → title →
// enter creates a real inbox task as the steering actor, it renders in
// the INBOX section on the next poll, and the v2 task.created event —
// status in the payload — lands on the data branch.
func TestTopQuickCaptureRealDaemon(t *testing.T) {
	repo := newRepo(t)
	if out, code := runCLI(t, repo, "init"); code != 0 {
		t.Fatalf("init exit %d; output:\n%s", code, out)
	}
	_, socket, err := readDiscovery(filepath.Join(repo, ".git", "tuhdoo", "daemon.json"))
	if err != nil {
		t.Fatalf("read daemon.json: %v", err)
	}
	c := newClient(socket)
	m := topModel{c: c, api: httpSteering{c: c, actor: "brandon"}, actor: "brandon", armed: true}
	m = refreshTop(t, m)

	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	if m.mode != modeCapture {
		t.Fatalf("i: mode = %d, want modeCapture", m.mode)
	}
	m, cmd := press(t, m, append(runes("idea: sparkline history"), keyOf(tea.KeyEnter))...)
	m = act(t, m, cmd)

	// The capture is a real task: inbox status, steering actor, shown in
	// the dim section on the next poll — and never claimable.
	m = refreshTop(t, m)
	var captured stateTask
	for _, task := range m.snap.state.Tasks {
		if task.Title == "idea: sparkline history" {
			captured = task
		}
	}
	if captured.ID == "" || captured.Status != "inbox" {
		t.Fatalf("captured task = %+v, want an inbox task", captured)
	}
	if h := m.snap.tasks[captured.ID]; h.Task.CreatedBy != "brandon" {
		t.Fatalf("captured task created_by = %q, want the steering actor", h.Task.CreatedBy)
	}
	if v := m.View(); !strings.Contains(v, "INBOX (1)") {
		t.Errorf("capture not rendered in the INBOX section; view:\n%s", v)
	}
	body, _ := json.Marshal(map[string]any{"next": true})
	req, err := http.NewRequest("POST", "http://tuhdoo/v0/claims", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Tuhdoo-Actor", "brandon/a1")
	resp, err := apiClient(t, repo).Do(req)
	if err != nil {
		t.Fatalf("claim_next: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("claim_next with only a capture: status %d, want 409", resp.StatusCode)
	}

	// End to end means the branch: the capture syncs as an ordinary v2
	// event (debounced, so poll).
	deadline := time.Now().Add(8 * time.Second)
	var line string
	for {
		if out, ok := grepDataBranch(repo, `"status":"inbox"`); ok {
			line = out
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("capture event never committed to the data branch")
		}
		time.Sleep(100 * time.Millisecond)
	}
	for _, want := range []string{`"actor":"brandon"`, `"type":"task.created"`, `"v":2`} {
		if !strings.Contains(line, want) {
			t.Errorf("capture event missing %s; event:\n%s", want, line)
		}
	}
}
