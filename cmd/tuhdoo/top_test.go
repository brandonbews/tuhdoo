package main

// Unit tests for tuhdoo top: pure state->render checks plus interaction
// logic driven through Update with a fake steeringAPI — no TTY, no
// daemon (the real daemon round trip lives in top_cli_test.go).

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/brandonbews/tuhdoo/internal/event"
)

// pint builds the pointer literals nullable priority fixtures need
// (P0-highest flip, 2026-08-21).
func pint(n int) *int { return &n }

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
				{ID: "t-pars", Title: "write the parser", Status: "open", Priority: pint(5), Situation: "ready"},
				{ID: "t-flor", Title: "sweep the floor", Status: "open", Priority: pint(1), Situation: "ready"},
				{ID: "t-flak", Title: "investigate the flake", Status: "open", Holder: "brandon/a1", Situation: "in_progress"},
				{ID: "t-lic", Title: "choose a license", Status: "open", Situation: "blocked", BlockingEscalations: []string{"01E1"}},
				{ID: "t-park", Title: "polish the docs", Status: "held", Priority: pint(2), Situation: "held"},
				{ID: "t-idea", Title: "idea: dark mode", Status: "inbox", Situation: "inbox"},
				{ID: "t-chor", Title: "old chore", Status: "done", Situation: "done"},
			},
			OpenEscalations: []escalationJSON{esc},
		},
		tasks: map[string]hydratedTask{
			"t-pars": {Task: taskJSON{
				ID: "t-pars", Title: "write the parser",
				// Edges: t-chor is done, so t-pars still classifies ready;
				// t-epic is unknown to the snapshot (annotates bare).
				DependsOn: []string{"t-chor", "t-epic"},
			}},
			"t-flor": {Task: taskJSON{ID: "t-flor", Title: "sweep the floor"}},
			"t-flak": {
				Task: taskJSON{
					ID: "t-flak", Title: "investigate the flake",
					Description: "The parser test flakes on CI.\nFind out why.",
					Priority:    pint(3),
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
			"t-park": {Task: taskJSON{ID: "t-park", Title: "polish the docs", Priority: pint(2)}},
			"t-idea": {Task: taskJSON{ID: "t-idea", Title: "idea: dark mode"}},
			// Status matches the state row: t-chor's detail is a closed
			// record (its ring drops the priority/labels stops).
			"t-chor": {Task: taskJSON{ID: "t-chor", Title: "old chore", Status: "done"}},
		},
	}
}

func newTopModel(api steeringAPI) topModel {
	s := topSnapshot()
	// repoName happens to match the product here, as it does when
	// dogfooding in this repo — the header renders the repo-root
	// basename, and this fixture's repo is called tuhdoo.
	return topModel{api: api, actor: "brandon", armed: true, repoName: "tuhdoo", snap: s, rows: buildRows(s)}
}

// newWatchModel is the same seeded model launched with --watch: disarmed.
func newWatchModel() topModel {
	s := topSnapshot()
	return topModel{repoName: "tuhdoo", snap: s, rows: buildRows(s)}
}

// classify's ready ordering is claim_next's (snapshot.go): most
// urgent first (P0-highest, 2026-08-21 — lower number wins), and
// within a priority the creation (ULID) order the state listing
// arrives in — the sort is stable, so equal-priority tasks never
// swap.
func TestClassifyReadyTieBreak(t *testing.T) {
	s := topSnapshot()
	s.state.Tasks = append(s.state.Tasks,
		stateTask{ID: "t-tie1", Title: "born first", Status: "open", Priority: pint(3), Situation: "ready"},
		stateTask{ID: "t-tie2", Title: "born second", Status: "open", Priority: pint(3), Situation: "ready"},
	)
	var ids []string
	for _, task := range s.classify().ready {
		ids = append(ids, task.ID)
	}
	if want := []string{"t-flor", "t-tie1", "t-tie2", "t-pars"}; !slices.Equal(ids, want) {
		t.Errorf("ready order = %v, want %v", ids, want)
	}
}

// newTopModelWithDep reseeds t-lic with an unmet dependency before
// building the model. Escalation-only blockage earns no task row (the
// Needs Input row is its single home, 2026-07-31), and an armed enter
// on that row answers — so tests that open t-lic's detail from the
// list walk to the BLOCKED row its unmet dep earns.
func newTopModelWithDep(api steeringAPI) topModel {
	s := topSnapshot()
	h := s.tasks["t-lic"]
	h.Task.DependsOn = []string{"t-flor"}
	s.tasks["t-lic"] = h
	for i, t := range s.state.Tasks {
		if t.ID == "t-lic" {
			s.state.Tasks[i].UnmetDeps = []string{"t-flor"}
		}
	}
	return topModel{api: api, actor: "brandon", armed: true, snap: s, rows: buildRows(s)}
}

// fakeSteering records calls; err, when set, fails every call.
type fakeSteering struct {
	answers    map[string]string
	priorities map[string]int
	cancelled  []string
	captured   []string            // quick-capture titles, in order
	titles     map[string]string   // task-view title edits
	descs      map[string]string   // task-view description edits
	labels     map[string][]string // task-view label edits (full replacement)
	err        error
}

