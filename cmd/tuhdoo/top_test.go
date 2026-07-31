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
				{ID: "t-pars", Title: "write the parser", Status: "open", Priority: 5},
				{ID: "t-flor", Title: "sweep the floor", Status: "open", Priority: 1},
				{ID: "t-flak", Title: "investigate the flake", Status: "open", Holder: "brandon/a1"},
				{ID: "t-lic", Title: "choose a license", Status: "open"},
				{ID: "t-chor", Title: "old chore", Status: "done"},
			},
			OpenEscalations: []escalationJSON{esc},
		},
		tasks: map[string]hydratedTask{
			"t-pars": {Task: taskJSON{
				ID: "t-pars", Title: "write the parser",
				// Edges: t-chor is done, so t-pars still classifies ready.
				Parents: []string{"t-epic"}, DependsOn: []string{"t-chor"},
			}},
			"t-flor": {Task: taskJSON{ID: "t-flor", Title: "sweep the floor"}},
			"t-flak": {
				Task: taskJSON{
					ID: "t-flak", Title: "investigate the flake",
					Description: "The parser test flakes on CI.\nFind out why.",
					Priority:    3,
				},
				Notes: []noteJSON{{
					ID: "01N1", Task: "t-flak", Actor: "brandon/a1",
					Text:    "Repros only under -race.",
					AddedAt: time.Date(2026, 7, 29, 15, 0, 0, 0, time.UTC),
				}},
				Runs: []runJSON{{
					ID: "01R1", Task: "t-flak", Actor: "brandon/a1",
					Outcome: "interrupted", Summary: "Bisecting the flake.",
				}},
				// Answered out of band and relayed (T5 relay_answer):
				// settled, so it is not in OpenEscalations — it renders
				// only in the task's history, attribution marked.
				Escalations: []escalationJSON{{
					ID: "01E2", Task: "t-flak", Actor: "brandon/a1",
					Question: "Skip the flaky test until fixed?",
					RaisedAt: time.Date(2026, 7, 29, 15, 30, 0, 0, time.UTC),
					Answered: true, Answer: "Skip it, link the issue.",
					AnsweredBy: "brandon", RelayedBy: "brandon/a1",
				}},
			},
			"t-lic":  {Task: taskJSON{ID: "t-lic", Title: "choose a license"}, Escalations: []escalationJSON{esc}},
			"t-chor": {Task: taskJSON{ID: "t-chor", Title: "old chore"}},
		},
	}
}

func newTopModel(api steeringAPI) topModel {
	s := topSnapshot()
	return topModel{api: api, actor: "brandon", armed: true, snap: s, rows: buildRows(s)}
}

// newWatchModel is the same seeded model launched with --watch: disarmed.
func newWatchModel() topModel {
	s := topSnapshot()
	return topModel{snap: s, rows: buildRows(s)}
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
		{rowTask, "ready", "t-pars"}, // p5 before p1
		{rowTask, "ready", "t-flor"},
		{rowTask, "inprogress", "t-flak"},
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
		"tuhdoo · local-only", "acting as brandon",
		"NEEDS INPUT (1)", "Which license?", "blocking · brandon/a2",
		"READY (2)", "write the parser", "sweep the floor",
		"IN PROGRESS (1)", "investigate the flake", "← brandon/a1",
		"BLOCKED (1)", "choose a license", "waiting:",
		"▸ t-lic   !   Which license?", // cursor starts on the first row
		"↑/↓ (j/k) move · enter open · a answer · p priority · c cancel · q quit",
		"1 done", // the footer bar tally replaced the counts line
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
	if v := m.View(); !strings.Contains(v, "▸ t-flak") {
		t.Errorf("cursor marker not on t-flak; view:\n%s", v)
	}
}

// Arrows are the advertised keys; they mirror j/k exactly in both
// cursored contexts — list move and detail scroll.
func TestTopArrowKeysMirrorJK(t *testing.T) {
	m := newTopModel(newFakeSteering())
	m, _ = press(t, m, keyOf(tea.KeyDown), keyOf(tea.KeyDown))
	if m.cursor != 2 {
		t.Errorf("two downs moved cursor to %d, want 2", m.cursor)
	}
	m, _ = press(t, m, keyOf(tea.KeyUp))
	if m.cursor != 1 {
		t.Errorf("up moved cursor to %d, want 1", m.cursor)
	}
}

