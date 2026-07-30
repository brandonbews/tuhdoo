package main

// Unit tests for tuhdoo top: pure state->render checks plus interaction
// logic driven through Update with a fake steeringAPI — no TTY, no
// daemon (the real daemon round trip lives in top_cli_test.go).

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// topSnapshot seeds every section: one blocking escalation (on an
// unclaimed task, so that task classifies blocked), two ready tasks,
// one in-progress task, one done task.
func topSnapshot() *snapshot {
	raised := time.Date(2026, 7, 29, 14, 3, 0, 0, time.UTC)
	esc := escalationJSON{
		ID: "01E1", Task: "t-lic", Actor: "brandon/a2",
		Question: "Which license?", Blocking: true, RaisedAt: raised,
	}
	return &snapshot{
		state: stateResp{
			Sync: syncJSON{Mode: "local-only"},
			Tasks: []stateTask{
				{ID: "t-parser", Title: "write the parser", Status: "open", Priority: 5},
				{ID: "t-floor", Title: "sweep the floor", Status: "open", Priority: 1},
				{ID: "t-flake", Title: "investigate the flake", Status: "open", Holder: "brandon/a1"},
				{ID: "t-lic", Title: "choose a license", Status: "open"},
				{ID: "t-chore", Title: "old chore", Status: "done"},
			},
			OpenEscalations: []escalationJSON{esc},
		},
		tasks: map[string]hydratedTask{
			"t-parser": {Task: taskJSON{ID: "t-parser", Title: "write the parser"}},
			"t-floor":  {Task: taskJSON{ID: "t-floor", Title: "sweep the floor"}},
			"t-flake":  {Task: taskJSON{ID: "t-flake", Title: "investigate the flake"}},
			"t-lic":    {Task: taskJSON{ID: "t-lic", Title: "choose a license"}, Escalations: []escalationJSON{esc}},
			"t-chore":  {Task: taskJSON{ID: "t-chore", Title: "old chore"}},
		},
	}
}

func newTopModel(api steeringAPI) topModel {
	s := topSnapshot()
	return topModel{api: api, actor: "brandon", snap: s, rows: buildRows(s)}
}

// fakeSteering records calls; err, when set, fails every call.
type fakeSteering struct {
	answers    map[string]string
	priorities map[string]int
	cancelled  []string
	err        error
}

func newFakeSteering() *fakeSteering {
	return &fakeSteering{answers: map[string]string{}, priorities: map[string]int{}}
}

func (f *fakeSteering) answerEscalation(escalation, answer string) error {
	if f.err != nil {
		return f.err
	}
	f.answers[escalation] = answer
	return nil
}

func (f *fakeSteering) setPriority(task string, priority int) error {
	if f.err != nil {
		return f.err
	}
	f.priorities[task] = priority
	return nil
}

func (f *fakeSteering) cancelTask(task string) error {
	if f.err != nil {
		return f.err
	}
	f.cancelled = append(f.cancelled, task)
	return nil
}

// press feeds key messages through Update, returning the final model
// and the last command produced.
func press(t *testing.T, m topModel, keys ...tea.KeyMsg) (topModel, tea.Cmd) {
	t.Helper()
	var cmd tea.Cmd
	for _, k := range keys {
		var mm tea.Model
		mm, cmd = m.Update(k)
		m = mm.(topModel)
	}
	return m, cmd
}