func newFakeSteering() *fakeSteering {
	return &fakeSteering{
		answers:    map[string]string{},
		priorities: map[string]int{},
		titles:     map[string]string{},
		descs:      map[string]string{},
		labels:     map[string][]string{},
	}
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

func (f *fakeSteering) captureTask(title string) error {
	if f.err != nil {
		return f.err
	}
	f.captured = append(f.captured, title)
	return nil
}

func (f *fakeSteering) setTitle(task, title string) error {
	if f.err != nil {
		return f.err
	}
	f.titles[task] = title
	return nil
}

func (f *fakeSteering) setDescription(task, description string) error {
	if f.err != nil {
		return f.err
	}
	f.descs[task] = description
	return nil
}

func (f *fakeSteering) setLabels(task string, labels []string) error {
	if f.err != nil {
		return f.err
	}
	f.labels[task] = labels
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
		{rowTask, "ready", "t-flor"}, // p1 before p5 (P0-highest)
		{rowTask, "ready", "t-pars"},
		{rowTask, "inprogress", "t-flak"},
		// t-lic is blocked by its escalation alone: its Needs Input row
		// is its single representation — no blocked row (2026-07-31).
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
	m.width = 100 // wide enough for the legend and the done tally together
	v := m.View()
	for _, want := range []string{
		"tuhdoo · local-only",
		"NEEDS INPUT (1)", "question: Which license?", "brandon/a2 · 2026-07-29 14:03 UTC",
		"READY (2)", "write the parser", "sweep the floor",
		"IN PROGRESS (1)", "investigate the flake", "← brandon/a1",
		"BLOCKED (0)",
		"▌ t-lic   !   choose a license", // cursor starts on the task-shaped escalation row
		"↑/↓ (j/k) move · enter open · p priority · c cancel · h history · q quit",
		"1 done", // the footer bar tally replaced the counts line
	} {
		if !strings.Contains(v, want) {
			t.Errorf("view missing %q; view:\n%s", want, v)
		}
	}
	if strings.Contains(v, "old chore") {
		t.Errorf("done task should not render a steerable row; view:\n%s", v)
	}
	// The escalation-blocked task renders exactly once — its task-shaped
	// Needs Input row — and the question exactly once, on that row's
	// question: line. No BLOCKED row, no "needs input (above)" pointer,
	// no "blocking" word: the ! badge carries it (grill cycle,
	// 2026-07-31).
	if n := strings.Count(v, "Which license?"); n != 1 {
		t.Errorf("question renders %d times, want exactly 1; view:\n%s", n, v)
	}
	if n := strings.Count(v, "choose a license"); n != 1 {
		t.Errorf("escalation-blocked task renders %d times, want exactly 1; view:\n%s", n, v)
	}
	for _, reject := range []string{"waiting:", "needs input", "blocking"} {
		if strings.Contains(v, reject) {
			t.Errorf("view still contains %q; view:\n%s", reject, v)
		}
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
	if v := m.View(); !strings.Contains(v, "▌ t-park") {
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
	m = openDetail(t, m, "t-flak") // the longest biography
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
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}) // t-flor (p1 leads ready)
	fresh := topSnapshot()
	mm, _ := m.Update(snapMsg{snap: fresh})
	m = mm.(topModel)
	if r, ok := m.selected(); !ok || r.id() != "t-flor" {
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

// TestTopAnswerFlow is the routing contract (task-view rework,
// 2026-08-01, superseding the 2026-07-30 inline prompt): enter on a
// Needs Input row lands in the task view with the escalation
// preselected — never the old bottom-of-dashboard answer prompt — and
// enter there opens answer entry, submitting through the same plumbing.
func TestTopAnswerFlow(t *testing.T) {
	fake := newFakeSteering()
	m := newTopModel(fake)
	m, cmd := press(t, m, keyOf(tea.KeyEnter))
	if m.mode != modeDetail || m.detailID != "t-lic" || cmd != nil {
		t.Fatalf("enter on an escalation row: mode %d detail %q, want the task view of t-lic", m.mode, m.detailID)
	}
	if m.detailFocus != 3 {
		t.Fatalf("detailFocus = %d, want the escalation stop preselected at 3 (after title, priority, and labels)", m.detailFocus)
	}
	v := m.View()
	for _, want := range []string{
		"t-lic — choose a license", // the task's context is on screen
		"NEEDS INPUT (1)",
		"▌ !   Which license?", // the escalation row, selected
	} {
		if !strings.Contains(v, want) {
			t.Errorf("task view missing %q; view:\n%s", want, v)
		}
	}
	// Enter on the selected question opens answer entry in the footer.
	m, _ = press(t, m, keyOf(tea.KeyEnter))
	if m.mode != modeAnswer {
		t.Fatalf("enter on the selected question: mode %d, want modeAnswer", m.mode)
	}
	v = m.View()
	if !strings.Contains(v, "answer") || !strings.Contains(v, "Which license?") ||
		!strings.Contains(v, "enter submits · esc cancels") {
		t.Errorf("answer prompt missing; view:\n%s", v)
	}
	m, _ = press(t, m, runes("Use MIT.")...)
	if m.input.String() != "Use MIT." {
		t.Fatalf("input = %q, want %q", m.input, "Use MIT.")
	}
	m, _ = press(t, m, keyOf(tea.KeyBackspace))
	if m.input.String() != "Use MIT" {
		t.Fatalf("backspace left %q", m.input)
	}
	m, cmd = press(t, m, keyOf(tea.KeyEnter))
	if m.mode != modeDetail {
		t.Errorf("mode after submit = %d, want modeDetail (back to the task view)", m.mode)
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
	// Success says nothing (feedback only, 2026-08-21): the refreshed
	// screen is the confirmation.
	if m.status != "" {
		t.Errorf("status = %q, want empty after success", m.status)
	}
}

func TestTopAnswerRejectsEmptyAndEscCancels(t *testing.T) {
	fake := newFakeSteering()
	m := newTopModel(fake)
	// enter routes to the task view; enter opens answer entry; enter on
	// empty input is rejected in place.
	m, cmd := press(t, m, keyOf(tea.KeyEnter), keyOf(tea.KeyEnter), keyOf(tea.KeyEnter))
	if cmd != nil {
		t.Error("empty answer still submitted")
	}
	if m.mode != modeAnswer || !strings.Contains(m.status, "empty") {
		t.Errorf("empty answer: mode %d status %q", m.mode, m.status)
	}
	m, _ = press(t, m, keyOf(tea.KeyEsc))
	if m.mode != modeDetail {
		t.Errorf("esc left mode %d, want modeDetail (back to the task view)", m.mode)
	}
	if len(fake.answers) != 0 {
		t.Errorf("esc still answered: %v", fake.answers)
	}
}

func TestTopActionKeysRespectRowKind(t *testing.T) {
	m := newTopModel(newFakeSteering())
	// p and a on an escalation row: ignored — they steer task rows only.
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if m.mode != modeNav {
		t.Errorf("p on an escalation row entered mode %d", m.mode)
	}
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if m.mode != modeNav {
		t.Errorf("c on an escalation row entered mode %d", m.mode)
	}
	// a is free (status-vocabulary revision, 2026-08-01: cancel moved
	// back to c) — it does nothing anywhere, task rows included.
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if m.mode != modeNav {
		t.Errorf("freed key a still entered mode %d", m.mode)
	}
	m, _ = press(t, m,
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}, // t-pars, a task row
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if m.mode != modeNav {
		t.Errorf("freed key a on a task row entered mode %d", m.mode)
	}
}

func TestTopPriorityFlow(t *testing.T) {
	fake := newFakeSteering()
	m := newTopModel(fake)
	m, _ = press(t, m,
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}, // t-flor (p1 leads ready)
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
	if got := fake.priorities["t-flor"]; got != 7 {
		t.Errorf("priority set to %d, want 7", got)
	}
}

func TestTopCancelFlow(t *testing.T) {
	fake := newFakeSteering()
	m := newTopModel(fake)
	m, _ = press(t, m,
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}, // t-pars (second in ready under P0-highest)
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if m.mode != modeConfirmCancel {
		t.Fatalf("mode = %d, want modeConfirmCancel", m.mode)
	}
	v := m.View()
	if !strings.Contains(v, "cancel t-pars") || !strings.Contains(v, "y/n") ||
		!strings.Contains(v, "history stays on the ledger") {
		t.Errorf("confirm prompt missing; view:\n%s", v)
	}
	// The archive vocabulary is retired (2026-08-01).
	if strings.Contains(v, "archive t-pars") {
		t.Errorf("confirm prompt still says archive; view:\n%s", v)
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
	am := cmd().(actionMsg)
	if am.err != nil {
		t.Fatalf("action error: %v", am.err)
	}
	if len(fake.cancelled) != 1 || fake.cancelled[0] != "t-pars" {
		t.Errorf("cancelled %v, want [t-pars]", fake.cancelled)
	}
	// No confirmation (feedback only, 2026-08-21): the row vanishing on
	// the refresh is the confirmation.
	mm, _ := m.Update(am)
	m = mm.(topModel)
	if m.status != "" {
		t.Errorf("status = %q, want empty after success", m.status)
	}
}

func TestTopActionErrorSurfacesInStatus(t *testing.T) {
	fake := newFakeSteering()
	fake.err = errors.New("writes rejected (fail-safe read-only)")
	m := newTopModel(fake)
	m, cmd := press(t, m,
		keyOf(tea.KeyEnter), // task view of t-lic, escalation preselected
		keyOf(tea.KeyEnter), // answer entry
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
	// In input mode q is text, ctrl+c still quits. (Enter routes to the
	// task view; enter there opens answer input for the preselected
	// escalation.)
	m, _ = press(t, m, keyOf(tea.KeyEnter), keyOf(tea.KeyEnter),
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if m.mode != modeAnswer || m.input.String() != "q" {
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
	// The badge reads a dim "watch" since the chrome pass (2026-08-21;
	// was "watch mode").
	for _, want := range []string{"watch", "BLOCKED (0)", "↑/↓ (j/k) move · enter open · h history · q quit"} {
		if !strings.Contains(v, want) {
			t.Errorf("watch-mode view missing %q; view:\n%s", want, v)
		}
	}
	for _, reject := range []string{"as brandon", "enter answer", "c cancel"} {
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
	m = openDetail(t, m, "t-flak")
	v := m.View()
	for _, want := range []string{
		"t-flak — investigate the flake",
		"DESCRIPTION", "The parser test flakes on CI.",
		"HISTORY", "note by brandon/a1", "Repros only under -race.",
		"run by brandon/a1 — interrupted", "Bisecting the flake.",
		"Skip the flaky test until fixed?",
		"A (brandon, relayed by brandon/a1): Skip it, link the issue.",
		// One blank line between consecutive history entries (entry
		// formatting, 2026-08-03) — the separator survives the per-entry
		// TrimRight because it leads the next entry.
		"Skip it, link the issue.\n\n  2026-07-29 15:00 UTC  note by brandon/a1",
		"Repros only under -race.\n\n  (unknown time)  run by brandon/a1 — interrupted",
		// A plain open focuses the title stop at the top of the view.
		"▌ t-flak — investigate the flake",
		// The armed legend: the ring is always live, enter opens editors.
		"↑/↓ (j/k) move · enter edit · p priority · c cancel · esc back · q quit",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("detail view missing %q; view:\n%s", want, v)
		}
	}
	if strings.Contains(v, "enter open") {
		t.Errorf("detail view leaked the list footer; view:\n%s", v)
	}
	// No open escalation: the NEEDS INPUT section does not render (the
	// answered one lives in HISTORY).
	if strings.Contains(v, "NEEDS INPUT") {
		t.Errorf("detail view grew an empty escalations section; view:\n%s", v)
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
	m = openDetail(t, m, "t-flak")
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
	m = openDetail(t, m, "t-flak") // the longest biography
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

// Edge markers: a row whose task has dependencies says so on its dim
// meta line (two-line rows, 2026-08-05); edge-free rows stay one line
// — the "no labels, no edges" signal. Same renderer in both modes.
func TestTopEdgeMarkers(t *testing.T) {
	for name, m := range map[string]topModel{
		"steer": newTopModel(newFakeSteering()),
		"watch": newWatchModel(),
	} {
		v := m.View()
		if !strings.Contains(v, "write the parser\n              2 deps") {
			t.Errorf("%s: parser row missing its edge meta line; view:\n%s", name, v)
		}
		if !strings.Contains(v, "sweep the floor\n  t-pars") {
			t.Errorf("%s: edge-free row grew a second line; view:\n%s", name, v)
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
		if got := event.ShortID(tt.in); got != tt.want {
			t.Errorf("event.ShortID(%q) = %q, want %q", tt.in, got, tt.want)
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
			{ID: long, Title: "the long one", Status: "open", Situation: "blocked", UnmetDeps: []string{dep}},
			{ID: dep, Title: "its dependency", Status: "open", Situation: "ready"},
		}},
		tasks: map[string]hydratedTask{
			long: {Task: taskJSON{ID: long, Title: "the long one", DependsOn: []string{dep}}},
			dep:  {Task: taskJSON{ID: dep, Title: "its dependency", DependsOn: []string{long}}},
		},
	}
	m := topModel{armed: true, actor: "brandon", snap: s, rows: buildRows(s)}
	v := m.View()
	for _, want := range []string{
		"▌ t-rqjm      its dependency", // ready row, selected; unprioritized = no badge
		"▌             1 dep",          // its edge marker on the meta line, barred too
		"t-d83w      the long one",     // blocked row: blank badge cell
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
	// The detail screen is short-form throughout — the `id` line included
	// (T7 revision, 2026-08-02: the full ULID has no TUI surface; one-shot
	// `tuhdoo task <id>` is where the full form lives).
	m, _ = press(t, m, keyOf(tea.KeyEnter)) // cursor on the ready row (dep)
	dv := m.View()
	for _, want := range []string{
		"t-rqjm — its dependency",
		"id          t-rqjm",
		// Its edges render as section rows (edge rows, 2026-08-11) —
		// short IDs there too, and this pair depends on each other, so
		// both directions show the same row shape.
		" DEPENDS ON (1)",
		" NEEDED BY (1)",
		"  t-d83w  open  the long one",
	} {
		if !strings.Contains(dv, want) {
			t.Errorf("detail view missing %q; view:\n%s", want, dv)
		}
	}
	if strings.Contains(dv, dep) || strings.Contains(dv, long) {
		t.Errorf("a full ULID leaked into detail; view:\n%s", dv)
	}
}

// taskRef is the edge annotation contract: references that resolve in
// the snapshot carry status and title (done and cancelled tasks are in
// the state listing even though they render no rows — the annotation
// is what proves such an edge isn't dangling); unresolvable IDs render
// bare — never an invented status; long titles are ellipsized. Status
// words are the stored words (status-vocabulary revision, 2026-08-01),
// except held, which reads "on hold".
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
			"t-d83w (cancelled — wide wide wide wide wide wide wide wide…)"},
	}
	for _, tt := range tests {
		if got := s.taskRef(tt.id); got != tt.want {
			t.Errorf("taskRef(%q) = %q, want %q", tt.id, got, tt.want)
		}
	}
}

// The task view's DEPENDS ON section (edge rows, 2026-08-11): the
// comma-joined depends-on field line is gone — one row per edge now,
// bold short ID (plain here), dim status word, plain title, each a
// single line. An edge to a task the snapshot has never heard of
// renders its bare short ID — never an invented status.
func TestTopDetailAnnotatesEdges(t *testing.T) {
	m := newTopModel(newFakeSteering())
	m, _ = press(t, m,
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}, // t-pars (second in ready under P0-highest)
		keyOf(tea.KeyEnter))
	v := m.View()
	for _, want := range []string{
		"t-pars — write the parser",
		"id          t-pars",
		" DEPENDS ON (2)",
		// t-chor resolves and annotates; t-epic is unknown to the
		// snapshot and stays a bare row.
		"  t-chor  done  old chore",
		"\n  t-epic\n",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("detail view missing %q; view:\n%s", want, v)
		}
	}
	if strings.Contains(v, "depends on  ") {
		t.Errorf("the comma-joined depends-on field line survived; view:\n%s", v)
	}
	if strings.Contains(v, "t-epic (") {
		t.Errorf("unresolvable edge grew an invented annotation; view:\n%s", v)
	}
	// Nothing depends on t-pars: no NEEDED BY section.
	if strings.Contains(v, "NEEDED BY") {
		t.Errorf("dependent-free task grew a NEEDED BY section; view:\n%s", v)
	}
}

// edgeSnapshot seeds an epic-shaped structure for the edge sections
// (edge rows, 2026-08-11): t-epic depends on four children spanning
// every status family, t-bbbb is needed by two tasks (creation order),
// and t-lone has no edges at all.
func edgeSnapshot() *snapshot {
	return &snapshot{
		state: stateResp{
			Sync: syncJSON{Mode: "local-only"},
			Tasks: []stateTask{
				{ID: "t-epic", Title: "ship the epic", Status: "open", Situation: "blocked",
					UnmetDeps: []string{"t-bbbb", "t-cccc", "t-dddd"}},
				{ID: "t-aaaa", Title: "first child", Status: "done", Situation: "done"},
				{ID: "t-bbbb", Title: "second child", Status: "open", Situation: "ready"},
				{ID: "t-cccc", Title: "third child", Status: "held", Situation: "held"},
				{ID: "t-dddd", Title: "dropped child", Status: "cancelled", Situation: "cancelled"},
				{ID: "t-late", Title: "build on it", Status: "open", Situation: "blocked",
					UnmetDeps: []string{"t-bbbb"}},
				{ID: "t-lone", Title: "no edges here", Status: "open", Situation: "ready"},
			},
		},
		tasks: map[string]hydratedTask{
			"t-epic": {Task: taskJSON{ID: "t-epic", Title: "ship the epic", Status: "open",
				CreatedAt: time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC), CreatedBy: "brandon",
				DependsOn: []string{"t-aaaa", "t-bbbb", "t-cccc", "t-dddd"}}},
			"t-aaaa": {Task: taskJSON{ID: "t-aaaa", Title: "first child", Status: "done"}},
			"t-bbbb": {Task: taskJSON{ID: "t-bbbb", Title: "second child", Status: "open"}},
			"t-cccc": {Task: taskJSON{ID: "t-cccc", Title: "third child", Status: "held"}},
			"t-dddd": {Task: taskJSON{ID: "t-dddd", Title: "dropped child", Status: "cancelled"}},
			"t-late": {Task: taskJSON{ID: "t-late", Title: "build on it", Status: "open", DependsOn: []string{"t-bbbb"}}},
			"t-lone": {Task: taskJSON{ID: "t-lone", Title: "no edges here", Status: "open"}},
		},
	}
}

// The edge sections end to end (edge rows, 2026-08-11): several deps
// render as aligned single lines with their status words visible; the
// depended-on task shows a NEEDED BY section listing every dependent
// regardless of status, in ULID order; a task with neither edge
// direction shows neither section. Watch mode renders the same rows —
// visible, never selectable, bar hint absent.
func TestTopDetailEdgeSections(t *testing.T) {
	s := edgeSnapshot()
	// The daemon's loud verdict rides the state row: one of the epic's
	// unmet deps sits cancelled.
	s.state.Tasks[0].CancelledDeps = []string{"t-dddd"}
	m := topModel{api: newFakeSteering(), actor: "brandon", armed: true, snap: s, rows: buildRows(s)}
	m = openDetail(t, m, "t-epic")
	v := m.View()
	// One aligned grid: statuses pad to the widest word ("cancelled"),
	// so every title starts at the same column.
	for _, want := range []string{
		" DEPENDS ON (4)",
		"  t-aaaa  done       first child",
		"  t-bbbb  open       second child",
		"  t-cccc  on hold    third child",
		"  t-dddd  cancelled  dropped child",
		// The cancelled dep also grows the detail's waiting field line
		// (top.go's field(-1, "waiting", …) wiring), naming the dep.
		"  waiting     waiting on cancelled t-dddd (cancelled — dropped child)",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("epic detail missing %q; view:\n%s", want, v)
		}
	}
	if strings.Contains(v, "NEEDED BY") {
		t.Errorf("nothing needs the epic, yet a NEEDED BY section rendered; view:\n%s", v)
	}
	// The depended-on child: NEEDED BY lists both dependents in ULID
	// (creation) order — the epic first — statuses visible even though
	// one dependent is blocked.
	m, _ = press(t, m, keyOf(tea.KeyEsc))
	m = openDetail(t, m, "t-bbbb")
	v = m.View()
	needed := strings.Index(v, " NEEDED BY (2)")
	if needed < 0 {
		t.Fatalf("child detail missing the NEEDED BY section; view:\n%s", v)
	}
	epicRow := strings.Index(v, "  t-epic  open  ship the epic")
	lateRow := strings.Index(v, "  t-late  open  build on it")
	if epicRow < needed || lateRow < epicRow {
		t.Errorf("dependents missing or out of ULID order (bar %d, epic %d, late %d); view:\n%s",
			needed, epicRow, lateRow, v)
	}
	if strings.Contains(v, "DEPENDS ON") {
		t.Errorf("dep-free child grew a DEPENDS ON section; view:\n%s", v)
	}
	// A task with neither edge direction shows neither section.
	m, _ = press(t, m, keyOf(tea.KeyEsc))
	m = openDetail(t, m, "t-lone")
	if v := m.View(); strings.Contains(v, "DEPENDS ON") || strings.Contains(v, "NEEDED BY") {
		t.Errorf("edge-free task grew an edge section; view:\n%s", v)
	}
	// Watch mode: the rows render — reading is watch work — but nothing
	// is selectable and the bar carries no steering hint.
	w := topModel{snap: s, rows: buildRows(s)}
	w = moveTo(t, w, "t-epic")
	w, _ = press(t, w, keyOf(tea.KeyEnter))
	wv := w.View()
	if !strings.Contains(wv, "  t-cccc  on hold    third child") {
		t.Errorf("watch detail lost the edge rows; view:\n%s", wv)
	}
	if strings.Contains(wv, "enter open") || strings.Contains(wv, "▌") {
		t.Errorf("watch detail advertises or selects edge rows; view:\n%s", wv)
	}
}

// Edge rows are selectable stops in the existing focus machinery:
// enter on a dep row opens the target task's view (pushing the current
// one onto the back stack), a hop along a NEEDED BY row pushes again,
// and esc walks the visited stack back — reaching the dashboard,
// cursor intact, after the first task. The selection bar covers the
// focused edge row.
func TestTopDetailEdgeNavigationAndBackStack(t *testing.T) {
	m := openDetail(t, newTopModel(newFakeSteering()), "t-pars")
	// Ring: title(0), priority(1), labels(2), dep t-chor(3) — the
	// unresolvable t-epic is never a stop — description(4).
	m, _ = press(t, m,
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	stops := m.detailStops()
	if len(stops) != 5 {
		t.Fatalf("ring has %d stops, want 5 (bare t-epic must not be a stop): %+v", len(stops), stops)
	}
	if s := stops[detailFocusIdx(m.detailFocus, len(stops))]; s.kind != stopDep || s.task != "t-chor" {
		t.Fatalf("three j from the title: stop %+v, want the t-chor dep row", s)
	}
	if s := stops[len(stops)-1]; s.kind != stopDescription {
		t.Fatalf("last stop %+v, want the description", s)
	}
	// The selection bar covers the focused edge row.
	if body := strings.Join(m.detailBody(), "\n"); !strings.Contains(body, "▌ t-chor  done  old chore") {
		t.Fatalf("selection bar not on the edge row; body:\n%s", body)
	}
	// enter hops to the dep's view, pushing the current task; the
	// hopped-to view opens at the top with the title focused.
	m, cmd := press(t, m, keyOf(tea.KeyEnter))
	if m.mode != modeDetail || m.detailID != "t-chor" || cmd != nil {
		t.Fatalf("enter on the dep row: mode %d detail %q cmd %v, want t-chor's view", m.mode, m.detailID, cmd)
	}
	if len(m.detailBack) != 1 || m.detailBack[0] != "t-pars" {
		t.Fatalf("back stack = %v, want [t-pars]", m.detailBack)
	}
	if m.detailFocus != 0 || m.detailScroll != 0 {
		t.Errorf("hop landed at focus %d scroll %d, want 0 0", m.detailFocus, m.detailScroll)
	}
	// t-chor is terminal: its ring is title(0), needed-by t-pars(1),
	// description(2) — an edge hop is navigation, not steering, so the
	// closed record keeps its edge stops. Hop back out along the
	// reverse edge: a second push, not a pop.
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if body := strings.Join(m.detailBody(), "\n"); !strings.Contains(body, "▌ t-pars  open  write the parser") {
		t.Fatalf("needed-by row not focused; body:\n%s", body)
	}
	m, _ = press(t, m, keyOf(tea.KeyEnter))
	if m.detailID != "t-pars" || len(m.detailBack) != 2 || m.detailBack[1] != "t-chor" {
		t.Fatalf("reverse hop: detail %q back %v, want t-pars with [t-pars t-chor]", m.detailID, m.detailBack)
	}
	// esc pops the stack one view at a time, then reaches the dashboard
	// with the cursor where the walk began.
	m, _ = press(t, m, keyOf(tea.KeyEsc))
	if m.mode != modeDetail || m.detailID != "t-chor" {
		t.Fatalf("first esc: mode %d detail %q, want t-chor's view", m.mode, m.detailID)
	}
	m, _ = press(t, m, keyOf(tea.KeyEsc))
	if m.mode != modeDetail || m.detailID != "t-pars" {
		t.Fatalf("second esc: mode %d detail %q, want t-pars's view", m.mode, m.detailID)
	}
	m, _ = press(t, m, keyOf(tea.KeyEsc))
	if m.mode != modeNav || m.detailID != "" {
		t.Fatalf("third esc: mode %d detail %q, want the dashboard", m.mode, m.detailID)
	}
	if r, ok := m.selected(); !ok || r.id() != "t-pars" {
		t.Errorf("cursor lost across the walk: %+v", r)
	}
}

// Click-to-open on edge rows falls out of the existing mouse
// machinery: a click selects the row under the pointer, a click on the
// selected row hops to its target; a bare, unresolvable edge row is
// never a stop, so clicks on it hit nothing.
func TestTopDetailEdgeClick(t *testing.T) {
	m := newTopModel(newFakeSteering())
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	m = mm.(topModel)
	m = openDetail(t, m, "t-pars")
	y := screenLineOf(t, m, "old chore")
	m, cmd := mouseTo(t, m, clickAt(0, y))
	if m.mode != modeDetail || cmd != nil {
		t.Fatalf("click on the dep row: mode %d cmd %v, want selection only", m.mode, cmd)
	}
	stops := m.detailStops()
	if s := stops[detailFocusIdx(m.detailFocus, len(stops))]; s.kind != stopDep || s.task != "t-chor" {
		t.Fatalf("click selected stop %+v, want the t-chor dep row", s)
	}
	m, _ = mouseTo(t, m, clickAt(0, y))
	if m.mode != modeDetail || m.detailID != "t-chor" {
		t.Fatalf("click on the selected dep row: mode %d detail %q, want the hop to t-chor", m.mode, m.detailID)
	}
	if len(m.detailBack) != 1 || m.detailBack[0] != "t-pars" {
		t.Errorf("back stack = %v, want [t-pars]", m.detailBack)
	}
	m, _ = press(t, m, keyOf(tea.KeyEsc)) // pop back to t-pars
	focus := m.detailFocus
	m, _ = mouseTo(t, m, clickAt(0, screenLineOf(t, m, "t-epic")))
	if m.mode != modeDetail || m.detailID != "t-pars" || m.detailFocus != focus {
		t.Errorf("click on the bare edge row: mode %d detail %q focus %d, want inert",
			m.mode, m.detailID, m.detailFocus)
	}
}

// The one-shot edge rows (edge rows, 2026-08-11): with a snapshot the
// depends-on block renders one line per edge — full ID, status word,
// title — plus a needed-by block of the reverse edges; unresolvable
// IDs stay bare rows. Full IDs throughout: plumbing keeps full IDs.
func TestPrintTaskEdgeRows(t *testing.T) {
	s := topSnapshot()
	var b strings.Builder
	printTask(&b, colors{}, s.tasks["t-pars"], s.stateTaskOf("t-pars"), s)
	v := b.String()
	want := "  depends on  t-chor  done  old chore\n              t-epic\n"
	if !strings.Contains(v, want) {
		t.Errorf("one-shot depends-on block diverged; want:\n%q\noutput:\n%s", want, v)
	}
	if strings.Contains(v, "needed by") {
		t.Errorf("dependent-free task grew a needed-by block; output:\n%s", v)
	}
	// The depended-on task carries the reverse edge.
	b.Reset()
	printTask(&b, colors{}, s.tasks["t-chor"], s.stateTaskOf("t-chor"), s)
	v = b.String()
	if want := "  needed by   t-pars  open  write the parser\n"; !strings.Contains(v, want) {
		t.Errorf("one-shot needed-by block diverged; want %q; output:\n%s", want, v)
	}
	if strings.Contains(v, "depends on") {
		t.Errorf("dep-free task grew a depends-on block; output:\n%s", v)
	}
}

// The one-shot `tuhdoo task` rendering is plumbing and must not grow
// display sugar: full IDs on the title and edge lines, no canonical
// `id` line, no annotations.
func TestPrintTaskOneShotKeepsFullIDs(t *testing.T) {
	long := "t-01KYT63MB28Z535SMJC9B0D83W"
	dep := "t-01KYT63MB28Z535SMJCA63RQJM"
	dep2 := "t-01KYT63MB28Z535SMJCBC7SY1P"
	var b strings.Builder
	printTask(&b, colors{}, hydratedTask{Task: taskJSON{
		ID: long, Title: "the long one",
		DependsOn: []string{dep, dep2},
	}}, stateTask{}, nil)
	v := b.String()
	for _, want := range []string{
		long + " — the long one",
		// One row per edge (edge rows, 2026-08-11): the second dep sits
		// on its own continuation line, indented to the value column —
		// and with no snapshot the rows stay bare full IDs.
		"depends on  " + dep + "\n              " + dep2 + "\n",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("one-shot rendering missing %q; output:\n%s", want, v)
		}
	}
	for _, reject := range []string{"t-d83w", "t-rqjm", "id          "} {
		if strings.Contains(v, reject) {
			t.Errorf("one-shot rendering grew TUI sugar %q; output:\n%s", reject, v)
		}
	}
}

// The task view's waiting line (2026-08-05 edge grill) carries the loud
// annotations only: loop membership and cancelled deps render, a plain
// unmet dep grows no line — the depends-on line already tells that.
func TestPrintTaskWaitingLine(t *testing.T) {
	h := hydratedTask{Task: taskJSON{ID: "t-x", Title: "stuck", DependsOn: []string{"t-dep"}}}
	var b strings.Builder
	printTask(&b, colors{}, h, stateTask{ID: "t-x", UnmetDeps: []string{"t-dep"}}, nil)
	if strings.Contains(b.String(), "waiting") {
		t.Errorf("plain unmet dep grew a waiting line:\n%s", b.String())
	}

	b.Reset()
	printTask(&b, colors{}, h, stateTask{ID: "t-x", UnmetDeps: []string{"t-dep"},
		CancelledDeps: []string{"t-dep"}, Cyclic: true}, nil)
	want := "  waiting     cyclic — a human must cut an edge; waiting on cancelled t-dep\n"
	if !strings.Contains(b.String(), want) {
		t.Errorf("annotated task view missing %q; output:\n%s", want, b.String())
	}
}

// The dashboard's waiting: line marks loop members and cancelled deps
// distinctly from ordinary waiting (2026-08-05 edge grill), rendering
// only the daemon's verdicts — never re-deriving them.
func TestBlockedReasonTUIMarks(t *testing.T) {
	id := func(s string) string { return s }
	row := stateTask{UnmetDeps: []string{"t-a", "t-b"}, CancelledDeps: []string{"t-b"}, Cyclic: true}
	want := "cyclic — a human must cut an edge; depends on t-a; waiting on cancelled t-b"
	if got := blockedReasonTUI(row, id); got != want {
		t.Errorf("blockedReasonTUI = %q, want %q", got, want)
	}
	// Unannotated rows keep their old line exactly.
	plain := stateTask{UnmetDeps: []string{"t-a"}}
	if got := blockedReasonTUI(plain, id); got != "depends on t-a" {
		t.Errorf("plain blockedReasonTUI = %q, want %q", got, "depends on t-a")
	}
}

// History entry formatting (2026-08-03) on the one-shot rendering: one
// blank line between consecutive entries — none before the first or
// after the last, the section framing owns the edges — and each
// header's descriptor bold while its stamp stays dim; the run outcome
// folds into the bold descriptor. Both surfaces share historyOf, so
// this pins the shape once at the source.
func TestPrintTaskHistoryEntryFormatting(t *testing.T) {
	h := hydratedTask{
		Task: taskJSON{ID: "t-hist", Title: "the biography"},
		Notes: []noteJSON{{
			ID: "01N1", Task: "t-hist", Actor: "brandon/a1",
			Text:    "Repros only under -race.",
			AddedAt: time.Date(2026, 7, 29, 15, 0, 0, 0, time.UTC),
		}},
		Runs: []runJSON{{
			ID: "01R1", Task: "t-hist", Actor: "brandon/a1",
			Outcome: "interrupted", Summary: "Bisecting the flake.",
		}},
		Escalations: []escalationJSON{{
			ID: "01E2", Task: "t-hist", Actor: "brandon/a7",
			Question: "Skip it?",
			RaisedAt: time.Date(2026, 7, 29, 15, 30, 0, 0, time.UTC),
		}},
	}
	var b strings.Builder
	printTask(&b, colors{}, h, stateTask{}, nil)
	// ULID order: escalation 01E2, then note 01N1, then run 01R1. The
	// suffix match proves no blank line trails the last entry.
	wantHist := strings.Join([]string{
		"History",
		"  2026-07-29 15:30 UTC  escalation from brandon/a7",
		"    Q: Skip it?",
		"    unanswered",
		"",
		"  2026-07-29 15:00 UTC  note by brandon/a1",
		"    Repros only under -race.",
		"",
		"  (unknown time)  run by brandon/a1 — interrupted",
		"    Bisecting the flake.",
		"",
	}, "\n")
	if v := b.String(); !strings.HasSuffix(v, wantHist) {
		t.Errorf("plain history section diverged.\nwant suffix:\n%s\ngot:\n%s", wantHist, v)
	}

	// With real colors: dim stamp, bold descriptor, on every kind.
	b.Reset()
	printTask(&b, ansiColors, h, stateTask{}, nil)
	v := b.String()
	for _, want := range []string{
		"  \x1b[90m2026-07-29 15:30 UTC\x1b[0m  \x1b[1mescalation from brandon/a7\x1b[0m\n",
		"  \x1b[90m2026-07-29 15:00 UTC\x1b[0m  \x1b[1mnote by brandon/a1\x1b[0m\n",
		"  \x1b[90m(unknown time)\x1b[0m  \x1b[1mrun by brandon/a1 — interrupted\x1b[0m\n",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("colored history missing %q; output:\n%q", want, v)
		}
	}

	// A single entry earns no separator anywhere.
	single := hydratedTask{
		Task:  h.Task,
		Notes: h.Notes,
	}
	b.Reset()
	printTask(&b, colors{}, single, stateTask{}, nil)
	wantSingle := strings.Join([]string{
		"History",
		"  2026-07-29 15:00 UTC  note by brandon/a1",
		"    Repros only under -race.",
		"",
	}, "\n")
	if v := b.String(); !strings.HasSuffix(v, wantSingle) || strings.Contains(v, "\n\n  2026") {
		t.Errorf("single-entry history grew a stray blank line.\ngot:\n%s", v)
	}
}

// Task edits render in history (2026-08-11 grill): every task.updated
// is an entry — dim stamp, bold "edit by <actor>" descriptor, and the
// daemon's compact per-field summaries verbatim on the detail line;
// a multi-field edit is one entry. Both surfaces share historyOf, so
// this pins the shape once at the source.
func TestPrintTaskHistoryUpdateEntry(t *testing.T) {
	h := hydratedTask{
		Task: taskJSON{ID: "t-hist", Title: "the biography"},
		Updates: []updateJSON{{
			ID: "01U1", Task: "t-hist", Actor: "brandon",
			Fields: []string{"retitled", "priority 0→2", "labels +launch −web"},
		}},
	}
	var b strings.Builder
	printTask(&b, colors{}, h, stateTask{}, nil)
	wantHist := strings.Join([]string{
		"History",
		"  (unknown time)  edit by brandon",
		"    retitled · priority 0→2 · labels +launch −web",
		"",
	}, "\n")
	if v := b.String(); !strings.HasSuffix(v, wantHist) {
		t.Errorf("update entry diverged.\nwant suffix:\n%s\ngot:\n%s", wantHist, v)
	}

	// With real colors: dim stamp, bold descriptor, plain summary.
	b.Reset()
	printTask(&b, ansiColors, h, stateTask{}, nil)
	want := "  \x1b[90m(unknown time)\x1b[0m  \x1b[1medit by brandon\x1b[0m\n" +
		"    retitled · priority 0→2 · labels +launch −web\n"
	if v := b.String(); !strings.Contains(v, want) {
		t.Errorf("colored update entry missing %q; output:\n%q", want, v)
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

// Armed task view of a task with an open escalation, opened from its
// task row: the plain open focuses the title, the ring walks down to
// the escalation, and enter answers it in place — same API call and
// refresh as any steering write — and the flow returns to the task
// view.
func TestTopDetailAnswerFlow(t *testing.T) {
	fake := newFakeSteering()
	m := openDetail(t, newTopModelWithDep(fake), "t-lic")
	v := m.View()
	if !strings.Contains(v, "NEEDS INPUT (1)") {
		t.Fatalf("open escalation section missing; view:\n%s", v)
	}
	// A plain open focuses the title, not the escalation (routed opens
	// preselect it — TestTopAnswerFlow).
	if !strings.Contains(v, "▌ t-lic — choose a license") {
		t.Fatalf("plain open did not focus the title; view:\n%s", v)
	}
	// The footer legend advertises the ring; "enter answer" rides the
	// NEEDS INPUT bar itself (edit affordance, 2026-08-01).
	if !strings.Contains(v, "↑/↓ (j/k) move · enter edit · p priority · c cancel · esc back · q quit") {
		t.Errorf("armed detail footer wrong; view:\n%s", v)
	}
	// Three stops down the ring — past priority and labels — is the
	// escalation.
	m, _ = press(t, m,
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if v := m.View(); !strings.Contains(v, "▌ !   Which license?") {
		t.Fatalf("escalation not focused after walking the ring; view:\n%s", v)
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
	// Success says nothing (feedback only, 2026-08-21): the refresh
	// below moving the answer into HISTORY is the confirmation.
	if m.status != "" {
		t.Errorf("status = %q, want empty after success", m.status)
	}

	// The refresh that lands the answer removes the selectable row: the
	// section is gone, the answer shows in HISTORY, and the focus clamps
	// onto the last stop — the description — so enter now opens its
	// editor: the ring never dangles.
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
	if strings.Contains(v, "NEEDS INPUT") {
		t.Errorf("answered escalation still renders a section; view:\n%s", v)
	}
	if !strings.Contains(v, "A (brandon): Use MIT.") {
		t.Errorf("answer missing from HISTORY after the refresh; view:\n%s", v)
	}
	if !strings.Contains(v, "↑/↓ (j/k) move · enter edit · p priority · c cancel · esc back · q quit") {
		t.Errorf("footer lost the armed ring legend; view:\n%s", v)
	}
	m, cmd = press(t, m, keyOf(tea.KeyEnter))
	if m.mode != modeEditDesc || cmd != nil {
		t.Errorf("enter after the ring shrank: mode %d cmd %v, want the description editor (focus clamps to the last stop)", m.mode, cmd)
	}
}

// esc from an input mode opened in detail returns to that detail, not
// the list; esc again reaches the list.
func TestTopDetailInputEscReturnsToDetail(t *testing.T) {
	fake := newFakeSteering()
	m := openDetail(t, newTopModelWithDep(fake), "t-lic")
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
	m := openDetail(t, newTopModelWithDep(fake), "t-lic")
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

// c in an armed detail cancels the viewed task behind the same y/n
// confirm as the list; the detail survives the cancel, showing the
// task's new cancelled status once the refresh lands.
func TestTopDetailCancelFlow(t *testing.T) {
	fake := newFakeSteering()
	m := openDetail(t, newTopModelWithDep(fake), "t-lic")
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if m.mode != modeConfirmCancel {
		t.Fatalf("c in detail: mode %d, want modeConfirmCancel", m.mode)
	}
	if v := m.View(); !strings.Contains(v, "cancel t-lic (choose a license)? y/n") {
		t.Errorf("confirm prompt does not name the viewed task; view:\n%s", v)
	}
	// n backs out to the detail without a call.
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if m.mode != modeDetail || len(fake.cancelled) != 0 {
		t.Fatalf("n did not back out to detail: mode %d cancelled %v", m.mode, fake.cancelled)
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
	if len(fake.cancelled) != 1 || fake.cancelled[0] != "t-lic" {
		t.Errorf("cancelled %v, want [t-lic]", fake.cancelled)
	}
	mm, _ := m.Update(am)
	m = mm.(topModel)
	if m.mode != modeDetail {
		t.Fatalf("cancel knocked the model out of detail: mode %d", m.mode)
	}
	// Success says nothing (feedback only, 2026-08-21): the status word
	// changing on the refresh below is the confirmation.
	if m.status != "" {
		t.Errorf("status = %q, want empty after success", m.status)
	}
	// The refresh lands the cancel: the task stays viewable — history
	// stays on the ledger — with the stored status word.
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
		t.Fatalf("refresh after cancel left mode %d detail %q", m.mode, m.detailID)
	}
	if v := m.View(); !strings.Contains(v, "status      cancelled") {
		t.Errorf("cancelled task's detail missing its new status; view:\n%s", v)
	}
}

// enter on the focused title stop (where a plain open lands) edits the
// viewed task's title in the widget's single-line mode, prefilled with
// the current title (task-view edits, 2026-08-01; ring, 2026-08-02):
// submit goes through the same task.updated plumbing as `tuhdoo update
// --title`, and the refreshed snapshot re-renders the view with the
// new content.
func TestTopDetailEditTitleFlow(t *testing.T) {
	fake := newFakeSteering()
	m := openDetail(t, newTopModel(fake), "t-flak")
	m, _ = press(t, m, keyOf(tea.KeyEnter))
	if m.mode != modeEditTitle {
		t.Fatalf("enter on the focused title: mode %d, want modeEditTitle", m.mode)
	}
	// Prefilled for editing, not retyping: the current title, cursor at
	// the end, single-line.
	if got := m.input.String(); got != "investigate the flake" || m.input.multiline {
		t.Fatalf("title editor opened with %q (multiline %v), want the current title single-line",
			got, m.input.multiline)
	}
	if m.input.cursor != len([]rune("investigate the flake")) {
		t.Fatalf("cursor at %d, want the end of the prefilled title", m.input.cursor)
	}
	v := m.View()
	for _, want := range []string{
		"title t-flak",
		"> investigate the flake█",
		"enter saves · esc cancels",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("title editor missing %q; view:\n%s", want, v)
		}
	}
	// Edit the tail word and submit.
	m, _ = press(t, m, keyOf(tea.KeyCtrlW))
	m, cmd := press(t, m, append(runes("flakiness"), keyOf(tea.KeyEnter))...)
	if m.mode != modeDetail || m.detailID != "t-flak" {
		t.Errorf("submit did not return to detail: mode %d detail %q", m.mode, m.detailID)
	}
	if cmd == nil {
		t.Fatal("submit produced no command")
	}
	am := cmd().(actionMsg)
	if am.err != nil {
		t.Fatalf("action error: %v", am.err)
	}
	if got := fake.titles["t-flak"]; got != "investigate the flakiness" {
		t.Errorf("title written as %q, want %q", got, "investigate the flakiness")
	}
	mm, _ := m.Update(am)
	m = mm.(topModel)
	if m.status != "" {
		t.Errorf("status = %q, want empty after success (feedback only, 2026-08-21)", m.status)
	}
	// The refresh lands the edit: the view re-renders with the new title.
	fresh := topSnapshot()
	for i := range fresh.state.Tasks {
		if fresh.state.Tasks[i].ID == "t-flak" {
			fresh.state.Tasks[i].Title = "investigate the flakiness"
		}
	}
	h := fresh.tasks["t-flak"]
	h.Task.Title = "investigate the flakiness"
	fresh.tasks["t-flak"] = h
	mm, _ = m.Update(snapMsg{snap: fresh})
	m = mm.(topModel)
	if v := m.View(); !strings.Contains(v, "t-flak — investigate the flakiness") {
		t.Errorf("edited title missing after refresh; view:\n%s", v)
	}
}

// The title editor's guard rails: an unchanged submit writes nothing,
// an emptied title is rejected in place, and esc cancels without a
// write.
func TestTopDetailEditTitleUnchangedEmptyAndEsc(t *testing.T) {
	fake := newFakeSteering()
	m := openDetail(t, newTopModel(fake), "t-flak")
	// Unchanged submit: straight back to the detail, no write, no error.
	// (The plain open leaves the title focused, so enter opens its editor.)
	m, cmd := press(t, m,
		keyOf(tea.KeyEnter),
		keyOf(tea.KeyEnter))
	if m.mode != modeDetail || cmd != nil || len(fake.titles) != 0 {
		t.Fatalf("unchanged submit: mode %d cmd %v titles %v, want a silent close", m.mode, cmd, fake.titles)
	}
	// Emptied title: rejected in place, prompt stays up.
	m, cmd = press(t, m,
		keyOf(tea.KeyEnter),
		keyOf(tea.KeyCtrlU), // cursor at end: kills the whole title
		keyOf(tea.KeyEnter))
	if cmd != nil || m.mode != modeEditTitle || m.status != "title cannot be empty" {
		t.Fatalf("empty title: mode %d status %q cmd %v, want in-place rejection", m.mode, m.status, cmd)
	}
	// esc abandons the edit without a write.
	m, _ = press(t, m, append(runes("junk"), keyOf(tea.KeyEsc))...)
	if m.mode != modeDetail || len(fake.titles) != 0 {
		t.Errorf("esc did not abandon the edit: mode %d titles %v", m.mode, fake.titles)
	}
}

// enter on the focused description stop edits it in the widget's
// multi-line mode — its first real consumer: prefilled, enter inserts
// a newline, ctrl+s submits through the update plumbing. With no open
// escalations the description is three ring stops below the title.
func TestTopDetailEditDescriptionFlow(t *testing.T) {
	fake := newFakeSteering()
	m := openDetail(t, newTopModel(fake), "t-flak")
	m, _ = press(t, m,
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}, // priority
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}, // labels
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}, // description
		keyOf(tea.KeyEnter))
	if m.mode != modeEditDesc {
		t.Fatalf("enter on the focused description: mode %d, want modeEditDesc", m.mode)
	}
	if got := m.input.String(); got != "The parser test flakes on CI.\nFind out why." || !m.input.multiline {
		t.Fatalf("description editor opened with %q (multiline %v), want the current description multi-line",
			got, m.input.multiline)
	}
	v := m.View()
	for _, want := range []string{
		"description t-flak (investigate the flake)",
		"> The parser test flakes on CI.",
		"> Find out why.█",
		"ctrl+s saves · enter newline · esc cancels",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("description editor missing %q; view:\n%s", want, v)
		}
	}
	// enter is an edit here — a newline, never a submit.
	m, cmd := press(t, m, keyOf(tea.KeyEnter))
	if cmd != nil || m.mode != modeEditDesc {
		t.Fatalf("enter submitted a multi-line edit: mode %d cmd %v", m.mode, cmd)
	}
	m, cmd = press(t, m, append(runes("Fix it."), keyOf(tea.KeyCtrlS))...)
	if m.mode != modeDetail || m.detailID != "t-flak" {
		t.Errorf("ctrl+s did not return to detail: mode %d detail %q", m.mode, m.detailID)
	}
	if cmd == nil {
		t.Fatal("ctrl+s produced no command")
	}
	am := cmd().(actionMsg)
	if am.err != nil {
		t.Fatalf("action error: %v", am.err)
	}
	want := "The parser test flakes on CI.\nFind out why.\nFix it."
	if got := fake.descs["t-flak"]; got != want {
		t.Errorf("description written as %q, want %q", got, want)
	}
	mm, _ := m.Update(am)
	m = mm.(topModel)
	if m.status != "" {
		t.Errorf("status = %q, want empty after success (feedback only, 2026-08-21)", m.status)
	}
	// The refresh lands the edit: the DESCRIPTION section re-renders.
	fresh := topSnapshot()
	h := fresh.tasks["t-flak"]
	h.Task.Description = want
	fresh.tasks["t-flak"] = h
	mm, _ = m.Update(snapMsg{snap: fresh})
	m = mm.(topModel)
	if v := m.View(); !strings.Contains(v, "Fix it.") {
		t.Errorf("edited description missing after refresh; view:\n%s", v)
	}
}

// The description editor writes nothing on an unchanged submit or esc;
// a task with no description renders the focusable dim placeholder,
// opens an empty multi-line editor, and a first description saves
// through the same path.
func TestTopDetailEditDescriptionUnchangedEscAndFirstWrite(t *testing.T) {
	fake := newFakeSteering()
	m := openDetail(t, newTopModel(fake), "t-flak")
	m, cmd := press(t, m,
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
		keyOf(tea.KeyEnter),
		keyOf(tea.KeyCtrlS))
	if m.mode != modeDetail || cmd != nil || len(fake.descs) != 0 {
		t.Fatalf("unchanged submit: mode %d cmd %v descs %v, want a silent close", m.mode, cmd, fake.descs)
	}
	// The submit returned to the detail with the description still
	// focused, so enter alone reopens it.
	m, _ = press(t, m, keyOf(tea.KeyEnter))
	m, _ = press(t, m, append(runes("scribble"), keyOf(tea.KeyEsc))...)
	if m.mode != modeDetail || len(fake.descs) != 0 {
		t.Fatalf("esc did not abandon the edit: mode %d descs %v", m.mode, fake.descs)
	}
	// A description-less task (t-pars): the dim "none" placeholder is a
	// real ring stop, the editor opens empty, and a typed description is
	// a real change. Four stops down: priority, labels, the t-chor dep
	// row (edge rows, 2026-08-11), then the description.
	m = openDetail(t, newTopModel(fake), "t-pars")
	m, _ = press(t, m,
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if v := m.View(); !strings.Contains(v, "▌ none") {
		t.Fatalf("empty-description placeholder not focused; view:\n%s", v)
	}
	m, _ = press(t, m, keyOf(tea.KeyEnter))
	if got := m.input.String(); got != "" {
		t.Fatalf("editor for a description-less task opened with %q", got)
	}
	m, cmd = press(t, m, append(runes("Parse the event log."), keyOf(tea.KeyCtrlS))...)
	if cmd == nil {
		t.Fatal("ctrl+s produced no command")
	}
	if am := cmd().(actionMsg); am.err != nil {
		t.Fatalf("action error: %v", am.err)
	}
	if got := fake.descs["t-pars"]; got != "Parse the event log." {
		t.Errorf("first description written as %q", got)
	}
}

// enter on the focused labels stop edits the list as one comma-joined
// line (labels editable, 2026-08-05): prefilled "tui, design", parsed
// with splitList on submit — split on commas, trim, drop empties, no
// dedup, no case-folding: what was typed is what is stored.
func TestTopDetailEditLabelsFlow(t *testing.T) {
	fake := newFakeSteering()
	m := newTopModel(fake)
	h := m.snap.tasks["t-flak"]
	h.Task.Labels = []string{"tui", "design"}
	m.snap.tasks["t-flak"] = h
	m = openDetail(t, m, "t-flak")
	// The meta line renders the stored list, comma-joined.
	if v := m.View(); !strings.Contains(v, "labels      tui, design") {
		t.Fatalf("labels line missing; view:\n%s", v)
	}
	// Two stops down the ring — past priority — is labels.
	m, _ = press(t, m,
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
		keyOf(tea.KeyEnter))
	if m.mode != modeEditLabels {
		t.Fatalf("enter on the focused labels: mode %d, want modeEditLabels", m.mode)
	}
	if got := m.input.String(); got != "tui, design" || m.input.multiline {
		t.Fatalf("labels editor opened with %q (multiline %v), want the comma-joined list single-line",
			got, m.input.multiline)
	}
	v := m.View()
	for _, want := range []string{
		"labels t-flak (investigate the flake)",
		"> tui, design█",
		"enter saves · esc cancels",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("labels editor missing %q; view:\n%s", want, v)
		}
	}
	// A real edit — append one label, sloppy spacing — writes the parsed
	// list: split on commas, trimmed, order kept.
	m, cmd := press(t, m, append(runes(",  ops"), keyOf(tea.KeyEnter))...)
	if m.mode != modeDetail || m.detailID != "t-flak" {
		t.Errorf("submit did not return to detail: mode %d detail %q", m.mode, m.detailID)
	}
	if cmd == nil {
		t.Fatal("submit produced no command")
	}
	am := cmd().(actionMsg)
	if am.err != nil {
		t.Fatalf("action error: %v", am.err)
	}
	if got := fake.labels["t-flak"]; !slices.Equal(got, []string{"tui", "design", "ops"}) {
		t.Errorf("labels written as %v, want [tui design ops]", got)
	}
	mm, _ := m.Update(am)
	m = mm.(topModel)
	if m.status != "" {
		t.Errorf("status = %q, want empty after success (feedback only, 2026-08-21)", m.status)
	}
	// The refresh lands the edit: the meta line re-renders the new list.
	fresh := topSnapshot()
	fh := fresh.tasks["t-flak"]
	fh.Task.Labels = []string{"tui", "design", "ops"}
	fresh.tasks["t-flak"] = fh
	mm, _ = m.Update(snapMsg{snap: fresh})
	m = mm.(topModel)
	if v := m.View(); !strings.Contains(v, "labels      tui, design, ops") {
		t.Errorf("edited labels missing after refresh; view:\n%s", v)
	}
}

// The labels editor's guard rails (labels editable, 2026-08-05): an
// unchanged submit writes nothing (the raw editWas guard), a respaced
// submit parses element-wise equal and writes nothing, esc abandons
// without a write, an emptied submit is a real write that clears every
// label (the CLI's explicit-empty --labels precedent), and reordering
// is a real edit — the comparison is order-sensitive.
func TestTopDetailEditLabelsUnchangedRespacedClearAndEsc(t *testing.T) {
	fake := newFakeSteering()
	m := newTopModel(fake)
	h := m.snap.tasks["t-flak"]
	h.Task.Labels = []string{"tui", "design"}
	m.snap.tasks["t-flak"] = h
	m = openDetail(t, m, "t-flak")
	// Unchanged submit: straight back to the detail, no write. (The
	// submit keeps labels focused, so enter alone reopens the editor
	// for the cases below.)
	m, cmd := press(t, m,
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
		keyOf(tea.KeyEnter),
		keyOf(tea.KeyEnter))
	if m.mode != modeDetail || cmd != nil || len(fake.labels) != 0 {
		t.Fatalf("unchanged submit: mode %d cmd %v labels %v, want a silent close", m.mode, cmd, fake.labels)
	}
	// Respaced: different bytes, the same parsed list — no write.
	m, cmd = press(t, m, append([]tea.KeyMsg{keyOf(tea.KeyEnter), keyOf(tea.KeyCtrlU)},
		append(runes("tui,design"), keyOf(tea.KeyEnter))...)...)
	if m.mode != modeDetail || cmd != nil || len(fake.labels) != 0 {
		t.Fatalf("respaced submit: mode %d cmd %v labels %v, want a silent close", m.mode, cmd, fake.labels)
	}
	// esc abandons a real edit without a write.
	m, _ = press(t, m, keyOf(tea.KeyEnter))
	m, _ = press(t, m, append(runes("junk"), keyOf(tea.KeyEsc))...)
	if m.mode != modeDetail || len(fake.labels) != 0 {
		t.Fatalf("esc did not abandon the edit: mode %d labels %v", m.mode, fake.labels)
	}
	// An emptied submit clears: a real write of the empty list.
	m, cmd = press(t, m, keyOf(tea.KeyEnter), keyOf(tea.KeyCtrlU), keyOf(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("empty submit produced no command")
	}
	if am := cmd().(actionMsg); am.err != nil {
		t.Fatalf("action error: %v", am.err)
	}
	if got, ok := fake.labels["t-flak"]; !ok || len(got) != 0 {
		t.Errorf("empty submit wrote %v (present %v), want the explicit empty list", got, ok)
	}
	// Reordering the same two labels is a real edit. (The snapshot has
	// not refreshed, so the editor still prefills "tui, design".)
	m, cmd = press(t, m, append([]tea.KeyMsg{keyOf(tea.KeyEnter), keyOf(tea.KeyCtrlU)},
		append(runes("design, tui"), keyOf(tea.KeyEnter))...)...)
	if cmd == nil {
		t.Fatal("reorder submit produced no command")
	}
	if am := cmd().(actionMsg); am.err != nil {
		t.Fatalf("action error: %v", am.err)
	}
	if got := fake.labels["t-flak"]; !slices.Equal(got, []string{"design", "tui"}) {
		t.Errorf("reorder written as %v, want [design tui]", got)
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
			// The holder keeps t-two an in-progress task row: with the
			// blocking escalation its single home in Needs Input
			// (2026-07-31), an unclaimed escalation-blocked task has no
			// task row to open detail from.
			Tasks: []stateTask{{ID: "t-two", Title: "twice escalated", Status: "open", Holder: "brandon/a1",
				Situation: "in_progress", BlockingEscalations: []string{"01E1"}}},
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

// The focus/scroll rule over the full ring: j/k move focus when a
// further stop exists in that direction — scrolling just enough to
// reveal it — and scroll one line otherwise. Traversal order is render
// order: title, priority, labels, each open escalation, description.
func TestTopDetailRingTraversalAndReveal(t *testing.T) {
	s := multiEscSnapshot()
	m := topModel{api: newFakeSteering(), actor: "brandon", armed: true, snap: s, rows: buildRows(s)}
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
	m = mm.(topModel)
	m = openDetail(t, m, "t-two")
	// A plain open focuses the title at the top of the view.
	if m.detailFocus != 0 || m.detailScroll != 0 {
		t.Fatalf("plain open: focus %d scroll %d, want the title at top", m.detailFocus, m.detailScroll)
	}
	body := strings.Join(m.detailBody(), "\n")
	if !strings.Contains(body, "▌ t-two — twice escalated") {
		t.Fatalf("title not selected on a plain open; body:\n%s", body)
	}
	inWindow := func(m topModel) bool {
		l := m.detailFocusLine()
		return l >= m.detailScroll && l < m.detailScroll+m.detailWindow()
	}
	// j walks the ring in render order, revealing each stop; the
	// previous stop hands the bar off.
	walk := []struct{ now, previous string }{
		{"▌ priority    none", "▌ t-two — twice escalated"},
		{"▌ labels      none", "▌ priority    none"},
		{"▌ !   First question?", "▌ labels      none"},
		{"▌     Second question?", "▌ !   First question?"},
		{"▌ A line of description.", "▌     Second question?"},
	}
	for i, w := range walk {
		m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		if m.detailFocus != i+1 {
			t.Fatalf("j #%d moved focus to %d, want %d", i+1, m.detailFocus, i+1)
		}
		body := strings.Join(m.detailBody(), "\n")
		if !strings.Contains(body, w.now) || strings.Contains(body, w.previous) {
			t.Fatalf("j #%d: want %q selected and %q released; body:\n%s", i+1, w.now, w.previous, body)
		}
		if !inWindow(m) {
			t.Errorf("j #%d: focused stop not revealed: line %d scroll %d window %d",
				i+1, m.detailFocusLine(), m.detailScroll, m.detailWindow())
		}
	}
	// enter answers the focused escalation, and esc keeps the focus.
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m, _ = press(t, m, keyOf(tea.KeyEnter))
	if m.mode != modeAnswer || m.target.esc.ID != "01E2" {
		t.Fatalf("enter on the second escalation: mode %d target %q, want modeAnswer 01E2", m.mode, m.target.esc.ID)
	}
	m, _ = press(t, m, keyOf(tea.KeyEsc))
	if m.mode != modeDetail || m.detailFocus != 4 {
		t.Fatalf("esc lost the detail focus: mode %d focus %d", m.mode, m.detailFocus)
	}
	// j below the description — the last stop — line-scrolls.
	m, _ = press(t, m,
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}) // back to the description
	before := m.detailScroll
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.detailFocus != 5 || m.detailScroll != before+1 {
		t.Errorf("j at the last stop: focus %d scroll %d, want 5 %d (line-scroll fallback)",
			m.detailFocus, m.detailScroll, before+1)
	}
	// k walks back to the title, revealing it at the top…
	for i := 0; i < 5; i++ {
		m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	}
	if m.detailFocus != 0 || m.detailScroll != 0 {
		t.Fatalf("k back to the title: focus %d scroll %d, want 0 0", m.detailFocus, m.detailScroll)
	}
	// …and above the title — with the window wheeled down off the top —
	// k line-scrolls back up instead of moving focus.
	m, _ = mouseTo(t, m, wheelMsg(tea.MouseButtonWheelDown), wheelMsg(tea.MouseButtonWheelDown))
	if m.detailScroll != 2 {
		t.Fatalf("wheel setup: scroll %d, want 2", m.detailScroll)
	}
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.detailFocus != 0 || m.detailScroll != 1 {
		t.Errorf("k at the first stop: focus %d scroll %d, want 0 1 (line-scroll fallback)",
			m.detailFocus, m.detailScroll)
	}
}

// enter opens the focused stop's editor — the right prefilled mode for
// every stop kind, walked in one pass down the ring.
func TestTopDetailEnterOpensEditorPerStop(t *testing.T) {
	s := multiEscSnapshot()
	m := topModel{api: newFakeSteering(), actor: "brandon", armed: true, snap: s, rows: buildRows(s)}
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	m = mm.(topModel)
	m = openDetail(t, m, "t-two")
	// Title: single-line, prefilled.
	m, _ = press(t, m, keyOf(tea.KeyEnter))
	if m.mode != modeEditTitle || m.input.String() != "twice escalated" || m.input.multiline {
		t.Fatalf("title stop: mode %d input %q multiline %v, want the prefilled single-line editor",
			m.mode, m.input, m.input.multiline)
	}
	m, _ = press(t, m, keyOf(tea.KeyEsc))
	// Priority: the numeric input, targeting the viewed task.
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}, keyOf(tea.KeyEnter))
	if m.mode != modePriority || m.target.task.ID != "t-two" {
		t.Fatalf("priority stop: mode %d target %q, want modePriority on t-two", m.mode, m.target.task.ID)
	}
	if v := m.View(); !strings.Contains(v, "priority t-two (twice escalated)") {
		t.Errorf("priority prompt does not name the viewed task; view:\n%s", v)
	}
	m, _ = press(t, m, keyOf(tea.KeyEsc))
	// Labels: single-line, prefilled with the comma-joined list — empty
	// here, t-two has none (prefill with values: TestTopDetailEditLabelsFlow).
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}, keyOf(tea.KeyEnter))
	if m.mode != modeEditLabels || m.input.String() != "" || m.input.multiline {
		t.Fatalf("labels stop: mode %d input %q multiline %v, want the empty single-line editor",
			m.mode, m.input, m.input.multiline)
	}
	if v := m.View(); !strings.Contains(v, "labels t-two (twice escalated)") {
		t.Errorf("labels prompt does not name the viewed task; view:\n%s", v)
	}
	m, _ = press(t, m, keyOf(tea.KeyEsc))
	// Each escalation: answer entry on its own question.
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}, keyOf(tea.KeyEnter))
	if m.mode != modeAnswer || m.target.esc.ID != "01E1" {
		t.Fatalf("first escalation stop: mode %d target %q, want modeAnswer 01E1", m.mode, m.target.esc.ID)
	}
	m, _ = press(t, m, keyOf(tea.KeyEsc))
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}, keyOf(tea.KeyEnter))
	if m.mode != modeAnswer || m.target.esc.ID != "01E2" {
		t.Fatalf("second escalation stop: mode %d target %q, want modeAnswer 01E2", m.mode, m.target.esc.ID)
	}
	m, _ = press(t, m, keyOf(tea.KeyEsc))
	// Description: multi-line, prefilled.
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}, keyOf(tea.KeyEnter))
	if m.mode != modeEditDesc || !m.input.multiline ||
		!strings.HasPrefix(m.input.String(), "A line of description.") {
		t.Fatalf("description stop: mode %d multiline %v input %q, want the prefilled multi-line editor",
			m.mode, m.input.multiline, m.input)
	}
}