func TestTopDetailArrowsScroll(t *testing.T) {
	m := newTopModel(newFakeSteering())
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 8})
	m = mm.(topModel)
	m, _ = press(t, m, keyOf(tea.KeyEnter))
	if m.mode != modeDetail {
		t.Fatalf("mode = %d, want modeDetail", m.mode)
	}
	if m.detailMaxScroll() == 0 {
		t.Fatal("detail fits in 8 rows; test needs scrollable content")
	}
	m, _ = press(t, m, keyOf(tea.KeyDown))
	if m.detailScroll != 1 {
		t.Errorf("down scrolled to %d, want 1", m.detailScroll)
	}
	m, _ = press(t, m, keyOf(tea.KeyUp), keyOf(tea.KeyUp))
	if m.detailScroll != 0 {
		t.Errorf("up should clamp at 0, got %d", m.detailScroll)
	}
}

func TestTopSelectionSurvivesRefresh(t *testing.T) {
	m := newTopModel(newFakeSteering())
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}) // t-pars
	fresh := topSnapshot()
	mm, _ := m.Update(snapMsg{snap: fresh})
	m = mm.(topModel)
	if r, ok := m.selected(); !ok || r.id() != "t-pars" {
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
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}, // t-pars
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
	if got := fake.priorities["t-pars"]; got != 7 {
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
	if v := m.View(); !strings.Contains(v, "cancel t-pars") || !strings.Contains(v, "y/n") {
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
	if len(fake.cancelled) != 1 || fake.cancelled[0] != "t-pars" {
		t.Errorf("cancelled %v, want [t-pars]", fake.cancelled)
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

// TestWatchModeDisarmed pins the --watch contract: steering keys are
// dead (successor of watch's zero-input-handling guarantee), the badge
// is visible, and the Blocked section renders (old watch never showed
// it; this is the one deliberate behavior change).
func TestWatchModeDisarmed(t *testing.T) {
	m := newWatchModel()
	for _, r := range []rune{'a', 'p', 'c'} {
		mm, cmd := press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		if mm.mode != modeNav {
			t.Errorf("%q in watch mode entered mode %d", r, mm.mode)
		}
		if cmd != nil {
			t.Errorf("%q in watch mode produced a command", r)
		}
	}
	// Navigation and quit still work.
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.cursor != 1 {
		t.Errorf("j in watch mode left cursor at %d, want 1", m.cursor)
	}
	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}); cmd == nil {
		t.Error("q in watch mode should quit")
	}
	v := m.View()
	for _, want := range []string{"watch mode", "BLOCKED (1)", "waiting:", "↑/↓ (j/k) move · enter open · q quit"} {
		if !strings.Contains(v, want) {
			t.Errorf("watch-mode view missing %q; view:\n%s", want, v)
		}
	}
	for _, reject := range []string{"acting as", "a answer"} {
		if strings.Contains(v, reject) {
			t.Errorf("watch-mode view should not contain %q; view:\n%s", reject, v)
		}
	}
	// The detail screen is read-only, so watch mode keeps it.
	m, _ = press(t, m, keyOf(tea.KeyEnter))
	if m.mode != modeDetail {
		t.Errorf("enter in watch mode: mode %d, want modeDetail", m.mode)
	}
}

// The detail screen: enter opens the row's task in place — for an
// escalation row, the task it hangs off — rendered like `tuhdoo task`.
func TestTopEnterOpensDetail(t *testing.T) {
	m := newTopModel(newFakeSteering())
	// Cursor starts on the escalation row: enter opens its parent task.
	m, _ = press(t, m, keyOf(tea.KeyEnter))
	if m.mode != modeDetail || m.detailID != "t-lic" {
		t.Fatalf("enter on escalation row: mode %d detail %q, want modeDetail t-lic", m.mode, m.detailID)
	}
	m, _ = press(t, m, keyOf(tea.KeyEsc))

	// A task row opens itself, with description and history visible.
	m, _ = press(t, m,
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}, // t-flak
		keyOf(tea.KeyEnter))
	if m.mode != modeDetail || m.detailID != "t-flak" {
		t.Fatalf("enter on task row: mode %d detail %q, want modeDetail t-flak", m.mode, m.detailID)
	}
	v := m.View()
	for _, want := range []string{
		"t-flak — investigate the flake",
		"Description", "The parser test flakes on CI.",
		"History", "note by brandon/a1", "Repros only under -race.",
		"run by brandon/a1", "interrupted", "Bisecting the flake.",
		"Skip the flaky test until fixed?",
		"A (brandon, relayed by brandon/a1): Skip it, link the issue.",
		"↑/↓ (j/k) scroll · esc back · q quit",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("detail view missing %q; view:\n%s", want, v)
		}
	}
	if strings.Contains(v, "enter open") {
		t.Errorf("detail view leaked the list footer; view:\n%s", v)
	}
}