func runes(s string) []tea.KeyMsg {
	var keys []tea.KeyMsg
	for _, r := range s {
		if r == ' ' {
			keys = append(keys, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
			continue
		}
		keys = append(keys, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return keys
}

func keyOf(t tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: t} }

func TestBuildRowsOrderAndSections(t *testing.T) {
	rows := buildRows(topSnapshot())
	want := []struct{ kind, section, id string }{
		{rowEscalation, "escalations", "01E1"},
		{rowTask, "ready", "t-parser"}, // p5 before p1
		{rowTask, "ready", "t-floor"},
		{rowTask, "inprogress", "t-flake"},
		{rowTask, "blocked", "t-lic"},
	}
	if len(rows) != len(want) {
		t.Fatalf("got %d rows, want %d: %+v", len(rows), len(want), rows)
	}
	for i, w := range want {
		if rows[i].kind != w.kind || rows[i].section != w.section || rows[i].id() != w.id {
			t.Errorf("row %d = kind %s section %s id %s; want %+v",
				i, rows[i].kind, rows[i].section, rows[i].id(), w)
		}
	}
}

func TestTopViewRendersSeededState(t *testing.T) {
	m := newTopModel(newFakeSteering())
	v := m.View()
	for _, want := range []string{
		"tuhdoo top", "local-only", "acting as brandon",
		"Open escalations (1)", "Which license?", "[blocking]", "asked by brandon/a2",
		"Ready (2)", "write the parser", "sweep the floor",
		"In progress (1)", "investigate the flake", "brandon/a1",
		"Blocked (1)", "choose a license", "waiting:",
		"▸ Which license?", // cursor starts on the first row
		"j/k move · a answer · p priority · c cancel · q quit",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("view missing %q; view:\n%s", want, v)
		}
	}
	if strings.Contains(v, "old chore") {
		t.Errorf("done task should not render a steerable row; view:\n%s", v)
	}
}

func TestTopNavigationMovesAndClamps(t *testing.T) {
	m := newTopModel(newFakeSteering())
	if m.cursor != 0 {
		t.Fatalf("cursor starts at %d, want 0", m.cursor)
	}
	m, _ = press(t, m, keyOf(tea.KeyUp))
	if m.cursor != 0 {
		t.Errorf("up at top moved cursor to %d", m.cursor)
	}
	for i := 0; i < 10; i++ {
		m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	}
	if want := len(m.rows) - 1; m.cursor != want {
		t.Errorf("j past bottom left cursor at %d, want %d", m.cursor, want)
	}
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if want := len(m.rows) - 2; m.cursor != want {
		t.Errorf("k moved cursor to %d, want %d", m.cursor, want)
	}
	if v := m.View(); !strings.Contains(v, "▸ t-flake") {
		t.Errorf("cursor marker not on t-flake; view:\n%s", v)
	}
}

func TestTopSelectionSurvivesRefresh(t *testing.T) {
	m := newTopModel(newFakeSteering())
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}) // t-parser
	fresh := topSnapshot()
	mm, _ := m.Update(snapMsg{snap: fresh})
	m = mm.(topModel)
	if r, ok := m.selected(); !ok || r.id() != "t-parser" {
		t.Errorf("selection lost across refresh: %+v", r)
	}

	// A refresh that dropped the selected row sends the cursor home.
	fresh = topSnapshot()
	fresh.state.Tasks = fresh.state.Tasks[2:] // parser and floor gone
	mm, _ = m.Update(snapMsg{snap: fresh})
	m = mm.(topModel)
	if m.cursor != 0 {
		t.Errorf("cursor after losing its row = %d, want 0", m.cursor)
	}
}

func TestTopAnswerFlow(t *testing.T) {
	fake := newFakeSteering()
	m := newTopModel(fake)
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if m.mode != modeAnswer {
		t.Fatalf("a on an escalation row: mode %d, want modeAnswer", m.mode)
	}
	if v := m.View(); !strings.Contains(v, "answer") || !strings.Contains(v, "enter submits") {
		t.Errorf("answer prompt missing; view:\n%s", v)
	}
	m, _ = press(t, m, runes("Use MIT.")...)
	if m.input != "Use MIT." {
		t.Fatalf("input = %q, want %q", m.input, "Use MIT.")
	}
	m, _ = press(t, m, keyOf(tea.KeyBackspace))
	if m.input != "Use MIT" {
		t.Fatalf("backspace left %q", m.input)
	}
	m, cmd := press(t, m, keyOf(tea.KeyEnter))
	if m.mode != modeNav {
		t.Errorf("mode after submit = %d, want modeNav", m.mode)
	}
	if cmd == nil {
		t.Fatal("submit produced no command")
	}
	am, ok := cmd().(actionMsg)
	if !ok {
		t.Fatalf("command produced %T, want actionMsg", cmd())
	}
	if am.err != nil {
		t.Fatalf("action error: %v", am.err)
	}
	if got := fake.answers["01E1"]; got != "Use MIT" {
		t.Errorf("answered with %q, want %q", got, "Use MIT")
	}
	mm, _ := m.Update(am)
	m = mm.(topModel)
	if !strings.Contains(m.status, "answered") {
		t.Errorf("status = %q, want an answered confirmation", m.status)
	}
}

func TestTopAnswerRejectsEmptyAndEscCancels(t *testing.T) {
	fake := newFakeSteering()
	m := newTopModel(fake)
	m, cmd := press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}, keyOf(tea.KeyEnter))
	if cmd != nil {
		t.Error("empty answer still submitted")
	}
	if m.mode != modeAnswer || !strings.Contains(m.status, "empty") {
		t.Errorf("empty answer: mode %d status %q", m.mode, m.status)
	}
	m, _ = press(t, m, keyOf(tea.KeyEsc))
	if m.mode != modeNav {
		t.Errorf("esc left mode %d, want modeNav", m.mode)
	}
	if len(fake.answers) != 0 {
		t.Errorf("esc still answered: %v", fake.answers)
	}
}