// Wrap-then-indent (2026-08-02): the title and description blocks wrap
// to the mark column's inner width first, then every screen line sits
// on the column — so a focused block's gutter is continuous across
// wrapped continuations and blank paragraph separators, instead of
// breaking wherever a logical line wrapped.
func TestTopDetailFocusedBlockGutterContinuous(t *testing.T) {
	s := topSnapshot()
	h := s.tasks["t-pars"]
	h.Task.Title = "write the parser for the event log so replay stays deterministic end to end"
	h.Task.Description = "Context: this first paragraph is long enough to wrap at eighty columns because it keeps going with detail.\n\n" +
		"The ask: a second paragraph, also long enough to wrap at the standard eighty column width."
	s.tasks["t-pars"] = h
	m := topModel{api: newFakeSteering(), actor: "brandon", armed: true, snap: s, rows: buildRows(s)}
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	m = mm.(topModel)
	m = openDetail(t, m, "t-pars")
	// The focused title wraps; both of its screen lines carry the gutter.
	body := m.detailBody()
	if !strings.HasPrefix(body[0], "▌ t-pars — ") || !strings.HasPrefix(body[1], "▌ ") {
		t.Fatalf("wrapped title not continuously guttered:\n%q\n%q", body[0], body[1])
	}
	if body[2] != "" {
		t.Fatalf("title block should be two wrapped lines; line 2 = %q", body[2])
	}
	// Focus the description — four stops down, past priority, labels,
	// and the t-chor dep row (edge rows, 2026-08-11): every line of the
	// block — continuations and the blank paragraph separator included —
	// carries the gutter.
	m, _ = press(t, m,
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	body = m.detailBody()
	start := -1
	for i, l := range body {
		if strings.Contains(l, "DESCRIPTION") {
			start = i + 1
			break
		}
	}
	if start < 0 {
		t.Fatalf("no DESCRIPTION bar; body:\n%s", strings.Join(body, "\n"))
	}
	end := start
	for end < len(body) && body[end] != "" { // the chrome blank before HISTORY
		end++
	}
	if end-start < 4 {
		t.Fatalf("description block too short to prove wrapping: %d lines", end-start)
	}
	blank := false
	for _, l := range body[start:end] {
		if !strings.HasPrefix(l, "▌ ") {
			t.Errorf("description line lost the gutter: %q", l)
		}
		if l == "▌ " {
			blank = true
		}
	}
	if !blank {
		t.Error("no guttered blank separator inside the description block")
	}
	if w := maxLineWidth(m.View()); w > 80 {
		t.Errorf("line wider than terminal: %d > 80", w)
	}
}

// The e/E edit chords are retired (the ring replaced them, 2026-08-02):
// E stays a dead key, and e — rebound as the context toggle (escalation
// readability, 2026-08-11) — is inert in an armed view unless an
// escalation stop is focused: it never opens an editor, never leaves
// the view. t-flak has no open escalation, so both keys change nothing.
func TestTopDetailEAndEUnbound(t *testing.T) {
	m := openDetail(t, newTopModel(newFakeSteering()), "t-flak")
	v := m.View()
	for _, r := range []rune{'e', 'E'} {
		mm, cmd := press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		if mm.mode != modeDetail || cmd != nil {
			t.Errorf("key %q: mode %d cmd %v, want modeDetail nil", r, mm.mode, cmd)
		}
		if mm.View() != v {
			t.Errorf("key %q changed the rendered view of a task with no open escalation", r)
		}
	}
}

// structuredEscSnapshot seeds one task with two open escalations
// (escalation readability, 2026-08-11): 01E8 carries a multi-line
// question — the structure the agent wrote — and a 13-line context;
// 01E9 a one-line question with a one-line context. Ring order: title,
// priority, labels, 01E8 (stop 3), 01E9 (stop 4), description.
func structuredEscSnapshot() *snapshot {
	raised := time.Date(2026, 8, 10, 9, 0, 0, 0, time.UTC)
	ctx := make([]string, 13)
	for i := range ctx {
		ctx[i] = fmt.Sprintf("context line %d", i+1)
	}
	e1 := escalationJSON{
		ID: "01E8", Task: "t-big", Actor: "brandon/a3",
		Question: "Pick a path.\nA) fork the library\nB) vendor the patch",
		Context:  strings.Join(ctx, "\n"),
		Blocking: true, RaisedAt: raised,
	}
	e2 := escalationJSON{
		ID: "01E9", Task: "t-big", Actor: "brandon/a3",
		Question: "Short one?", Context: "one line only", RaisedAt: raised,
	}
	return &snapshot{
		state: stateResp{
			Tasks: []stateTask{{ID: "t-big", Title: "structured ask", Status: "open", Holder: "brandon/a3",
				Situation: "in_progress", BlockingEscalations: []string{"01E8"}}},
			OpenEscalations: []escalationJSON{e1, e2},
		},
		tasks: map[string]hydratedTask{
			"t-big": {Task: taskJSON{ID: "t-big", Title: "structured ask"},
				Escalations: []escalationJSON{e1, e2}},
		},
	}
}

// gutterLines are the rendered body lines carrying the selection bar.
func gutterLines(body []string) []string {
	var out []string
	for _, l := range body {
		if strings.HasPrefix(l, "▌") {
			out = append(out, l)
		}
	}
	return out
}

// The escalation block's rework (escalation readability, 2026-08-11):
// the question keeps its line structure instead of being one-lined; the
// context collapses to its first line plus an accurate hidden-line
// count; e toggles the focused escalation's context open and closed —
// with the selection bar covering the whole block in both shapes, and
// the focus ring intact across the height change. A context that fits
// one line renders as-is, stub-free. The toggle state is view-local:
// closing the view resets every context to the collapsed stub.
func TestTopDetailEscalationStructureAndContextToggle(t *testing.T) {
	s := structuredEscSnapshot()
	m := topModel{api: newFakeSteering(), actor: "brandon", armed: true, snap: s, rows: buildRows(s)}
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	m = mm.(topModel)
	m, _ = press(t, m, keyOf(tea.KeyEnter)) // first Needs Input row → t-big, 01E8 preselected
	if m.mode != modeDetail || m.detailID != "t-big" || m.detailFocus != 3 {
		t.Fatalf("enter on the Needs Input row: mode %d detail %q focus %d, want t-big with 01E8 preselected at 3",
			m.mode, m.detailID, m.detailFocus)
	}
	body := m.detailBody()
	joined := strings.Join(body, "\n")
	// The question renders with its structure, every line selected.
	mustContain(t, joined,
		"▌ !   Pick a path.",
		"▌     A) fork the library",
		"▌     B) vendor the patch")
	// The context is a collapsed stub: first line plus the hidden count.
	mustContain(t, joined, "▌     context line 1", "▌     (+12 lines — e to expand)")
	if strings.Contains(joined, "context line 2") {
		t.Fatalf("collapsed context leaked past its first line; body:\n%s", joined)
	}
	// The one-line context renders as-is: exactly one stub on screen.
	mustContain(t, joined, "one line only")
	if strings.Count(joined, "e to expand") != 1 {
		t.Errorf("want exactly one collapsed stub; body:\n%s", joined)
	}
	// The section bar advertises both section keys.
	mustContain(t, m.View(), "enter answer · e context")
	// The selection bar covers the whole collapsed block: three question
	// lines, the two-line context stub, the meta line.
	if g := gutterLines(body); len(g) != 6 {
		t.Fatalf("collapsed block has %d gutter lines, want 6:\n%s", len(g), strings.Join(g, "\n"))
	}
	collapsedLen := len(body)
	// e expands the focused context in full; the bar covers the taller
	// shape, and the view grows by exactly the hidden lines.
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	body = m.detailBody()
	joined = strings.Join(body, "\n")
	mustContain(t, joined, "▌     context line 2", "▌     context line 13")
	if strings.Contains(joined, "e to expand") {
		t.Fatalf("stub still renders while expanded; body:\n%s", joined)
	}
	if g := gutterLines(body); len(g) != 17 { // 3 question + 13 context + meta
		t.Fatalf("expanded block has %d gutter lines, want 17:\n%s", len(g), strings.Join(g, "\n"))
	}
	if len(body) != collapsedLen+11 { // 13 lines where the 2-line stub sat
		t.Errorf("expanding grew the body by %d lines, want 11", len(body)-collapsedLen)
	}
	// The height change feeds the same lines selection uses: a click on
	// the last context line still hits the escalation stop.
	if i := m.detailStopAt(screenLineOf(t, m, "context line 13")); i != 3 {
		t.Errorf("click on an expanded context line maps to stop %d, want 3", i)
	}
	// The ring still walks over the taller block: j to 01E9, k back —
	// the bar lands on the collapsed neighbor, then back on the tall one.
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.detailFocus != 4 {
		t.Fatalf("j off the expanded escalation: focus %d, want 4", m.detailFocus)
	}
	if joined = strings.Join(m.detailBody(), "\n"); !strings.Contains(joined, "▌     Short one?") {
		t.Fatalf("neighbor escalation not selected; body:\n%s", joined)
	}
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.detailFocus != 3 {
		t.Fatalf("k back onto the expanded escalation: focus %d, want 3", m.detailFocus)
	}
	// e again re-collapses; enter still answers the focused escalation.
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	joined = strings.Join(m.detailBody(), "\n")
	if strings.Contains(joined, "context line 2") || !strings.Contains(joined, "(+12 lines — e to expand)") {
		t.Fatalf("second e did not re-collapse; body:\n%s", joined)
	}
	m, _ = press(t, m, keyOf(tea.KeyEnter))
	if m.mode != modeAnswer || m.target.esc.ID != "01E8" {
		t.Fatalf("enter after toggling: mode %d target %q, want modeAnswer on 01E8", m.mode, m.target.esc.ID)
	}
	m, _ = press(t, m, keyOf(tea.KeyEsc))
	// Ephemeral view state: expand, close the view, reopen — collapsed.
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m, _ = press(t, m, keyOf(tea.KeyEsc))
	if m.mode != modeNav || m.escExpanded != nil {
		t.Fatalf("closing the view kept toggle state: mode %d escExpanded %v", m.mode, m.escExpanded)
	}
	m, _ = press(t, m, keyOf(tea.KeyEnter))
	if joined = strings.Join(m.detailBody(), "\n"); !strings.Contains(joined, "(+12 lines — e to expand)") {
		t.Fatalf("reopened view lost the collapsed default; body:\n%s", joined)
	}
}

// Watch mode keeps the context toggle — expansion is reading, not
// steering: with no focus ring, e flips every open escalation's
// context, and nothing is ever selectable or armed.
func TestWatchModeContextToggle(t *testing.T) {
	s := structuredEscSnapshot()
	m := topModel{snap: s, rows: buildRows(s)}
	m, _ = press(t, m, keyOf(tea.KeyEnter)) // → read-only detail of t-big
	if m.mode != modeDetail || m.detailID != "t-big" {
		t.Fatalf("mode %d detail %q, want modeDetail t-big", m.mode, m.detailID)
	}
	v := m.View()
	mustContain(t, v, "(+12 lines — e to expand)", "e context")
	if strings.Contains(v, "context line 2") || strings.Contains(v, "enter answer") {
		t.Fatalf("watch detail leaked the collapsed context or an answer hint; view:\n%s", v)
	}
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	v = m.View()
	mustContain(t, v, "context line 2", "context line 13")
	if strings.Contains(v, "e to expand") || strings.Contains(v, "▌") {
		t.Fatalf("watch expand left a stub or a selection; view:\n%s", v)
	}
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if v = m.View(); !strings.Contains(v, "(+12 lines — e to expand)") {
		t.Fatalf("second e did not re-collapse; view:\n%s", v)
	}
}

// Routing preselects the entered-from question: enter on the second
// Needs Input row of a twice-escalated task lands in its task view with
// that escalation selected, not the first.
func TestTopEnterOnNeedsInputPreselectsThatEscalation(t *testing.T) {
	s := multiEscSnapshot()
	m := topModel{api: newFakeSteering(), actor: "brandon", armed: true, snap: s, rows: buildRows(s)}
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	m = mm.(topModel)
	m, _ = press(t, m,
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}, // the second Needs Input row
		keyOf(tea.KeyEnter))
	if m.mode != modeDetail || m.detailID != "t-two" || m.detailFocus != 4 {
		t.Fatalf("enter on the second Needs Input row: mode %d detail %q focus %d, want t-two with the second escalation stop (4) focused",
			m.mode, m.detailID, m.detailFocus)
	}
	body := strings.Join(m.detailBody(), "\n")
	if !strings.Contains(body, "▌     Second question?") || strings.Contains(body, "▌ !   First question?") {
		t.Errorf("second question not the selected row; body:\n%s", body)
	}
}