func TestTopDetailEscReturnsAndQQuits(t *testing.T) {
	m := newTopModel(newFakeSteering())
	m, _ = press(t, m,
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
		keyOf(tea.KeyEnter))
	if m.mode != modeDetail {
		t.Fatalf("mode = %d, want modeDetail", m.mode)
	}
	m, cmd := press(t, m, keyOf(tea.KeyEsc))
	if m.mode != modeNav || cmd != nil {
		t.Fatalf("esc from detail: mode %d cmd %v, want modeNav nil", m.mode, cmd)
	}
	if m.cursor != 1 {
		t.Errorf("cursor after esc = %d, want 1 (unchanged)", m.cursor)
	}
	m, cmd = press(t, m, keyOf(tea.KeyEnter), tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("q in detail should quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("q in detail produced %T, want tea.QuitMsg", cmd())
	}
}

// The detail screen renders from the polled snapshot, so a refresh
// that adds a note shows up without leaving the screen.
func TestTopDetailStaysLiveAcrossRefresh(t *testing.T) {
	m := newTopModel(newFakeSteering())
	m, _ = press(t, m,
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
		keyOf(tea.KeyEnter)) // t-flak
	fresh := topSnapshot()
	h := fresh.tasks["t-flak"]
	h.Notes = append(h.Notes, noteJSON{
		ID: "01N2", Task: "t-flak", Actor: "brandon/a1",
		Text:    "Found it: unsynchronized map.",
		AddedAt: time.Date(2026, 7, 29, 16, 0, 0, 0, time.UTC),
	})
	fresh.tasks["t-flak"] = h
	mm, _ := m.Update(snapMsg{snap: fresh})
	m = mm.(topModel)
	if m.mode != modeDetail {
		t.Fatalf("refresh knocked the model out of detail: mode %d", m.mode)
	}
	if v := m.View(); !strings.Contains(v, "unsynchronized map") {
		t.Errorf("refreshed note missing from live detail; view:\n%s", v)
	}
}

// With a known terminal height the body renders a sliding window:
// j walks to the tail and clamps, k walks back to the top.
func TestTopDetailScrollClamps(t *testing.T) {
	m := newTopModel(newFakeSteering())
	m, _ = press(t, m,
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
		keyOf(tea.KeyEnter)) // t-flak: the longest biography
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 8})
	m = mm.(topModel)
	body := m.detailBody()
	if len(body) <= m.detailWindow() {
		t.Fatalf("seed too short to scroll: %d lines, window %d", len(body), m.detailWindow())
	}
	first, last := body[0], body[len(body)-1]
	if v := m.View(); !strings.Contains(v, first) || strings.Contains(v, last) {
		t.Fatalf("unscrolled window wrong; view:\n%s", v)
	}
	for i := 0; i < 100; i++ {
		m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	}
	if want := m.detailMaxScroll(); m.detailScroll != want {
		t.Errorf("j past the end: scroll %d, want clamp at %d", m.detailScroll, want)
	}
	if v := m.View(); !strings.Contains(v, last) || strings.Contains(v, first) {
		t.Errorf("scrolled-to-tail window wrong; view:\n%s", v)
	}
	for i := 0; i < 100; i++ {
		m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	}
	if m.detailScroll != 0 {
		t.Errorf("k past the top: scroll %d, want 0", m.detailScroll)
	}
}

