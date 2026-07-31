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
// one in-progress task, one held task, one inbox capture, one done
// task.
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
				{ID: "t-park", Title: "polish the docs", Status: "held", Priority: 2},
				{ID: "t-idea", Title: "idea: dark mode", Status: "inbox"},
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
			"t-park": {Task: taskJSON{ID: "t-park", Title: "polish the docs", Priority: 2}},
			"t-idea": {Task: taskJSON{ID: "t-idea", Title: "idea: dark mode"}},
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
	archived   []string
	captured   []string // quick-capture titles, in order
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

func (f *fakeSteering) archiveTask(task string) error {
	if f.err != nil {
		return f.err
	}
	f.archived = append(f.archived, task)
	return nil
}

func (f *fakeSteering) captureTask(title string) error {
	if f.err != nil {
		return f.err
	}
	f.captured = append(f.captured, title)
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
		{rowTask, "held", "t-park"},  // the shelves close the list,
		{rowTask, "inbox", "t-idea"}, // held above inbox (2026-07-31)
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
		"BLOCKED (1)", "choose a license", "waiting: needs input (above)",
		"▸ t-lic   !   Which license?", // cursor starts on the first row
		"↑/↓ (j/k) move · enter answer/open · p priority · c archive · q quit",
		"1 done", // the footer bar tally replaced the counts line
	} {
		if !strings.Contains(v, want) {
			t.Errorf("view missing %q; view:\n%s", want, v)
		}
	}
	if strings.Contains(v, "old chore") {
		t.Errorf("done task should not render a steerable row; view:\n%s", v)
	}
	// The question renders once — its Needs Input row — never repeated
	// in the blocked row's waiting: reason (steering feedback,
	// 2026-07-30).
	if n := strings.Count(v, "Which license?"); n != 1 {
		t.Errorf("question renders %d times, want exactly 1; view:\n%s", n, v)
	}
	if strings.Contains(v, "waiting: escalation:") {
		t.Errorf("blocked row still repeats the escalation; view:\n%s", v)
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
	// len-2 is the held row: the shelves are ordinary cursor stops.
	if v := m.View(); !strings.Contains(v, "▸ t-park") {
		t.Errorf("cursor marker not on t-park; view:\n%s", v)
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
	m, _ = press(t, m,
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}, // t-flak: the longest biography
		keyOf(tea.KeyEnter))
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
	// Enter on a Needs Input row goes straight into answering — no
	// detail detour, no separate answer key.
	m, _ = press(t, m, keyOf(tea.KeyEnter))
	if m.mode != modeAnswer {
		t.Fatalf("enter on an escalation row: mode %d, want modeAnswer", m.mode)
	}
	v := m.View()
	if !strings.Contains(v, "answer") || !strings.Contains(v, "Which license?") ||
		!strings.Contains(v, "enter submits · esc cancels") {
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
	m, cmd := press(t, m, keyOf(tea.KeyEnter), keyOf(tea.KeyEnter))
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
	// a died when enter became the answer key (Cycle-4 rule: no
	// vestigial keys) — it does nothing anywhere, escalation row included.
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if m.mode != modeNav {
		t.Errorf("removed key a still entered mode %d", m.mode)
	}
	// p and c on an escalation row: ignored.
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
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

func TestTopArchiveFlow(t *testing.T) {
	fake := newFakeSteering()
	m := newTopModel(fake)
	m, _ = press(t, m,
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if m.mode != modeConfirmArchive {
		t.Fatalf("mode = %d, want modeConfirmArchive", m.mode)
	}
	v := m.View()
	if !strings.Contains(v, "archive t-pars") || !strings.Contains(v, "y/n") ||
		!strings.Contains(v, "history stays on the ledger") {
		t.Errorf("confirm prompt missing; view:\n%s", v)
	}
	// The human verb never says cancel — that word belongs to esc.
	if strings.Contains(v, "cancel t-pars") {
		t.Errorf("confirm prompt still says cancel; view:\n%s", v)
	}
	// n backs out without a call.
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if m.mode != modeNav || len(fake.archived) != 0 {
		t.Fatalf("n did not back out cleanly: mode %d archived %v", m.mode, fake.archived)
	}
	// y confirms.
	m, cmd := press(t, m,
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if cmd == nil {
		t.Fatal("confirm produced no command")
	}
	am := cmd().(actionMsg)
	if am.err != nil {
		t.Fatalf("action error: %v", am.err)
	}
	if len(fake.archived) != 1 || fake.archived[0] != "t-pars" {
		t.Errorf("archived %v, want [t-pars]", fake.archived)
	}
	// The confirmation names the human verb.
	mm, _ := m.Update(am)
	m = mm.(topModel)
	if m.status != "archived t-pars" {
		t.Errorf("status = %q, want %q", m.status, "archived t-pars")
	}
}

func TestTopActionErrorSurfacesInStatus(t *testing.T) {
	fake := newFakeSteering()
	fake.err = errors.New("writes rejected (fail-safe read-only)")
	m := newTopModel(fake)
	m, cmd := press(t, m,
		keyOf(tea.KeyEnter),
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
	// In input mode q is text, ctrl+c still quits. (Enter on the
	// escalation row the cursor starts on opens answer input.)
	m, _ = press(t, m, keyOf(tea.KeyEnter),
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
	for _, r := range []rune{'a', 'p', 'c', 'i'} {
		mm, cmd := press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		if mm.mode != modeNav {
			t.Errorf("%q in watch mode entered mode %d", r, mm.mode)
		}
		if cmd != nil {
			t.Errorf("%q in watch mode produced a command", r)
		}
	}
	// Enter on the Needs Input row never opens answer input in a
	// disarmed pane: it falls through to the read-only detail of the
	// escalation's task, and esc steps back.
	m, cmd := press(t, m, keyOf(tea.KeyEnter))
	if m.mode != modeDetail || m.detailID != "t-lic" {
		t.Fatalf("enter on escalation row in watch mode: mode %d detail %q, want modeDetail t-lic", m.mode, m.detailID)
	}
	if cmd != nil {
		t.Error("enter on escalation row in watch mode produced a command")
	}
	m, _ = press(t, m, keyOf(tea.KeyEsc))
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
	for _, reject := range []string{"acting as", "enter answer"} {
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

// The detail screen: enter on a task row opens the task in place,
// rendered like `tuhdoo task`. (On an armed escalation row enter
// answers instead — TestTopAnswerFlow; watch mode's escalation-row
// detail lives in TestWatchModeDisarmed.)
func TestTopEnterOpensDetail(t *testing.T) {
	m := newTopModel(newFakeSteering())
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
		// Armed with no open escalation: p/c advertised, no enter.
		"↑/↓ (j/k) scroll · p priority · c archive · esc back · q quit",
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
		// Post-rebrand IDs shorten with their own tuh- prefix; the two
		// eras coexist side by side (T7, 2026-07-31).
		{"tuh-01KYT63MB28Z535SMJC9B0D83W", "tuh-d83w"},
		{"tuh-01KYT63MB28Z535SMJCA63RQJM", "tuh-rqjm"},
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
// bare — never an invented status; long titles are ellipsized. The
// status word is the human-facing one: the plumbing value "cancelled"
// renders as "archived" (T7, 2026-07-31).
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
		// The shelves annotate with their own words (2026-07-31): a dep
		// waiting on an inbox/held task says exactly why it waits.
		{"t-park", "t-park (on hold — polish the docs)"},
		{"t-idea", "t-idea (inbox — idea: dark mode)"},
		{"t-01KYT63MB28Z535SMJC9B0D83W",
			"t-d83w (archived — wide wide wide wide wide wide wide wide…)"},
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

// The cross-prefix rule for the tuh-/t- era split (T7, 2026-07-31): a
// fragment's prefix is matched literally against the ID, so t-d83w and
// tuh-d83w name distinct tasks even with identical tails; a bare
// fragment matches both eras and turns ambiguous when the tails
// collide. Neither prefix is ever rejected.
func TestResolveTaskIDCrossPrefix(t *testing.T) {
	oldID := "t-01KYT63MB28Z535SMJC9B0D83W"   // pre-rebrand era
	newID := "tuh-01KYT63MB28Z535SMJC9B0D83W" // same tail, tuh- era
	other := "tuh-01KYT63MB28Z535SMJCA63RQJM"
	tasks := []stateTask{
		{ID: oldID, Title: "the old-era one"},
		{ID: newID, Title: "the new-era twin"},
		{ID: other, Title: "the new-era loner"},
	}
	tests := []struct {
		name, frag, want, wantErr string
	}{
		{name: "old full ID", frag: oldID, want: oldID},
		{name: "new full ID", frag: newID, want: newID},
		{name: "old short form stays in its era", frag: "t-d83w", want: oldID},
		{name: "new short form stays in its era", frag: "tuh-d83w", want: newID},
		{name: "new short form, unique tail", frag: "tuh-rqjm", want: other},
		{name: "bare tail spans both eras", frag: "d83w", wantErr: "ambiguous"},
		{name: "bare unique tail resolves", frag: "rqjm", want: other},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveTaskID(tt.frag, tasks)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
				// Both eras' twins are listed as candidates.
				for _, cand := range []string{oldID, newID} {
					if !strings.Contains(err.Error(), cand) {
						t.Errorf("ambiguity error missing candidate %q: %v", cand, err)
					}
				}
				if strings.Contains(err.Error(), other) {
					t.Errorf("ambiguity error lists non-matching task: %v", err)
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

// ---- the armed detail screen steers the viewed task (task t-…63RQJM) ----

// openDetail walks the cursor to a task's row and opens its detail.
func openDetail(t *testing.T, m topModel, id string) topModel {
	t.Helper()
	m = moveTo(t, m, id)
	m, _ = press(t, m, keyOf(tea.KeyEnter))
	if m.mode != modeDetail || m.detailID != id {
		t.Fatalf("enter did not open detail of %s: mode %d detail %q", id, m.mode, m.detailID)
	}
	return m
}

// Armed detail of a task with an open escalation: the escalation is the
// focused item, enter answers it in place — same API call and refresh
// as answering from the list — and the flow returns to the detail.
func TestTopDetailAnswerFlow(t *testing.T) {
	fake := newFakeSteering()
	m := openDetail(t, newTopModel(fake), "t-lic")
	v := m.View()
	if !strings.Contains(v, "▸ unanswered — enter to answer") {
		t.Fatalf("open escalation not rendered as the focused item; view:\n%s", v)
	}
	if !strings.Contains(v, "↑/↓ (j/k) move · enter answer · p priority · c archive · esc back · q quit") {
		t.Errorf("armed detail footer wrong; view:\n%s", v)
	}
	m, _ = press(t, m, keyOf(tea.KeyEnter))
	if m.mode != modeAnswer {
		t.Fatalf("enter on the focused escalation: mode %d, want modeAnswer", m.mode)
	}
	// Answering targets the escalation belonging to the viewed task, and
	// the detail stays on screen with the standard prompt as its footer.
	if m.target.esc.ID != "01E1" {
		t.Fatalf("answer targets %q, want 01E1 (the viewed task's escalation)", m.target.esc.ID)
	}
	v = m.View()
	for _, want := range []string{
		"t-lic — choose a license", // still the detail frame
		"Which license?", "enter submits · esc cancels",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("detail answer prompt missing %q; view:\n%s", want, v)
		}
	}
	m, cmd := press(t, m, append(runes("Use MIT."), keyOf(tea.KeyEnter))...)
	if m.mode != modeDetail || m.detailID != "t-lic" {
		t.Errorf("submit did not return to detail: mode %d detail %q", m.mode, m.detailID)
	}
	if cmd == nil {
		t.Fatal("submit produced no command")
	}
	am := cmd().(actionMsg)
	if am.err != nil {
		t.Fatalf("action error: %v", am.err)
	}
	if got := fake.answers["01E1"]; got != "Use MIT." {
		t.Errorf("answered with %q, want %q", got, "Use MIT.")
	}
	mm, _ := m.Update(am)
	m = mm.(topModel)
	if v := m.View(); !strings.Contains(v, "answered") {
		t.Errorf("detail view does not surface the action status; view:\n%s", v)
	}

	// The refresh that lands the answer removes the focusable item:
	// marker gone, enter dead, footer back to scroll/p/c.
	fresh := topSnapshot()
	h := fresh.tasks["t-lic"]
	e := h.Escalations[0]
	e.Answered, e.Answer, e.AnsweredBy = true, "Use MIT.", "brandon"
	h.Escalations = []escalationJSON{e}
	fresh.tasks["t-lic"] = h
	fresh.state.OpenEscalations = nil
	mm, _ = m.Update(snapMsg{snap: fresh})
	m = mm.(topModel)
	v = m.View()
	if strings.Contains(v, "▸ unanswered") {
		t.Errorf("answered escalation still renders focused; view:\n%s", v)
	}
	if !strings.Contains(v, "↑/↓ (j/k) scroll · p priority · c archive · esc back · q quit") {
		t.Errorf("footer still advertises enter answer; view:\n%s", v)
	}
	m, cmd = press(t, m, keyOf(tea.KeyEnter))
	if m.mode != modeDetail || cmd != nil {
		t.Errorf("enter with nothing focusable: mode %d cmd %v, want modeDetail nil", m.mode, cmd)
	}
}

// esc from an input mode opened in detail returns to that detail, not
// the list; esc again reaches the list.
func TestTopDetailInputEscReturnsToDetail(t *testing.T) {
	fake := newFakeSteering()
	m := openDetail(t, newTopModel(fake), "t-lic")
	m, _ = press(t, m, keyOf(tea.KeyEnter), keyOf(tea.KeyEsc))
	if m.mode != modeDetail || m.detailID != "t-lic" {
		t.Fatalf("esc from detail answer: mode %d detail %q, want modeDetail t-lic", m.mode, m.detailID)
	}
	if len(fake.answers) != 0 {
		t.Errorf("esc still answered: %v", fake.answers)
	}
	m, _ = press(t, m, keyOf(tea.KeyEsc))
	if m.mode != modeNav {
		t.Errorf("esc from detail: mode %d, want modeNav", m.mode)
	}
	// Back on the list, input modes return to the list again.
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}) // cursor still on t-lic's task row
	if m.mode != modePriority {
		t.Fatalf("p on the blocked task row: mode %d, want modePriority", m.mode)
	}
	m, _ = press(t, m, keyOf(tea.KeyEsc))
	if m.mode != modeNav {
		t.Errorf("esc from a list-opened prompt: mode %d, want modeNav", m.mode)
	}
}

// p in an armed detail reprioritizes the viewed task with the same
// prompt as the list, returning to the detail after submit.
func TestTopDetailPriorityFlow(t *testing.T) {
	fake := newFakeSteering()
	m := openDetail(t, newTopModel(fake), "t-lic")
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if m.mode != modePriority {
		t.Fatalf("p in detail: mode %d, want modePriority", m.mode)
	}
	if v := m.View(); !strings.Contains(v, "priority t-lic (choose a license)") {
		t.Errorf("priority prompt does not name the viewed task; view:\n%s", v)
	}
	m, cmd := press(t, m, append(runes("4"), keyOf(tea.KeyEnter))...)
	if m.mode != modeDetail || m.detailID != "t-lic" {
		t.Errorf("submit did not return to detail: mode %d detail %q", m.mode, m.detailID)
	}
	if cmd == nil {
		t.Fatal("submit produced no command")
	}
	if am := cmd().(actionMsg); am.err != nil {
		t.Fatalf("action error: %v", am.err)
	}
	if got := fake.priorities["t-lic"]; got != 4 {
		t.Errorf("priority set to %d on the viewed task, want 4", got)
	}
}

// c in an armed detail archives the viewed task behind the same y/n
// confirm as the list; the detail survives the archive, showing the
// task's new archived status once the refresh lands.
func TestTopDetailArchiveFlow(t *testing.T) {
	fake := newFakeSteering()
	m := openDetail(t, newTopModel(fake), "t-lic")
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if m.mode != modeConfirmArchive {
		t.Fatalf("c in detail: mode %d, want modeConfirmArchive", m.mode)
	}
	if v := m.View(); !strings.Contains(v, "archive t-lic (choose a license)? y/n") {
		t.Errorf("confirm prompt does not name the viewed task; view:\n%s", v)
	}
	// n backs out to the detail without a call.
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if m.mode != modeDetail || len(fake.archived) != 0 {
		t.Fatalf("n did not back out to detail: mode %d archived %v", m.mode, fake.archived)
	}
	// y confirms.
	m, cmd := press(t, m,
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if cmd == nil {
		t.Fatal("confirm produced no command")
	}
	am := cmd().(actionMsg)
	if am.err != nil {
		t.Fatalf("action error: %v", am.err)
	}
	if len(fake.archived) != 1 || fake.archived[0] != "t-lic" {
		t.Errorf("archived %v, want [t-lic]", fake.archived)
	}
	mm, _ := m.Update(am)
	m = mm.(topModel)
	if m.mode != modeDetail {
		t.Fatalf("archive knocked the model out of detail: mode %d", m.mode)
	}
	if v := m.View(); !strings.Contains(v, "archived t-lic") {
		t.Errorf("archive status not surfaced in detail; view:\n%s", v)
	}
	// The refresh lands the archive: the task stays viewable — history
	// stays on the ledger — with the human-facing status word.
	fresh := topSnapshot()
	for i := range fresh.state.Tasks {
		if fresh.state.Tasks[i].ID == "t-lic" {
			fresh.state.Tasks[i].Status = "cancelled"
		}
	}
	h := fresh.tasks["t-lic"]
	h.Task.Status = "cancelled"
	fresh.tasks["t-lic"] = h
	mm, _ = m.Update(snapMsg{snap: fresh})
	m = mm.(topModel)
	if m.mode != modeDetail || m.detailID != "t-lic" {
		t.Fatalf("refresh after archive left mode %d detail %q", m.mode, m.detailID)
	}
	if v := m.View(); !strings.Contains(v, "status      archived") {
		t.Errorf("archived task's detail missing its new status; view:\n%s", v)
	}
}

// multiEscSnapshot seeds one task with two open escalations and a long
// description, for the focus-vs-scroll rule.
func multiEscSnapshot() *snapshot {
	raised := time.Date(2026, 7, 30, 9, 0, 0, 0, time.UTC)
	e1 := escalationJSON{ID: "01E1", Task: "t-two", Actor: "brandon/a1",
		Question: "First question?", Blocking: true, RaisedAt: raised}
	e2 := escalationJSON{ID: "01E2", Task: "t-two", Actor: "brandon/a1",
		Question: "Second question?", RaisedAt: raised}
	return &snapshot{
		state: stateResp{
			Tasks:           []stateTask{{ID: "t-two", Title: "twice escalated", Status: "open"}},
			OpenEscalations: []escalationJSON{e1, e2},
		},
		tasks: map[string]hydratedTask{
			"t-two": {
				Task: taskJSON{ID: "t-two", Title: "twice escalated",
					Description: strings.Repeat("A line of description.\n", 20)},
				Escalations: []escalationJSON{e1, e2},
			},
		},
	}
}

// The focus/scroll rule: j/k move focus when a further open escalation
// exists in that direction — scrolling just enough to reveal it — and
// scroll one line otherwise.
func TestTopDetailFocusMovesAmongEscalations(t *testing.T) {
	s := multiEscSnapshot()
	m := topModel{api: newFakeSteering(), actor: "brandon", armed: true, snap: s, rows: buildRows(s)}
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
	m = mm.(topModel)
	m = openDetail(t, m, "t-two")
	if m.detailFocus != 0 {
		t.Fatalf("focus starts at %d, want 0", m.detailFocus)
	}
	if n := strings.Count(strings.Join(m.detailBody(), "\n"), "▸ unanswered"); n != 1 {
		t.Fatalf("want exactly one focused marker, got %d", n)
	}
	inWindow := func(m topModel) bool {
		l := m.detailFocusLine()
		return l >= m.detailScroll && l < m.detailScroll+m.detailWindow()
	}
	// j moves focus to the second escalation and reveals it.
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.detailFocus != 1 {
		t.Fatalf("j moved focus to %d, want 1", m.detailFocus)
	}
	if !inWindow(m) {
		t.Errorf("focused marker not revealed: line %d scroll %d window %d",
			m.detailFocusLine(), m.detailScroll, m.detailWindow())
	}
	// enter answers the focused (second) escalation.
	m, _ = press(t, m, keyOf(tea.KeyEnter))
	if m.mode != modeAnswer || m.target.esc.ID != "01E2" {
		t.Fatalf("enter on second focus: mode %d target %q, want modeAnswer 01E2", m.mode, m.target.esc.ID)
	}
	m, _ = press(t, m, keyOf(tea.KeyEsc))
	if m.mode != modeDetail || m.detailFocus != 1 {
		t.Fatalf("esc lost the detail focus: mode %d focus %d", m.mode, m.detailFocus)
	}
	// k moves focus back up; k again (no item further up) scrolls a line.
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.detailFocus != 0 {
		t.Fatalf("k moved focus to %d, want 0", m.detailFocus)
	}
	if !inWindow(m) {
		t.Errorf("first marker not revealed after k: line %d scroll %d", m.detailFocusLine(), m.detailScroll)
	}
	before := m.detailScroll
	if before == 0 {
		t.Fatal("test needs a scrolled-down window to exercise the k fallback")
	}
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.detailFocus != 0 || m.detailScroll != before-1 {
		t.Errorf("k at first focus: focus %d scroll %d, want 0 %d (line-scroll fallback)",
			m.detailFocus, m.detailScroll, before-1)
	}
}

// detailFocusIdx clamps like detailScroll: content shrinking under a
// refresh pulls the focus back onto a real item, or to none at all.
func TestDetailFocusIdx(t *testing.T) {
	tests := []struct{ focus, n, want int }{
		{0, 0, -1}, // nothing focusable
		{3, 0, -1},
		{0, 2, 0},
		{1, 2, 1},
		{5, 2, 1}, // shrunk under refresh: clamp to the last
	}
	for _, tt := range tests {
		if got := detailFocusIdx(tt.focus, tt.n); got != tt.want {
			t.Errorf("detailFocusIdx(%d, %d) = %d, want %d", tt.focus, tt.n, got, tt.want)
		}
	}
}

// Watch mode's detail is fully disarmed: no focusable affordances, no
// input — enter, p, and c are dead; j/k only scroll; the footer stays
// read-only.
func TestWatchModeDetailFullyDisarmed(t *testing.T) {
	m := newWatchModel()
	m, _ = press(t, m, keyOf(tea.KeyEnter)) // escalation row → read-only detail of t-lic
	if m.mode != modeDetail || m.detailID != "t-lic" {
		t.Fatalf("mode %d detail %q, want modeDetail t-lic", m.mode, m.detailID)
	}
	v := m.View()
	if strings.Contains(v, "▸ unanswered") {
		t.Errorf("watch detail renders a focusable item; view:\n%s", v)
	}
	if !strings.Contains(v, "    unanswered") {
		t.Errorf("open escalation missing from watch detail history; view:\n%s", v)
	}
	if !strings.Contains(v, "↑/↓ (j/k) scroll · esc back · q quit") {
		t.Errorf("watch detail footer not read-only; view:\n%s", v)
	}
	for _, reject := range []string{"enter answer", "p priority", "c archive"} {
		if strings.Contains(v, reject) {
			t.Errorf("watch detail advertises %q; view:\n%s", reject, v)
		}
	}
	for _, k := range []tea.KeyMsg{
		keyOf(tea.KeyEnter),
		{Type: tea.KeyRunes, Runes: []rune{'p'}},
		{Type: tea.KeyRunes, Runes: []rune{'c'}},
		{Type: tea.KeyRunes, Runes: []rune{'a'}}, // the removed key stays removed here too
	} {
		mm, cmd := press(t, m, k)
		if mm.mode != modeDetail || cmd != nil {
			t.Errorf("%q in watch detail: mode %d cmd %v, want modeDetail nil", k.String(), mm.mode, cmd)
		}
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

// ---- mouse (task t-…HJEV9VK): click selects, click again acts as enter ----

// clickAt is a left-button press at screen position (x, y).
func clickAt(x, y int) tea.MouseMsg {
	return tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}
}

func wheelMsg(b tea.MouseButton) tea.MouseMsg {
	return tea.MouseMsg{Button: b, Action: tea.MouseActionPress}
}

// mouseTo feeds mouse messages through Update.
func mouseTo(t *testing.T, m topModel, msgs ...tea.MouseMsg) (topModel, tea.Cmd) {
	t.Helper()
	var cmd tea.Cmd
	for _, msg := range msgs {
		var mm tea.Model
		mm, cmd = m.Update(msg)
		m = mm.(topModel)
	}
	return m, cmd
}

// screenLineOf finds the 0-based screen row of the first rendered line
// containing sub — clicks in these tests aim at what is actually drawn,
// never at hand-counted coordinates.
func screenLineOf(t *testing.T, m topModel, sub string) int {
	t.Helper()
	for i, l := range strings.Split(m.View(), "\n") {
		if strings.Contains(l, sub) {
			return i
		}
	}
	t.Fatalf("no rendered line contains %q; view:\n%s", sub, m.View())
	return -1
}

// A single click moves the cursor to the row under the pointer, across
// the variable-height layout: one-line ready rows, two-line escalation
// and blocked rows (either line hits), and chrome (bars, blanks, the
// header, the footer) hitting nothing.
func TestTopClickSelectsAcrossVariableHeights(t *testing.T) {
	m := newTopModel(newFakeSteering())
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	m = mm.(topModel)
	tests := []struct {
		name, aim string
		want      int // cursor after the click; -1 = unchanged
	}{
		{"one-line ready row", "sweep the floor", 2},
		{"in-progress row", "investigate the flake", 3},
		{"blocked row first line", "choose a license", 4},
		{"blocked row second line", "waiting: needs input (above)", 4},
		{"escalation second line", "blocking · brandon/a2", 0},
		{"section bar", "READY (2)", -1},
		{"header bar", "acting as brandon", -1},
		{"footer bar", "1 done", -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := m
			m.cursor = 1 // t-pars: nothing under test starts selected
			m, cmd := mouseTo(t, m, clickAt(0, screenLineOf(t, m, tt.aim)))
			want := tt.want
			if want == -1 {
				want = 1
			}
			if m.cursor != want || m.mode != modeNav || cmd != nil {
				t.Errorf("click on %s: cursor %d mode %d cmd %v, want cursor %d modeNav nil",
					tt.name, m.cursor, m.mode, cmd, want)
			}
		})
	}
	// A click past the rendered frame hits nothing.
	m.cursor = 1
	if m, _ := mouseTo(t, m, clickAt(0, 39)); m.cursor != 1 || m.mode != modeNav {
		t.Errorf("click below the list: cursor %d mode %d, want 1 modeNav", m.cursor, m.mode)
	}
}

// Click on the already-selected row acts as enter — and a double-click
// is exactly that: press one selects, press two finds it selected. On
// an armed escalation row that means answering; on a task row, detail.
func TestTopClickOnSelectedActsAsEnter(t *testing.T) {
	m := newTopModel(newFakeSteering())
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	m = mm.(topModel)
	// Double-click a task row: first press selects, second opens detail.
	y := screenLineOf(t, m, "write the parser")
	m, _ = mouseTo(t, m, clickAt(0, y))
	if m.cursor != 1 || m.mode != modeNav {
		t.Fatalf("first click: cursor %d mode %d, want 1 modeNav", m.cursor, m.mode)
	}
	m, _ = mouseTo(t, m, clickAt(0, y))
	if m.mode != modeDetail || m.detailID != "t-pars" {
		t.Fatalf("second click: mode %d detail %q, want modeDetail t-pars", m.mode, m.detailID)
	}
	m, _ = press(t, m, keyOf(tea.KeyEsc))
	// Click the escalation row (cursor starts there after esc restores
	// nav on row 1 — walk back first).
	m, _ = press(t, m, keyOf(tea.KeyUp))
	m, _ = mouseTo(t, m, clickAt(0, screenLineOf(t, m, "Which license?")))
	if m.mode != modeAnswer || m.target.esc.ID != "01E1" {
		t.Fatalf("click on selected escalation row: mode %d target %q, want modeAnswer 01E1",
			m.mode, m.target.esc.ID)
	}
}

// Hit-testing replays the cursor-following window: with the list taller
// than the terminal and the view scrolled to the bottom, clicks land on
// the rows actually drawn at those screen lines, not the unscrolled
// layout.
func TestTopClickHitsScrolledRows(t *testing.T) {
	m := newTopModel(newFakeSteering())
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 8})
	m = mm.(topModel)
	for i := 0; i < 4; i++ {
		m, _ = press(t, m, keyOf(tea.KeyDown)) // to t-lic; top scrolls off
	}
	if v := m.View(); strings.Contains(v, "Which license?") {
		t.Fatalf("test needs the top scrolled off; view:\n%s", v)
	}
	// The blocked row's second line, at its scrolled screen position,
	// still resolves to the selected row — so the click acts as enter.
	m2, _ := mouseTo(t, m, clickAt(0, screenLineOf(t, m, "waiting: needs input (above)")))
	if m2.mode != modeDetail || m2.detailID != "t-lic" {
		t.Errorf("click on scrolled selected row: mode %d detail %q, want modeDetail t-lic",
			m2.mode, m2.detailID)
	}
	// The BLOCKED bar above it stays chrome.
	m3, _ := mouseTo(t, m, clickAt(0, screenLineOf(t, m, "BLOCKED (1)")))
	if m3.mode != modeNav || m3.cursor != 4 {
		t.Errorf("click on scrolled bar: mode %d cursor %d, want modeNav 4", m3.mode, m3.cursor)
	}
}

// Watch mode: a click may select and (on the selected row) open the
// read-only detail — exactly enter's watch behavior — but never any
// input mode, escalation rows included.
func TestWatchModeClickNeverOpensInput(t *testing.T) {
	m := newWatchModel()
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	m = mm.(topModel)
	y := screenLineOf(t, m, "Which license?")
	m, cmd := mouseTo(t, m, clickAt(0, y)) // cursor starts on the escalation row
	if m.mode != modeDetail || m.detailID != "t-lic" || cmd != nil {
		t.Fatalf("click on selected escalation in watch mode: mode %d detail %q cmd %v, want read-only detail of t-lic",
			m.mode, m.detailID, cmd)
	}
	m, _ = press(t, m, keyOf(tea.KeyEsc))
	// Clicking every rendered line, twice each, never yields an input mode.
	for y := 0; y < 40; y++ {
		m, _ = mouseTo(t, m, clickAt(0, y), clickAt(0, y))
		if m.mode == modeAnswer || m.mode == modePriority || m.mode == modeConfirmArchive {
			t.Fatalf("double-click at line %d in watch mode opened input mode %d", y, m.mode)
		}
		m, _ = press(t, m, keyOf(tea.KeyEsc)) // step back out of any detail
	}
}

// The wheel scrolls: in the list it moves the cursor (the window
// follows it), in the detail it moves the line window; both clamp.
func TestTopWheelScrolls(t *testing.T) {
	m := newTopModel(newFakeSteering())
	m, _ = mouseTo(t, m, wheelMsg(tea.MouseButtonWheelUp))
	if m.cursor != 0 {
		t.Errorf("wheel up at top moved cursor to %d", m.cursor)
	}
	for i := 0; i < 10; i++ {
		m, _ = mouseTo(t, m, wheelMsg(tea.MouseButtonWheelDown))
	}
	if want := len(m.rows) - 1; m.cursor != want {
		t.Errorf("wheel down past bottom left cursor at %d, want %d", m.cursor, want)
	}
	m, _ = mouseTo(t, m, wheelMsg(tea.MouseButtonWheelUp))
	if want := len(m.rows) - 2; m.cursor != want {
		t.Errorf("wheel up moved cursor to %d, want %d", m.cursor, want)
	}
	// Detail: the wheel drives the same clamped line window as j/k.
	m = openDetail(t, newTopModel(newFakeSteering()), "t-flak")
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 8})
	m = mm.(topModel)
	if m.detailMaxScroll() == 0 {
		t.Fatal("detail fits in 8 rows; test needs scrollable content")
	}
	m, _ = mouseTo(t, m, wheelMsg(tea.MouseButtonWheelDown))
	if m.detailScroll != 1 {
		t.Errorf("wheel down scrolled detail to %d, want 1", m.detailScroll)
	}
	for i := 0; i < 100; i++ {
		m, _ = mouseTo(t, m, wheelMsg(tea.MouseButtonWheelDown))
	}
	if want := m.detailMaxScroll(); m.detailScroll != want {
		t.Errorf("wheel past the end: scroll %d, want clamp at %d", m.detailScroll, want)
	}
	for i := 0; i < 100; i++ {
		m, _ = mouseTo(t, m, wheelMsg(tea.MouseButtonWheelUp))
	}
	if m.detailScroll != 0 {
		t.Errorf("wheel past the top: scroll %d, want 0", m.detailScroll)
	}
}

// Input modes ignore the mouse: a stray click never disturbs a pending
// answer or moves the cursor out from under it.
func TestTopClickIgnoredDuringInput(t *testing.T) {
	m := newTopModel(newFakeSteering())
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	m = mm.(topModel)
	m, _ = press(t, m, keyOf(tea.KeyEnter)) // answer the escalation
	m, _ = press(t, m, runes("Use MIT.")...)
	m, cmd := mouseTo(t, m,
		clickAt(0, 10), clickAt(0, 10), wheelMsg(tea.MouseButtonWheelDown))
	if m.mode != modeAnswer || m.input != "Use MIT." || m.cursor != 0 || cmd != nil {
		t.Errorf("mouse during input: mode %d input %q cursor %d cmd %v, want untouched modeAnswer",
			m.mode, m.input, m.cursor, cmd)
	}
}

// Clicks before the first snapshot, or while the daemon is unreachable,
// hit nothing and never panic.
func TestTopClickBeforeSnapshotIsInert(t *testing.T) {
	m := topModel{actor: "brandon", armed: true}
	if m, _ := mouseTo(t, m, clickAt(0, 3)); m.mode != modeNav || m.cursor != 0 {
		t.Errorf("click with no snapshot: mode %d cursor %d", m.mode, m.cursor)
	}
	m.err = errors.New("dial unix: no such file")
	m.snap = topSnapshot()
	m.rows = buildRows(m.snap)
	if m, _ := mouseTo(t, m, clickAt(0, 3)); m.mode != modeNav || m.cursor != 0 {
		t.Errorf("click while unreachable: mode %d cursor %d", m.mode, m.cursor)
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

// ---- inbox and held (2026-07-31): quick capture and the shelves ----

// The full capture flow: i opens a single-line input, typing builds the
// title, enter creates a title-only inbox item through the steering API
// — no y/n anywhere (capture is reversible via archive).
func TestTopQuickCaptureFlow(t *testing.T) {
	fake := newFakeSteering()
	m := newTopModel(fake)
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	if m.mode != modeCapture {
		t.Fatalf("i: mode = %d, want modeCapture", m.mode)
	}
	v := m.View()
	if !strings.Contains(v, "capture") || !strings.Contains(v, "to inbox") {
		t.Errorf("capture prompt missing; view:\n%s", v)
	}
	if strings.Contains(v, "y/n") {
		t.Errorf("capture must be y/n-free; view:\n%s", v)
	}
	m, cmd := press(t, m, append(runes("idea: sparkline history"), keyOf(tea.KeyEnter))...)
	if cmd == nil {
		t.Fatal("submit produced no command")
	}
	am := cmd().(actionMsg)
	if am.err != nil {
		t.Fatalf("action error: %v", am.err)
	}
	if len(fake.captured) != 1 || fake.captured[0] != "idea: sparkline history" {
		t.Fatalf("captured %v, want the typed title", fake.captured)
	}
	if m.mode != modeNav {
		t.Errorf("mode after submit = %d, want modeNav", m.mode)
	}
	mm, _ := m.Update(am)
	if m = mm.(topModel); !strings.Contains(m.status, "captured") {
		t.Errorf("status = %q, want a captured confirmation", m.status)
	}
}

// Empty titles are rejected in place; esc abandons the capture without
// any write.
func TestTopQuickCaptureRejectsEmptyAndEscCancels(t *testing.T) {
	fake := newFakeSteering()
	m := newTopModel(fake)
	m, cmd := press(t, m,
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}},
		keyOf(tea.KeyEnter))
	if cmd != nil || len(fake.captured) != 0 {
		t.Fatal("empty capture must not produce a write")
	}
	if m.mode != modeCapture || m.status != "title cannot be empty" {
		t.Fatalf("mode %d status %q, want in-place rejection", m.mode, m.status)
	}
	m, _ = press(t, m, append(runes("half a thou"), keyOf(tea.KeyEsc))...)
	if m.mode != modeNav || len(fake.captured) != 0 {
		t.Fatalf("esc did not abandon capture: mode %d captured %v", m.mode, fake.captured)
	}
}

// Capture is fully absent in watch mode: i is dead (covered in
// TestWatchModeDisarmed) and no bar or footer advertises it.
func TestWatchModeNeverAdvertisesCapture(t *testing.T) {
	m := newWatchModel()
	if v := m.View(); strings.Contains(v, "i capture") {
		t.Errorf("watch mode advertises capture; view:\n%s", v)
	}
}

// Shelf rows are ordinary rows: enter opens the biography, c archives —
// on both held and inbox.
func TestTopShelfRowsOpenDetailAndArchive(t *testing.T) {
	for _, id := range []string{"t-park", "t-idea"} {
		fake := newFakeSteering()
		m := newTopModel(fake)
		m = moveTo(t, m, id)
		m, _ = press(t, m, keyOf(tea.KeyEnter))
		if m.mode != modeDetail || m.detailID != id {
			t.Fatalf("enter on %s: mode %d detail %q, want its detail", id, m.mode, m.detailID)
		}
		m, _ = press(t, m, keyOf(tea.KeyEsc))
		m, cmd := press(t, m,
			tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}},
			tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
		if cmd == nil {
			t.Fatalf("archive on %s produced no command", id)
		}
		if am := cmd().(actionMsg); am.err != nil {
			t.Fatalf("archive %s: %v", id, am.err)
		}
		if len(fake.archived) != 1 || fake.archived[0] != id {
			t.Fatalf("archived %v, want [%s]", fake.archived, id)
		}
	}
}

// A task blocked on an inbox or held dependency names that status in
// its waiting: reason, through the existing taskRef annotation.
func TestTopBlockedOnShelvedDependency(t *testing.T) {
	s := topSnapshot()
	s.state.Tasks = append(s.state.Tasks,
		stateTask{ID: "t-wait", Title: "build on the idea", Status: "open"})
	s.tasks["t-wait"] = hydratedTask{Task: taskJSON{
		ID: "t-wait", Title: "build on the idea",
		DependsOn: []string{"t-idea", "t-park"},
	}}
	m := topModel{armed: true, actor: "brandon", snap: s, rows: buildRows(s), width: 160, height: 60}
	v := m.View()
	for _, want := range []string{
		"depends on t-idea (inbox — idea: dark mode)",
		"depends on t-park (on hold — polish the docs)",
	} {
		if !strings.Contains(wrapForSearch(v), want) {
			t.Errorf("waiting reason missing %q; view:\n%s", want, v)
		}
	}
}

// wrapForSearch flattens a rendered view so assertions survive line
// wrapping: newlines plus their indentation collapse to nothing.
func wrapForSearch(v string) string {
	out := strings.ReplaceAll(v, "\n", "")
	return strings.Join(strings.Fields(out), " ")
}