// A task with two open escalations: each is answered from its task
// view, one after the other, through the ordinary select → enter →
// submit loop; the view re-renders between the two.
func TestTopTaskViewAnswersBothEscalations(t *testing.T) {
	fake := newFakeSteering()
	s := multiEscSnapshot()
	m := topModel{api: fake, actor: "brandon", armed: true, snap: s, rows: buildRows(s)}
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	m = mm.(topModel)
	// Route in from the first Needs Input row; answer the first question.
	m, _ = press(t, m, keyOf(tea.KeyEnter), keyOf(tea.KeyEnter))
	if m.mode != modeAnswer || m.target.esc.ID != "01E1" {
		t.Fatalf("mode %d target %q, want modeAnswer on 01E1", m.mode, m.target.esc.ID)
	}
	m, cmd := press(t, m, append(runes("Yes."), keyOf(tea.KeyEnter))...)
	if cmd == nil {
		t.Fatal("submit produced no command")
	}
	if am := cmd().(actionMsg); am.err != nil {
		t.Fatalf("action error: %v", am.err)
	}
	// The refresh lands the first answer: one open escalation remains,
	// and it is the selected row now.
	fresh := multiEscSnapshot()
	h := fresh.tasks["t-two"]
	e := h.Escalations[0]
	e.Answered, e.Answer, e.AnsweredBy = true, "Yes.", "brandon"
	h.Escalations = []escalationJSON{e, h.Escalations[1]}
	fresh.tasks["t-two"] = h
	fresh.state.OpenEscalations = fresh.state.OpenEscalations[1:]
	mm, _ = m.Update(snapMsg{snap: fresh})
	m = mm.(topModel)
	if m.mode != modeDetail || m.detailID != "t-two" {
		t.Fatalf("refresh knocked the model out of the task view: mode %d detail %q", m.mode, m.detailID)
	}
	v := m.View()
	if !strings.Contains(v, "NEEDS INPUT (1)") || !strings.Contains(v, "▌     Second question?") {
		t.Fatalf("remaining question not re-rendered as the selected row; view:\n%s", v)
	}
	// Answer the second from the same screen.
	m, _ = press(t, m, keyOf(tea.KeyEnter))
	if m.mode != modeAnswer || m.target.esc.ID != "01E2" {
		t.Fatalf("mode %d target %q, want modeAnswer on 01E2", m.mode, m.target.esc.ID)
	}
	m, cmd = press(t, m, append(runes("Do both."), keyOf(tea.KeyEnter))...)
	if cmd == nil {
		t.Fatal("submit produced no command")
	}
	if am := cmd().(actionMsg); am.err != nil {
		t.Fatalf("action error: %v", am.err)
	}
	if m.mode != modeDetail {
		t.Errorf("mode after second submit = %d, want modeDetail", m.mode)
	}
	if fake.answers["01E1"] != "Yes." || fake.answers["01E2"] != "Do both." {
		t.Errorf("answers = %v, want both questions recorded", fake.answers)
	}
}