// Edge markers: a row whose task has parents or dependencies says so
// inline; edge-free rows stay clean. Same renderer in both modes.
func TestTopEdgeMarkers(t *testing.T) {
	for name, m := range map[string]topModel{
		"steer": newTopModel(newFakeSteering()),
		"watch": newWatchModel(),
	} {
		v := m.View()
		if !strings.Contains(v, "write the parser  · in t-epic · 1 dep") {
			t.Errorf("%s: parser row missing edge markers; view:\n%s", name, v)
		}
		if strings.Contains(v, "sweep the floor  ·") {
			t.Errorf("%s: edge-free row grew a marker; view:\n%s", name, v)
		}
	}
}

// TestTopViewLoadingAndError ports watch's pre-snapshot and
// daemon-unreachable renderings, which the merged model now owns.
func TestTopViewLoadingAndError(t *testing.T) {
	m := topModel{actor: "brandon", armed: true}
	if v := m.View(); !strings.Contains(v, "loading...") {
		t.Errorf("view before first snapshot missing loading; view:\n%s", v)
	}
	m.err = errors.New("dial unix: no such file")
	v := m.View()
	if !strings.Contains(v, "daemon unreachable") || !strings.Contains(v, "(retrying)") {
		t.Errorf("error view missing unreachable/retrying; view:\n%s", v)
	}
}