func TestTopActionKeysRespectRowKind(t *testing.T) {
	m := newTopModel(newFakeSteering())
	// a on a task row: ignored.
	m, _ = press(t, m,
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if m.mode != modeNav {
		t.Errorf("a on a task row entered mode %d", m.mode)
	}
	// p and c on an escalation row: ignored.
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if m.mode != modeNav {
		t.Errorf("p on an escalation row entered mode %d", m.mode)
	}
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if m.mode != modeNav {
		t.Errorf("c on an escalation row entered mode %d", m.mode)
	}
}

func TestTopPriorityFlow(t *testing.T) {
	fake := newFakeSteering()
	m := newTopModel(fake)
	m, _ = press(t, m,
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}, // t-parser
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if m.mode != modePriority {
		t.Fatalf("mode = %d, want modePriority", m.mode)
	}

	// Non-numeric input is rejected and the prompt stays up.
	m, cmd := press(t, m, append(runes("high"), keyOf(tea.KeyEnter))...)
	if cmd != nil || m.mode != modePriority {
		t.Fatalf("bad priority accepted: mode %d cmd %v", m.mode, cmd)
	}
	if !strings.Contains(m.status, "integer") {
		t.Errorf("status = %q, want integer complaint", m.status)
	}
	for range "high" {
		m, _ = press(t, m, keyOf(tea.KeyBackspace))
	}

	m, cmd = press(t, m, append(runes("7"), keyOf(tea.KeyEnter))...)
	if cmd == nil {
		t.Fatal("submit produced no command")
	}
	if am := cmd().(actionMsg); am.err != nil {
		t.Fatalf("action error: %v", am.err)
	}
	if got := fake.priorities["t-parser"]; got != 7 {
		t.Errorf("priority set to %d, want 7", got)
	}
}

func TestTopCancelFlow(t *testing.T) {
	fake := newFakeSteering()
	m := newTopModel(fake)
	m, _ = press(t, m,
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if m.mode != modeConfirmCancel {
		t.Fatalf("mode = %d, want modeConfirmCancel", m.mode)
	}
	if v := m.View(); !strings.Contains(v, "cancel t-parser") || !strings.Contains(v, "y/n") {
		t.Errorf("confirm prompt missing; view:\n%s", v)
	}
	// n backs out without a call.
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if m.mode != modeNav || len(fake.cancelled) != 0 {
		t.Fatalf("n did not back out cleanly: mode %d cancelled %v", m.mode, fake.cancelled)
	}
	// y confirms.
	m, cmd := press(t, m,
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if cmd == nil {
		t.Fatal("confirm produced no command")
	}
	if am := cmd().(actionMsg); am.err != nil {
		t.Fatalf("action error: %v", am.err)
	}
	if len(fake.cancelled) != 1 || fake.cancelled[0] != "t-parser" {
		t.Errorf("cancelled %v, want [t-parser]", fake.cancelled)
	}
}

func TestTopActionErrorSurfacesInStatus(t *testing.T) {
	fake := newFakeSteering()
	fake.err = errors.New("writes rejected (fail-safe read-only)")
	m := newTopModel(fake)
	m, cmd := press(t, m,
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}},
		keyOf(tea.KeyEnter))
	am := cmd().(actionMsg)
	if am.err == nil {
		t.Fatal("expected an action error")
	}
	mm, _ := m.Update(am)
	m = mm.(topModel)
	if !strings.Contains(m.status, "fail-safe") {
		t.Errorf("status = %q, want the daemon error surfaced", m.status)
	}
}

func TestTopQuitKeys(t *testing.T) {
	m := newTopModel(newFakeSteering())
	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune{'q'}},
		{Type: tea.KeyCtrlC},
	} {
		_, cmd := m.Update(key)
		if cmd == nil {
			t.Errorf("key %q should quit in nav mode", key.String())
			continue
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Errorf("key %q produced %T, want tea.QuitMsg", key.String(), cmd())
		}
	}
	// In input mode q is text, ctrl+c still quits.
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if m.mode != modeAnswer || m.input != "q" {
		t.Errorf("q in answer mode: mode %d input %q", m.mode, m.input)
	}
	_, cmd := m.Update(keyOf(tea.KeyCtrlC))
	if cmd == nil {
		t.Fatal("ctrl+c in input mode should quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("ctrl+c in input mode produced %T, want tea.QuitMsg", cmd())
	}
}

func TestTopActor(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    string
		wantErr string
	}{
		{name: "as flag", args: []string{"--as", "brandon"}, want: "brandon"},
		{name: "as equals form", args: []string{"--as=brandon"}, want: "brandon"},
		{name: "agent principal rejected", args: []string{"--as", "brandon/impl-1"}, wantErr: "root principal"},
		{name: "whitespace rejected", args: []string{"--as", "two words"}, wantErr: "invalid actor"},
		{name: "unknown flag", args: []string{"--verbose"}, wantErr: "usage"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := topActor(tt.args)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("actor = %q, want %q", got, tt.want)
			}
		})
	}
}