// In the task view, a click moves the selection among stops; a click
// on the selected stop opens its editor; chrome hits nothing — the
// dashboard's click contract on the new surface.
func TestTopDetailClickSelectsAndAnswers(t *testing.T) {
	s := multiEscSnapshot()
	m := topModel{api: newFakeSteering(), actor: "brandon", armed: true, snap: s, rows: buildRows(s)}
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	m = mm.(topModel)
	m, _ = press(t, m, keyOf(tea.KeyEnter)) // task view, first question selected
	y := screenLineOf(t, m, "Second question?")
	m, cmd := mouseTo(t, m, clickAt(0, y))
	if m.mode != modeDetail || m.detailFocus != 4 || cmd != nil {
		t.Fatalf("click on the unselected question: mode %d focus %d cmd %v, want selection moved",
			m.mode, m.detailFocus, cmd)
	}
	// Chrome — the section bar — hits nothing.
	m2, _ := mouseTo(t, m, clickAt(0, screenLineOf(t, m, "NEEDS INPUT (2)")))
	if m2.mode != modeDetail || m2.detailFocus != 4 {
		t.Errorf("click on the section bar: mode %d focus %d, want unchanged", m2.mode, m2.detailFocus)
	}
	// Click again: the selected question opens answer entry.
	m, _ = mouseTo(t, m, clickAt(0, y))
	if m.mode != modeAnswer || m.target.esc.ID != "01E2" {
		t.Fatalf("click on the selected question: mode %d target %q, want modeAnswer on 01E2",
			m.mode, m.target.esc.ID)
	}
}