func TestTopActor(t *testing.T) {
	tests := []struct {
		name    string
		as      string
		want    string
		wantErr string
	}{
		{name: "explicit override", as: "brandon", want: "brandon"},
		{name: "agent principal rejected", as: "brandon/impl-1", wantErr: "root principal"},
		{name: "whitespace rejected", as: "two words", wantErr: "invalid actor"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := topActor(tt.as)
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

// shortID abbreviates from the tail, where same-batch ULIDs differ;
// already-short toy IDs pass through untouched.
func TestShortID(t *testing.T) {
	tests := []struct{ in, want string }{
		// A real same-batch trio: identical prefixes, distinct tails.
		{"t-01KYT63MB28Z535SMJC9B0D83W", "t-d83w"},
		{"t-01KYT63MB28Z535SMJCA63RQJM", "t-rqjm"},
		{"t-01KYT63MB28Z535SMJCBC7SY1P", "t-sy1p"},
		{"t-lic", "t-lic"},
		{"t-epic", "t-epic"},
		{"01KYT63MB28Z535SMJC9B0D83W", "d83w"}, // no type prefix
	}
	for _, tt := range tests {
		if got := shortID(tt.in); got != tt.want {
			t.Errorf("shortID(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// List rows, waiting: reasons, and edge markers show the short form;
// the full ULID never renders in the list.
func TestTopRowsShowShortIDs(t *testing.T) {
	long := "t-01KYT63MB28Z535SMJC9B0D83W"
	dep := "t-01KYT63MB28Z535SMJCA63RQJM"
	s := &snapshot{
		state: stateResp{Tasks: []stateTask{
			{ID: long, Title: "the long one", Status: "open"},
			{ID: dep, Title: "its dependency", Status: "open"},
		}},
		tasks: map[string]hydratedTask{
			long: {Task: taskJSON{ID: long, Title: "the long one", DependsOn: []string{dep}}},
			dep:  {Task: taskJSON{ID: dep, Title: "its dependency", Parents: []string{long}}},
		},
	}
	m := topModel{armed: true, actor: "brandon", snap: s, rows: buildRows(s)}
	v := m.View()
	for _, want := range []string{
		"t-rqjm  p0  its dependency  · in t-d83w", // ready row + parent marker
		"t-d83w      the long one",                // blocked row: blank badge cell
		// The waiting: reason annotates its dep with status and title.
		"waiting: depends on t-rqjm (open — its dependency)",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("view missing %q; view:\n%s", want, v)
		}
	}
	if strings.Contains(v, long) {
		t.Errorf("full ULID leaked into the list; view:\n%s", v)
	}
	// The detail screen leads with the short form and keeps the full ID
	// exactly once: the dimmed canonical `id` line.
	m, _ = press(t, m, keyOf(tea.KeyEnter)) // cursor on the ready row (dep)
	dv := m.View()
	for _, want := range []string{
		"t-rqjm — its dependency",
		"id          " + dep,
		// Its parent edge is short and annotated too.
		"parents     t-d83w (open — the long one)",
	} {
		if !strings.Contains(dv, want) {
			t.Errorf("detail view missing %q; view:\n%s", want, dv)
		}
	}
	if n := strings.Count(dv, dep); n != 1 {
		t.Errorf("full ULID appears %d times in detail, want exactly 1 (the canonical line); view:\n%s", n, dv)
	}
	if strings.Contains(dv, long) {
		t.Errorf("another task's full ULID leaked into detail; view:\n%s", dv)
	}
}

// taskRef is the edge annotation contract: references that resolve in
// the snapshot carry status and title (done and cancelled tasks are in
// the state listing even though they render no rows — the annotation
// is what proves such an edge isn't dangling); unresolvable IDs render
// bare — never an invented status; long titles are ellipsized.
func TestSnapshotTaskRef(t *testing.T) {
	s := topSnapshot()
	s.state.Tasks = append(s.state.Tasks, stateTask{
		ID:     "t-01KYT63MB28Z535SMJC9B0D83W",
		Title:  strings.Repeat("wide ", 20), // 100 runes, ellipsized at 40
		Status: "cancelled",
	})
	tests := []struct{ id, want string }{
		{"t-lic", "t-lic (open — choose a license)"},
		{"t-chor", "t-chor (done — old chore)"}, // done: no row anywhere, still resolves
		{"t-epic", "t-epic"},                    // unresolvable: bare, no status
		{"t-01KYT63MB28Z535SMJC9B0D83W",
			"t-d83w (cancelled — wide wide wide wide wide wide wide wide…)"},
	}
	for _, tt := range tests {
		if got := s.taskRef(tt.id); got != tt.want {
			t.Errorf("taskRef(%q) = %q, want %q", tt.id, got, tt.want)
		}
	}
}

// The detail screen annotates dep/parent edges from the snapshot; an
// edge to a task the snapshot has never heard of stays bare.
func TestTopDetailAnnotatesEdges(t *testing.T) {
	m := newTopModel(newFakeSteering())
	m, _ = press(t, m,
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}, // t-pars
		keyOf(tea.KeyEnter))
	v := m.View()
	for _, want := range []string{
		"t-pars — write the parser",
		"id          t-pars",
		"parents     t-epic", // unknown to the snapshot: bare
		"depends on  t-chor (done — old chore)",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("detail view missing %q; view:\n%s", want, v)
		}
	}
	if strings.Contains(v, "t-epic (") {
		t.Errorf("unresolvable edge grew an invented annotation; view:\n%s", v)
	}
}

// The one-shot `tuhdoo task` rendering is plumbing and must not grow
// display sugar: full IDs on the title and edge lines, no canonical
// `id` line, no annotations.
func TestPrintTaskOneShotKeepsFullIDs(t *testing.T) {
	long := "t-01KYT63MB28Z535SMJC9B0D83W"
	dep := "t-01KYT63MB28Z535SMJCA63RQJM"
	parent := "t-01KYT63MB28Z535SMJCBC7SY1P"
	var b strings.Builder
	printTask(&b, colors{}, hydratedTask{Task: taskJSON{
		ID: long, Title: "the long one",
		Parents: []string{parent}, DependsOn: []string{dep},
	}})
	v := b.String()
	for _, want := range []string{
		long + " — the long one",
		"parents     " + parent + "\n",
		"depends on  " + dep + "\n",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("one-shot rendering missing %q; output:\n%s", want, v)
		}
	}
	for _, reject := range []string{"t-d83w", "t-rqjm", "t-sy1p", "id          "} {
		if strings.Contains(v, reject) {
			t.Errorf("one-shot rendering grew TUI sugar %q; output:\n%s", reject, v)
		}
	}
}

// resolveTaskID is the input half of the short-ID contract: full IDs
// pass through, short forms and unambiguous fragments resolve,
// ambiguity errors listing every candidate, and nothing is guessed.
func TestResolveTaskID(t *testing.T) {
	long := "t-01KYT63MB28Z535SMJC9B0D83W"
	dep := "t-01KYT63MB28Z535SMJCA63RQJM"
	tasks := []stateTask{
		{ID: long, Title: "the long one"},
		{ID: dep, Title: "its dependency"},
	}
	tests := []struct {
		name, frag, want, wantErr string
	}{
		{name: "full ID", frag: long, want: long},
		{name: "full ID lowercase", frag: strings.ToLower(long), want: long},
		{name: "short form", frag: "t-d83w", want: long},
		{name: "bare tail fragment", frag: "d83w", want: long},
		{name: "uppercase fragment", frag: "63RQJM", want: dep},
		{name: "long unique fragment", frag: "t-01KYT63MB28Z535SMJCA", want: dep},
		{name: "ambiguous", frag: "t-01KYT63MB2", wantErr: "ambiguous"},
		{name: "unknown", frag: "t-nope", wantErr: "unknown task"},
		{name: "empty", frag: "", wantErr: "empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveTaskID(tt.frag, tasks)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
				if tt.wantErr == "ambiguous" {
					for _, cand := range []string{"t-d83w", "t-rqjm", long, dep} {
						if !strings.Contains(err.Error(), cand) {
							t.Errorf("ambiguity error missing candidate %q: %v", cand, err)
						}
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("resolveTaskID(%q) = %q, want %q", tt.frag, got, tt.want)
			}
		})
	}
}

// maxLineWidth is the widest rendered line in runes (colors are empty
// in tests, so rune count is display width for this ASCII-ish output).
func maxLineWidth(v string) int {
	max := 0
	for _, l := range strings.Split(v, "\n") {
		if n := len([]rune(l)); n > max {
			max = n
		}
	}
	return max
}

// With a known terminal size, no list line exceeds the width, the
// whole frame fits the height, and the cursor row stays visible even
// at the bottom of a list taller than the screen.
func TestTopListWrapsAndScrolls(t *testing.T) {
	m := newTopModel(newFakeSteering())
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 30, Height: 12})
	m = mm.(topModel)
	v := m.View()
	if w := maxLineWidth(v); w > 30 {
		t.Errorf("line wider than terminal: %d > 30; view:\n%s", w, v)
	}
	if n := strings.Count(strings.TrimRight(v, "\n"), "\n") + 1; n > 12 {
		t.Errorf("frame taller than terminal: %d > 12 lines; view:\n%s", n, v)
	}
	if !strings.Contains(v, "▸") {
		t.Errorf("cursor row not visible at top; view:\n%s", v)
	}
	// Walk to the last row: the window must follow the cursor there.
	for i := 0; i < 10; i++ {
		m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	}
	v = m.View()
	if !strings.Contains(v, "▸") || !strings.Contains(v, "choose a license") {
		t.Errorf("cursor row (blocked t-lic) not visible after scrolling; view:\n%s", v)
	}
	if strings.Contains(v, "Which license?") {
		t.Errorf("top of list should have scrolled off; view:\n%s", v)
	}
	if n := strings.Count(strings.TrimRight(v, "\n"), "\n") + 1; n > 12 {
		t.Errorf("scrolled frame taller than terminal: %d > 12 lines; view:\n%s", n, v)
	}
}

// The detail body wraps to the width before the scroll window slices
// it, so narrow terminals scroll real screen lines.
func TestTopDetailWrapsToWidth(t *testing.T) {
	m := newTopModel(newFakeSteering())
	m, _ = press(t, m,
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
		keyOf(tea.KeyEnter)) // t-flak
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 24, Height: 10})
	m = mm.(topModel)
	if w := maxLineWidth(m.View()); w > 24 {
		t.Errorf("detail line wider than terminal: %d > 24; view:\n%s", w, m.View())
	}
	if m.detailMaxScroll() == 0 {
		t.Error("wrapped body should be scrollable at 24x10")
	}
	for i := 0; i < 100; i++ {
		m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	}
	if v := m.View(); !strings.Contains(v, "Bisecting the flake.") {
		t.Errorf("tail not reachable by scrolling; view:\n%s", v)
	}
}