// Field stops answer to the mouse exactly like escalation rows: click
// selects the stop under the pointer, click on the selected stop opens
// its prefilled editor.
func TestTopDetailClickFieldStops(t *testing.T) {
	m := newTopModel(newFakeSteering())
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	m = mm.(topModel)
	m = openDetail(t, m, "t-flak")
	// Click the description body: selection moves off the title.
	y := screenLineOf(t, m, "The parser test flakes on CI.")
	m, cmd := mouseTo(t, m, clickAt(0, y))
	if m.mode != modeDetail || m.detailFocus != 3 || cmd != nil {
		t.Fatalf("click on the description: mode %d focus %d cmd %v, want the description stop selected",
			m.mode, m.detailFocus, cmd)
	}
	// Click again: the multi-line editor opens, prefilled.
	m, _ = mouseTo(t, m, clickAt(0, y))
	if m.mode != modeEditDesc || !m.input.multiline || !strings.HasPrefix(m.input.String(), "The parser test") {
		t.Fatalf("second click on the description: mode %d multiline %v input %q, want the prefilled editor",
			m.mode, m.input.multiline, m.input)
	}
	m, _ = press(t, m, keyOf(tea.KeyEsc))
	// Same double-click contract on the priority line and the title.
	y = screenLineOf(t, m, "priority")
	m, _ = mouseTo(t, m, clickAt(0, y), clickAt(0, y))
	if m.mode != modePriority || m.target.task.ID != "t-flak" {
		t.Fatalf("double-click on priority: mode %d target %q, want modePriority on t-flak", m.mode, m.target.task.ID)
	}
	m, _ = press(t, m, keyOf(tea.KeyEsc))
	// …and on the labels meta line.
	y = screenLineOf(t, m, "labels")
	m, _ = mouseTo(t, m, clickAt(0, y), clickAt(0, y))
	if m.mode != modeEditLabels || m.target.task.ID != "t-flak" {
		t.Fatalf("double-click on labels: mode %d target %q, want modeEditLabels on t-flak", m.mode, m.target.task.ID)
	}
	m, _ = press(t, m, keyOf(tea.KeyEsc))
	y = screenLineOf(t, m, "investigate the flake")
	m, _ = mouseTo(t, m, clickAt(0, y), clickAt(0, y))
	if m.mode != modeEditTitle || m.input.String() != "investigate the flake" {
		t.Fatalf("double-click on the title: mode %d input %q, want the prefilled title editor", m.mode, m.input)
	}
	// A read-only meta line — the id — is never a stop: clicks hit nothing.
	m, _ = press(t, m, keyOf(tea.KeyEsc))
	focus := m.detailFocus
	m, _ = mouseTo(t, m, clickAt(0, screenLineOf(t, m, "id          ")))
	if m.mode != modeDetail || m.detailFocus != focus {
		t.Errorf("click on the id line: mode %d focus %d, want unchanged", m.mode, m.detailFocus)
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
	if strings.Contains(v, "▌") {
		t.Errorf("watch detail renders a selected row; view:\n%s", v)
	}
	// The escalations section still shows the question — visible, never
	// selectable, and its bar carries no steering hint.
	if !strings.Contains(v, "NEEDS INPUT (1)") || !strings.Contains(v, "Which license?") {
		t.Errorf("open escalation missing from watch detail; view:\n%s", v)
	}
	// The labels line renders here too — one uniform rule (labels
	// editable, 2026-08-05) — read-only like everything else.
	if !strings.Contains(v, "labels      none") {
		t.Errorf("watch detail missing the labels placeholder line; view:\n%s", v)
	}
	if !strings.Contains(v, "↑/↓ (j/k) scroll · esc back · q quit") {
		t.Errorf("watch detail footer not read-only; view:\n%s", v)
	}
	for _, reject := range []string{"enter answer", "p priority", "c cancel", "e/E edit"} {
		if strings.Contains(v, reject) {
			t.Errorf("watch detail advertises %q; view:\n%s", reject, v)
		}
	}
	for _, k := range []tea.KeyMsg{
		keyOf(tea.KeyEnter),
		{Type: tea.KeyRunes, Runes: []rune{'p'}},
		{Type: tea.KeyRunes, Runes: []rune{'a'}},
		{Type: tea.KeyRunes, Runes: []rune{'c'}}, // the freed key stays free here too
		// e is the context toggle — a read affordance watch keeps
		// (TestWatchModeContextToggle); with no context on this
		// escalation it changes nothing, and it never opens input.
		{Type: tea.KeyRunes, Runes: []rune{'e'}},
		{Type: tea.KeyRunes, Runes: []rune{'E'}},
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
	if !strings.Contains(v, "▌") {
		t.Errorf("cursor row not visible at top; view:\n%s", v)
	}
	// Walk to the last row: the window must follow the cursor there.
	for i := 0; i < 10; i++ {
		m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	}
	v = m.View()
	if !strings.Contains(v, "▌") || !strings.Contains(v, "idea: dark mode") {
		t.Errorf("cursor row (inbox t-idea) not visible after scrolling; view:\n%s", v)
	}
	if strings.Contains(v, "question:") {
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
// the variable-height layout: one-line ready rows, the three-line
// escalation row (any line hits), and chrome (bars, blanks, the
// header, the footer) hitting nothing.
func TestTopClickSelectsAcrossVariableHeights(t *testing.T) {
	m := newTopModel(newFakeSteering())
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	m = mm.(topModel)
	tests := []struct {
		name, aim string
		want      int // cursor after the click; -1 = unchanged
	}{
		{"one-line ready row", "sweep the floor", 1}, // t-flor leads ready (P0-highest)
		{"in-progress row", "investigate the flake", 3},
		{"escalation title line", "choose a license", 0},
		{"escalation question line", "question: Which license?", 0},
		{"escalation meta line", "brandon/a2", 0},
		{"held row", "polish the docs", 4},
		{"section bar", "READY (2)", -1},
		{"header bar", "tuhdoo · local-only", -1},
		{"footer bar", "h history", -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := m
			m.cursor = 2 // t-pars: nothing under test starts selected
			m, cmd := mouseTo(t, m, clickAt(0, screenLineOf(t, m, tt.aim)))
			want := tt.want
			if want == -1 {
				want = 2
			}
			if m.cursor != want || m.mode != modeNav || cmd != nil {
				t.Errorf("click on %s: cursor %d mode %d cmd %v, want cursor %d modeNav nil",
					tt.name, m.cursor, m.mode, cmd, want)
			}
		})
	}
	// A click past the rendered frame hits nothing.
	m.cursor = 2
	if m, _ := mouseTo(t, m, clickAt(0, 39)); m.cursor != 2 || m.mode != modeNav {
		t.Errorf("click below the list: cursor %d mode %d, want 2 modeNav", m.cursor, m.mode)
	}
}

// Click on the already-selected row acts as enter — and a double-click
// is exactly that: press one selects, press two finds it selected. On
// a task row that opens detail; on an escalation row it opens the task
// view with the question preselected (task-view rework, 2026-08-01).
func TestTopClickOnSelectedActsAsEnter(t *testing.T) {
	m := newTopModel(newFakeSteering())
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	m = mm.(topModel)
	// Double-click a task row: first press selects, second opens detail.
	y := screenLineOf(t, m, "write the parser")
	m, _ = mouseTo(t, m, clickAt(0, y))
	if m.cursor != 2 || m.mode != modeNav {
		t.Fatalf("first click: cursor %d mode %d, want 2 modeNav", m.cursor, m.mode)
	}
	m, _ = mouseTo(t, m, clickAt(0, y))
	if m.mode != modeDetail || m.detailID != "t-pars" {
		t.Fatalf("second click: mode %d detail %q, want modeDetail t-pars", m.mode, m.detailID)
	}
	m, _ = press(t, m, keyOf(tea.KeyEsc))
	// Click the escalation row (esc restores nav on row 2 under
	// P0-highest — walk back up first).
	m, _ = press(t, m, keyOf(tea.KeyUp), keyOf(tea.KeyUp))
	m, _ = mouseTo(t, m, clickAt(0, screenLineOf(t, m, "Which license?")))
	if m.mode != modeDetail || m.detailID != "t-lic" || m.detailFocus != 3 {
		t.Fatalf("click on selected escalation row: mode %d detail %q focus %d, want the task view of t-lic with its escalation stop focused",
			m.mode, m.detailID, m.detailFocus)
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
	for i := 0; i < 5; i++ {
		m, _ = press(t, m, keyOf(tea.KeyDown)) // to t-idea; top scrolls off
	}
	if v := m.View(); strings.Contains(v, "question:") {
		t.Fatalf("test needs the top scrolled off; view:\n%s", v)
	}
	// The inbox row, at its scrolled screen position, still resolves to
	// the selected row — so the click acts as enter.
	m2, _ := mouseTo(t, m, clickAt(0, screenLineOf(t, m, "idea: dark mode")))
	if m2.mode != modeDetail || m2.detailID != "t-idea" {
		t.Errorf("click on scrolled selected row: mode %d detail %q, want modeDetail t-idea",
			m2.mode, m2.detailID)
	}
	// The INBOX bar above it stays chrome.
	m3, _ := mouseTo(t, m, clickAt(0, screenLineOf(t, m, "INBOX (1)")))
	if m3.mode != modeNav || m3.cursor != 5 {
		t.Errorf("click on scrolled bar: mode %d cursor %d, want modeNav 5", m3.mode, m3.cursor)
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
		if m.mode == modeAnswer || m.mode == modePriority || m.mode == modeConfirmCancel {
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
	// enter routes to the task view; enter there opens answer entry.
	m, _ = press(t, m, keyOf(tea.KeyEnter), keyOf(tea.KeyEnter))
	m, _ = press(t, m, runes("Use MIT.")...)
	m, cmd := mouseTo(t, m,
		clickAt(0, 10), clickAt(0, 10), wheelMsg(tea.MouseButtonWheelDown))
	if m.mode != modeAnswer || m.input.String() != "Use MIT." || m.cursor != 0 || cmd != nil {
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
	m = openDetail(t, m, "t-flak")
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
// — no y/n anywhere (capture is reversible via cancel).
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
	// No "captured to inbox" confirmation (feedback only, 2026-08-21):
	// the INBOX section growing a row is the confirmation — accepted
	// that it may land below the fold.
	mm, _ := m.Update(am)
	if m = mm.(topModel); m.status != "" {
		t.Errorf("status = %q, want empty after success", m.status)
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

// Shelf rows are ordinary rows: enter opens the biography, c cancels —
// on both held and inbox.
func TestTopShelfRowsOpenDetailAndCancel(t *testing.T) {
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
			t.Fatalf("cancel on %s produced no command", id)
		}
		if am := cmd().(actionMsg); am.err != nil {
			t.Fatalf("cancel %s: %v", id, am.err)
		}
		if len(fake.cancelled) != 1 || fake.cancelled[0] != id {
			t.Fatalf("cancelled %v, want [%s]", fake.cancelled, id)
		}
	}
}

// A task blocked on an inbox or held dependency names that status in
// its waiting: reason, through the existing taskRef annotation.
func TestTopBlockedOnShelvedDependency(t *testing.T) {
	s := topSnapshot()
	s.state.Tasks = append(s.state.Tasks,
		stateTask{ID: "t-wait", Title: "build on the idea", Status: "open",
			Situation: "blocked", UnmetDeps: []string{"t-idea", "t-park"}})
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

// ---- Needs Input as the single home (task tuh-…VK83BN, 2026-07-31) ----

// A task blocked by both unmet deps and an open blocking escalation
// keeps rows in both sections; its waiting: line names only the deps —
// "needs input (above)" is dead — and the BLOCKED bar count equals its
// rendered rows (the escalation-only t-lic renders none).
func TestTopBlockedRowNamesOnlyUnmetDeps(t *testing.T) {
	s := topSnapshot()
	esc := escalationJSON{
		ID: "01E9", Task: "t-dual", Actor: "brandon/a3",
		Question: "Proceed with v2?", Blocking: true,
		RaisedAt: time.Date(2026, 7, 30, 10, 0, 0, 0, time.UTC),
	}
	s.state.Tasks = append(s.state.Tasks, stateTask{ID: "t-dual", Title: "port the daemon", Status: "open",
		Situation: "blocked", UnmetDeps: []string{"t-flor"}, BlockingEscalations: []string{"01E9"}})
	s.state.OpenEscalations = append(s.state.OpenEscalations, esc)
	s.tasks["t-dual"] = hydratedTask{
		Task:        taskJSON{ID: "t-dual", Title: "port the daemon", DependsOn: []string{"t-flor"}},
		Escalations: []escalationJSON{esc},
	}
	m := topModel{armed: true, actor: "brandon", snap: s, rows: buildRows(s), width: 120, height: 60}
	v := m.View()
	for _, want := range []string{
		"NEEDS INPUT (2)",
		"question: Proceed with v2?",
		"BLOCKED (1)",
		"waiting: depends on t-flor (open — sweep the floor)",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("view missing %q; view:\n%s", want, v)
		}
	}
	if strings.Contains(v, "needs input") {
		t.Errorf("waiting: line still points at the Needs Input section; view:\n%s", v)
	}
	// The bar count is the rendered row count: exactly one waiting: line.
	if n := strings.Count(v, "waiting:"); n != 1 {
		t.Errorf("%d waiting: lines, want 1; view:\n%s", n, v)
	}
	// Both of t-dual's rows carry the task title — they state different
	// truths (a question waits; a dep is unmet) and that is accepted.
	if n := strings.Count(v, "port the daemon"); n != 2 {
		t.Errorf("dual-blocked title renders %d times, want 2; view:\n%s", n, v)
	}
}

// A non-blocking escalation renders the same task-shaped three-liner
// with an empty badge cell — no ! — and its task keeps its ordinary
// status row: claimability is untouched, and the title appearing in
// both places is accepted (the rows state different truths).
func TestTopNonBlockingEscalationRow(t *testing.T) {
	s := topSnapshot()
	esc := escalationJSON{
		ID: "01E8", Task: "t-flor", Actor: "brandon/a4",
		Question: "Mop or broom?",
		RaisedAt: time.Date(2026, 7, 30, 11, 0, 0, 0, time.UTC),
	}
	s.state.OpenEscalations = append(s.state.OpenEscalations, esc)
	h := s.tasks["t-flor"]
	h.Escalations = append(h.Escalations, esc)
	s.tasks["t-flor"] = h
	m := topModel{armed: true, actor: "brandon", snap: s, rows: buildRows(s), width: 80, height: 60}
	v := m.View()
	for _, want := range []string{
		"NEEDS INPUT (2)",
		"  t-flor      sweep the floor", // empty badge cell on the grid
		"              question: Mop or broom?",
		"              brandon/a4 · 2026-07-30 11:00 UTC",
		"READY (2)",
		"  t-flor  p1  sweep the floor", // still rowed under its status section
	} {
		if !strings.Contains(v, want) {
			t.Errorf("view missing %q; view:\n%s", want, v)
		}
	}
	// Exactly one ! badge on screen: the blocking t-lic escalation's.
	if n := strings.Count(v, "!"); n != 1 {
		t.Errorf("%d ! badges, want 1 (non-blocking rows carry none); view:\n%s", n, v)
	}
}

// wrapForSearch flattens a rendered view so assertions survive line
// wrapping: newlines plus their indentation collapse to nothing.
func wrapForSearch(v string) string {
	out := strings.ReplaceAll(v, "\n", "")
	return strings.Join(strings.Fields(out), " ")
}

// ---- history mode (history view, 2026-08-02): the done/cancelled shelf ----

// historySnapshot extends topSnapshot with close metadata and more
// terminal tasks: three done and two cancelled, created in one order
// and closed in another, so reverse-chron-by-close is distinguishable
// from creation order.
func historySnapshot() *snapshot {
	s := topSnapshot()
	day := func(d int) *time.Time {
		t := time.Date(2026, 7, d, 9, 0, 0, 0, time.UTC)
		return &t
	}
	// The seeded done task gets its close stamp, in both payloads.
	for i := range s.state.Tasks {
		if s.state.Tasks[i].ID == "t-chor" {
			s.state.Tasks[i].ClosedAt, s.state.Tasks[i].ClosedBy = day(28), "brandon/impl-1"
		}
	}
	ch := s.tasks["t-chor"]
	ch.Task.Status, ch.Task.ClosedAt, ch.Task.ClosedBy = "done", day(28), "brandon/impl-1"
	s.tasks["t-chor"] = ch
	add := func(st stateTask, hy hydratedTask) {
		s.state.Tasks = append(s.state.Tasks, st)
		s.tasks[st.ID] = hy
	}
	add(stateTask{ID: "t-ship", Title: "ship the tui", Status: "done", Situation: "done",
		Labels: []string{"tui"}, ClosedAt: day(30), ClosedBy: "brandon/claude-code-1"},
		hydratedTask{Task: taskJSON{ID: "t-ship", Title: "ship the tui", Status: "done",
			Labels: []string{"tui"}, ClosedAt: day(30), ClosedBy: "brandon/claude-code-1"}})
	add(stateTask{ID: "t-mgr8", Title: "migrate the backlog", Status: "done", Situation: "done",
		ClosedAt: day(29), ClosedBy: "brandon"},
		hydratedTask{Task: taskJSON{ID: "t-mgr8", Title: "migrate the backlog", Status: "done",
			DependsOn: []string{"t-chor"}, ClosedAt: day(29), ClosedBy: "brandon"}})
	add(stateTask{ID: "t-zzzz", Title: "zombie idea", Status: "cancelled", Situation: "cancelled",
		ClosedAt: day(31), ClosedBy: "brandon/a2"},
		hydratedTask{Task: taskJSON{ID: "t-zzzz", Title: "zombie idea", Status: "cancelled",
			ClosedAt: day(31), ClosedBy: "brandon/a2"}})
	// A cancelled task with an unanswered escalation: on a terminal
	// task it is record, not work — it renders in the detail's History.
	add(stateTask{ID: "t-drop", Title: "drop the wiki", Status: "cancelled", Situation: "cancelled",
		ClosedAt: day(27), ClosedBy: "brandon"},
		hydratedTask{
			Task: taskJSON{ID: "t-drop", Title: "drop the wiki", Status: "cancelled",
				CreatedAt: time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC), CreatedBy: "brandon",
				ClosedAt: day(27), ClosedBy: "brandon"},
			Escalations: []escalationJSON{{
				ID: "01E7", Task: "t-drop", Actor: "brandon/a7",
				Question: "Keep the wiki export?",
				RaisedAt: time.Date(2026, 7, 25, 9, 0, 0, 0, time.UTC),
			}},
		})
	return s
}

func newHistoryModel(api steeringAPI) topModel {
	s := historySnapshot()
	return topModel{api: api, actor: "brandon", armed: true, repoName: "tuhdoo", snap: s, rows: buildRows(s)}
}

// h opens history from the top list of both panes — browsing is
// reading — with DONE then CANCELLED, each newest close first; esc
// returns to the dashboard.
func TestTopHistoryOpensFromBothPanes(t *testing.T) {
	s := historySnapshot()
	for name, m := range map[string]topModel{
		"armed": newHistoryModel(newFakeSteering()),
		"watch": {snap: s, rows: buildRows(s)},
	} {
		m, cmd := press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
		if !m.history || m.mode != modeNav || m.cursor != 0 || cmd != nil {
			t.Fatalf("%s: h left history %v mode %d cursor %d cmd %v", name, m.history, m.mode, m.cursor, cmd)
		}
		// Reverse-chron within each bar, DONE before CANCELLED — not
		// creation order (t-chor was created first, closed 07-28).
		want := []struct{ section, id string }{
			{"done", "t-ship"}, {"done", "t-mgr8"}, {"done", "t-chor"},
			{"cancelled", "t-zzzz"}, {"cancelled", "t-drop"},
		}
		if len(m.rows) != len(want) {
			t.Fatalf("%s: %d history rows, want %d: %+v", name, len(m.rows), len(want), m.rows)
		}
		for i, w := range want {
			if m.rows[i].section != w.section || m.rows[i].id() != w.id {
				t.Errorf("%s: row %d = %s %s, want %+v", name, i, m.rows[i].section, m.rows[i].id(), w)
			}
		}
		v := m.View()
		for _, wantS := range []string{
			"DONE (3)", "CANCELLED (2)",
			// Two-line anatomy (grill 2026-08-05): title line, then the
			// dim meta line with the close stamp · closing actor tail.
			"▌ t-ship      ship the tui",
			"▌             [tui] · 2026-07-30 · brandon/claude-code-1",
			"  t-mgr8      migrate the backlog",
			"              1 dep · 2026-07-29 · brandon",
			"  t-chor      old chore",
			"              2026-07-28 · brandon/impl-1",
			"  t-zzzz      zombie idea",
			"              2026-07-31 · brandon/a2",
			"  t-drop      drop the wiki",
			"              2026-07-27 · brandon",
			"↑/↓ (j/k) move · enter open · esc back · q quit",
		} {
			if !strings.Contains(v, wantS) {
				t.Errorf("%s: history view missing %q; view:\n%s", name, wantS, v)
			}
		}
		// The open queue does not render here, and no steering or
		// history key is advertised.
		for _, reject := range []string{"READY", "NEEDS INPUT", "INBOX", "write the parser",
			"h history", "p priority", "c cancel", "i capture", "done "} {
			if strings.Contains(v, reject) {
				t.Errorf("%s: history view still contains %q; view:\n%s", name, reject, v)
			}
		}
		// q quits from history.
		if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}); cmd == nil {
			t.Errorf("%s: q in history should quit", name)
		}
		// esc returns to the dashboard.
		m, cmd = press(t, m, keyOf(tea.KeyEsc))
		if m.history || m.mode != modeNav || cmd != nil {
			t.Fatalf("%s: esc left history %v mode %d", name, m.history, m.mode)
		}
		if v := m.View(); !strings.Contains(v, "READY (2)") || strings.Contains(v, "CANCELLED") {
			t.Errorf("%s: esc did not restore the dashboard; view:\n%s", name, v)
		}
	}
}

// The esc stack: enter on a history row opens the ordinary task view,
// esc from there returns to the history list — not the dashboard — and
// a second esc reaches the dashboard. h inside the task view is dead.
func TestTopHistoryEscStack(t *testing.T) {
	m := newHistoryModel(newFakeSteering())
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	m = moveTo(t, m, "t-chor")
	m, _ = press(t, m, keyOf(tea.KeyEnter))
	if m.mode != modeDetail || m.detailID != "t-chor" {
		t.Fatalf("enter on a history row: mode %d detail %q, want the task view of t-chor", m.mode, m.detailID)
	}
	// h in the task view changes nothing.
	m, cmd := press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if m.mode != modeDetail || cmd != nil {
		t.Errorf("h in detail: mode %d cmd %v, want modeDetail nil", m.mode, cmd)
	}
	m, _ = press(t, m, keyOf(tea.KeyEsc))
	if m.mode != modeNav || !m.history {
		t.Fatalf("esc from a history-opened detail: mode %d history %v, want the history list", m.mode, m.history)
	}
	if m.cursor >= len(m.rows) || m.rows[m.cursor].id() != "t-chor" {
		t.Errorf("history cursor lost across the detail round trip: %d", m.cursor)
	}
	if v := m.View(); !strings.Contains(v, "DONE (3)") {
		t.Errorf("history list not restored; view:\n%s", v)
	}
	m, _ = press(t, m, keyOf(tea.KeyEsc))
	if m.history || m.mode != modeNav {
		t.Fatalf("second esc: history %v mode %d, want the dashboard", m.history, m.mode)
	}
}

// Enter and click open detail from history exactly like the dashboard:
// click selects the row under the pointer, click on the selected row
// opens its task view.
func TestTopHistoryEnterAndClickOpenDetail(t *testing.T) {
	m := newHistoryModel(newFakeSteering())
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	m = mm.(topModel)
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	// enter on the selected top row.
	m, _ = press(t, m, keyOf(tea.KeyEnter))
	if m.mode != modeDetail || m.detailID != "t-ship" {
		t.Fatalf("enter: mode %d detail %q, want modeDetail t-ship", m.mode, m.detailID)
	}
	m, _ = press(t, m, keyOf(tea.KeyEsc))
	// Click an unselected row: selection moves, nothing opens.
	y := screenLineOf(t, m, "old chore")
	m, cmd := mouseTo(t, m, clickAt(0, y))
	if m.mode != modeNav || m.cursor != 2 || cmd != nil {
		t.Fatalf("click on unselected history row: mode %d cursor %d, want selection moved", m.mode, m.cursor)
	}
	// A section bar stays chrome.
	m2, _ := mouseTo(t, m, clickAt(0, screenLineOf(t, m, "CANCELLED (2)")))
	if m2.mode != modeNav || m2.cursor != 2 {
		t.Errorf("click on the CANCELLED bar: mode %d cursor %d, want unchanged", m2.mode, m2.cursor)
	}
	// Click the selected row: acts as enter.
	m, _ = mouseTo(t, m, clickAt(0, y))
	if m.mode != modeDetail || m.detailID != "t-chor" {
		t.Fatalf("click on selected history row: mode %d detail %q, want modeDetail t-chor", m.mode, m.detailID)
	}
}

// History scrolls exactly like the dashboard: the cursor-following
// window keeps the selected row visible, j/k and the wheel clamp.
func TestTopHistoryScrollMatchesDashboard(t *testing.T) {
	m := newHistoryModel(newFakeSteering())
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 8})
	m = mm.(topModel)
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	for i := 0; i < 10; i++ {
		m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	}
	if want := len(m.rows) - 1; m.cursor != want {
		t.Fatalf("j past bottom left cursor at %d, want clamp at %d", m.cursor, want)
	}
	v := m.View()
	if !strings.Contains(v, "▌ t-drop") {
		t.Errorf("cursor row not visible after scrolling; view:\n%s", v)
	}
	if strings.Contains(v, "ship the tui") {
		t.Errorf("top of history should have scrolled off; view:\n%s", v)
	}
	if n := strings.Count(strings.TrimRight(v, "\n"), "\n") + 1; n > 8 {
		t.Errorf("frame taller than terminal: %d > 8 lines; view:\n%s", n, v)
	}
	// The wheel mirrors j/k, clamping at both ends.
	for i := 0; i < 10; i++ {
		m, _ = mouseTo(t, m, wheelMsg(tea.MouseButtonWheelUp))
	}
	if m.cursor != 0 {
		t.Errorf("wheel past the top left cursor at %d, want 0", m.cursor)
	}
}

// The history list is read-only in the armed pane too: p, c, and i are
// dead on its rows.
func TestTopHistorySteeringKeysDead(t *testing.T) {
	fake := newFakeSteering()
	m := newHistoryModel(fake)
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	for _, r := range []rune{'p', 'c', 'i'} {
		mm, cmd := press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		if mm.mode != modeNav || cmd != nil {
			t.Errorf("%q on a history row: mode %d cmd %v, want dead", r, mm.mode, cmd)
		}
	}
	if len(fake.cancelled) != 0 || len(fake.priorities) != 0 || len(fake.captured) != 0 {
		t.Errorf("history keys still wrote: %+v", fake)
	}
}

// A refresh while history is open rebuilds history rows — not the
// dashboard's — and keeps the selection on its row.
func TestTopHistoryRefreshKeepsRowsAndSelection(t *testing.T) {
	m := newHistoryModel(newFakeSteering())
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	m = moveTo(t, m, "t-chor")
	mm, _ := m.Update(snapMsg{snap: historySnapshot()})
	m = mm.(topModel)
	if !m.history {
		t.Fatal("refresh knocked the model out of history")
	}
	if r, ok := m.selected(); !ok || r.id() != "t-chor" || r.section != "done" {
		t.Errorf("selection lost across refresh: %+v", r)
	}
	if v := m.View(); !strings.Contains(v, "DONE (3)") || strings.Contains(v, "READY") {
		t.Errorf("refresh rebuilt the wrong rows; view:\n%s", v)
	}
}

// The task view of a terminal task refuses steering: p and c are dead,
// the armed footer stops advertising them, and priority is not a ring
// stop — one j from the title lands on the description.
func TestTopDetailTerminalTaskSteeringDead(t *testing.T) {
	fake := newFakeSteering()
	m := newHistoryModel(fake)
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	m, _ = press(t, m, keyOf(tea.KeyEnter)) // t-ship, done
	if m.mode != modeDetail || m.detailID != "t-ship" {
		t.Fatalf("mode %d detail %q, want modeDetail t-ship", m.mode, m.detailID)
	}
	for _, r := range []rune{'p', 'c'} {
		mm, cmd := press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		if mm.mode != modeDetail || cmd != nil {
			t.Errorf("%q on a terminal task's detail: mode %d cmd %v, want dead", r, mm.mode, cmd)
		}
	}
	if len(fake.cancelled) != 0 || len(fake.priorities) != 0 {
		t.Errorf("terminal detail still wrote: %+v", fake)
	}
	v := m.View()
	if !strings.Contains(v, " ↑/↓ (j/k) move · enter edit · esc back · q quit") {
		t.Errorf("terminal detail footer wrong; view:\n%s", v)
	}
	for _, reject := range []string{"p priority", "c cancel"} {
		if strings.Contains(v, reject) {
			t.Errorf("terminal detail advertises %q; view:\n%s", reject, v)
		}
	}
	// The ring skips priority and labels — a closed record is browsed,
	// not steered: title, then description.
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m, _ = press(t, m, keyOf(tea.KeyEnter))
	if m.mode != modeEditDesc {
		t.Errorf("enter one stop below the title: mode %d, want modeEditDesc (no priority or labels stop)", m.mode)
	}
}

// A terminal task's status line carries its close metadata, and its
// unanswered escalation renders in History — the NEEDS INPUT section
// only serves open work. Both panes render the same lines.
func TestTopDetailTerminalStatusAndEscalationRecord(t *testing.T) {
	s := historySnapshot()
	for name, m := range map[string]topModel{
		"armed": newHistoryModel(newFakeSteering()),
		"watch": {snap: s, rows: buildRows(s)},
	} {
		m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
		m = moveTo(t, m, "t-drop")
		m, _ = press(t, m, keyOf(tea.KeyEnter))
		v := m.View()
		for _, want := range []string{
			"t-drop — drop the wiki",
			"status      cancelled — 2026-07-27 by brandon",
			"HISTORY",
			"escalation from brandon/a7",
			"Q: Keep the wiki export?",
			"unanswered",
		} {
			if !strings.Contains(v, want) {
				t.Errorf("%s: terminal detail missing %q; view:\n%s", name, want, v)
			}
		}
		if strings.Contains(v, "NEEDS INPUT") {
			t.Errorf("%s: a terminal task's unanswered escalation still solicits answers; view:\n%s", name, v)
		}
		// The done wording: "finished <day> by <actor>".
		m, _ = press(t, m, keyOf(tea.KeyEsc))
		m = moveTo(t, m, "t-ship")
		m, _ = press(t, m, keyOf(tea.KeyEnter))
		if v := m.View(); !strings.Contains(v, "status      done — finished 2026-07-30 by brandon/claude-code-1") {
			t.Errorf("%s: done status line missing close metadata; view:\n%s", name, v)
		}
	}
}
