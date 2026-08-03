package main

// The interactive TUI (002 T7, revised by Cycle 4): the single live
// human surface. Reads poll the daemon on a tick; the steering writes
// (answer an escalation, reprioritize, cancel, capture, edit
// title/description) go through the daemon HTTP API only, stamped with
// the acting human principal.
// Watch mode is the same screen disarmed: steering keys dead, fixed at
// launch — no keypress can re-arm a disarmed pane.

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/brandonbews/tuhdoo/internal/daemon"
	"github.com/brandonbews/tuhdoo/internal/views"
)

// wrapTo wraps rendered output to the terminal width, ANSI-aware:
// wordwrap for readability, then a hard wrap so nothing (ULIDs, long
// unbroken tokens) ever exceeds the width. Zero width — tests, or no
// WindowSizeMsg yet — wraps nothing.
func wrapTo(s string, width int) string {
	if width <= 0 {
		return s
	}
	return ansi.Hardwrap(ansi.Wordwrap(s, width, ""), width, true)
}

// ---- polling: state arrives by tick, never by keypress ----

const tuiRefresh = 2 * time.Second

type tickMsg time.Time

// snapMsg carries one poll result; err is shown and retried, never fatal.
type snapMsg struct {
	snap *snapshot
	err  error
}

func tickCmd() tea.Cmd {
	return tea.Tick(tuiRefresh, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func fetchCmd(c *client) tea.Cmd {
	return func() tea.Msg {
		s, err := fetchSnapshot(c)
		return snapMsg{snap: s, err: err}
	}
}

// steeringAPI is the full set of writes top can perform. An interface
// so interaction tests run against a fake; the real one speaks the
// daemon HTTP API — never git, never the ops layer (T7).
type steeringAPI interface {
	answerEscalation(escalation, answer string) error
	setPriority(task string, priority int) error
	cancelTask(task string) error
	captureTask(title string) error
	setTitle(task, title string) error
	setDescription(task, description string) error
}

// httpSteering implements steeringAPI over the daemon's JSON HTTP API,
// reusing the admin verbs that already exist there: no new write paths.
type httpSteering struct {
	c     *client
	actor string
}

func (s httpSteering) answerEscalation(escalation, answer string) error {
	return s.c.write("POST", "/v0/escalations/answer", s.actor, map[string]any{
		"escalation": escalation, "answer": answer,
	})
}

func (s httpSteering) setPriority(task string, priority int) error {
	return s.c.write("PATCH", "/v0/tasks/"+task, s.actor, map[string]any{"priority": priority})
}

// cancelTask writes the terminal curation status. The task model has
// no delete (D5) — "cancelled" is terminal but never removed, and
// since the status-vocabulary revision (2026-08-01) cancel is the word
// everywhere: the one-day "archive" porcelain verb is retired.
func (s httpSteering) cancelTask(task string) error {
	return s.c.write("PATCH", "/v0/tasks/"+task, s.actor, map[string]any{"status": "cancelled"})
}

// setTitle and setDescription are the task view's edit writes
// (dogfood capture, 2026-08-01): the same PATCH — and so the same
// task.updated ledger event — as `tuhdoo update --title/--desc`,
// stamped with the steering human.
func (s httpSteering) setTitle(task, title string) error {
	return s.c.write("PATCH", "/v0/tasks/"+task, s.actor, map[string]any{"title": title})
}

func (s httpSteering) setDescription(task, description string) error {
	return s.c.write("PATCH", "/v0/tasks/"+task, s.actor, map[string]any{"description": description})
}

// captureTask is TUI quick-capture (2026-07-31): a title-only inbox
// item, created as the steering human. Capture is deliberately cheap —
// no priority, no description, no confirm (cancel reverses it); the
// scoping work happens at promotion, which is an agent/CLI
// conversation, never a TUI key.
func (s httpSteering) captureTask(title string) error {
	return s.c.write("POST", "/v0/tasks", s.actor,
		[]map[string]any{{"title": title, "status": "inbox"}})
}

// ---- rows: what the cursor moves over ----

const (
	rowEscalation = "escalation"
	rowTask       = "task"
)

// topRow is one selectable line: an open escalation or an open task.
type topRow struct {
	kind    string
	section string         // section heading it renders under
	esc     escalationJSON // set when kind == rowEscalation
	task    stateTask      // set when kind == rowTask
}

// id returns the row's stable identity across refreshes (event ULIDs
// are unique across kinds).
func (r topRow) id() string {
	if r.kind == rowEscalation {
		return r.esc.ID
	}
	return r.task.ID
}

// buildRows flattens a snapshot into the selectable rows in render
// order: open escalations, then ready, in-progress, and blocked tasks,
// then the dim shelves — held above inbox (2026-07-31; held passed
// triage, so it sits closer to workable than raw captures). Done and
// cancelled tasks are not steerable and get no rows; held and inbox
// rows are ordinary rows — enter opens detail, c cancels.
func buildRows(s *snapshot) []topRow {
	var rows []topRow
	for _, e := range s.state.OpenEscalations {
		rows = append(rows, topRow{kind: rowEscalation, section: "escalations", esc: e})
	}
	b := s.classify()
	for _, t := range b.ready {
		rows = append(rows, topRow{kind: rowTask, section: "ready", task: t})
	}
	for _, t := range b.inProgress {
		rows = append(rows, topRow{kind: rowTask, section: "inprogress", task: t})
	}
	for _, t := range b.blocked {
		// A task whose only blocker is an open blocking escalation is
		// represented by its Needs Input row alone (grill cycle,
		// 2026-07-31); only unmet deps earn a BLOCKED row. The one-shot
		// commands keep the full blocked bucket — their count may differ.
		if !s.hasUnmetDeps(t.ID) {
			continue
		}
		rows = append(rows, topRow{kind: rowTask, section: "blocked", task: t})
	}
	for _, t := range b.held {
		rows = append(rows, topRow{kind: rowTask, section: "held", task: t})
	}
	for _, t := range b.inbox {
		rows = append(rows, topRow{kind: rowTask, section: "inbox", task: t})
	}
	return rows
}

// buildHistoryRows flattens the terminal shelf for history mode
// (history view, 2026-08-02): DONE then CANCELLED, each newest close
// first — recency is the browse axis here, not priority. History is a
// "what did I build" device for the steering human; forensics stay in
// the raw events.
func buildHistoryRows(s *snapshot) []topRow {
	b := s.classify()
	var rows []topRow
	for _, t := range closedNewestFirst(b.done) {
		rows = append(rows, topRow{kind: rowTask, section: "done", task: t})
	}
	for _, t := range closedNewestFirst(b.cancelled) {
		rows = append(rows, topRow{kind: rowTask, section: "cancelled", task: t})
	}
	return rows
}

// closedNewestFirst orders one terminal bucket by close time, newest
// first. Ties — and rows a pre-upgrade daemon sent without a stamp,
// which sink to the bottom — keep creation (ULID) order: the ordering
// is deterministic for any snapshot.
func closedNewestFirst(ts []stateTask) []stateTask {
	out := make([]stateTask, len(ts))
	copy(out, ts)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].ClosedAt, out[j].ClosedAt
		if a == nil || b == nil {
			return b == nil && a != nil
		}
		return a.After(*b)
	})
	return out
}

// activeRows builds the rows of whichever list is on screen.
func (m topModel) activeRows(s *snapshot) []topRow {
	if m.history {
		return buildHistoryRows(s)
	}
	return buildRows(s)
}

// ---- the model ----

// Input modes. Nav is the resting state; the others capture keys until
// enter/esc (or y/n for the cancel confirmation). Detail is the
// in-place task view: armed it steers the viewed task (the focus ring
// selects a field, enter opens its editor, p/c reprioritize/cancel),
// disarmed it is read-only; esc steps back to the list either way.
const (
	modeNav = iota
	modeAnswer
	modePriority
	modeConfirmCancel
	modeCapture // quick-capture: one line of title, straight to inbox
	modeDetail
	modeEditTitle // task-view edits (2026-08-01): the widget prefilled
	modeEditDesc  // with the current value; unchanged submit writes nothing
)

// actionMsg is the result of one steering write.
type actionMsg struct {
	desc string
	err  error
}

type topModel struct {
	c     *client
	api   steeringAPI
	col   colors
	actor string
	armed bool // false in watch mode: steering keys are dead

	snap    *snapshot
	err     error
	rows    []topRow
	cursor  int
	history bool // history mode (2026-08-02): the list shows the done/cancelled shelf

	mode    int
	back    int       // mode an input mode returns to on esc/submit: the list or the detail it was opened from
	input   textInput // the shared text-entry widget (textinput.go); zero value = empty single-line
	target  topRow    // row a pending answer/priority/cancel/edit applies to
	editWas string    // the edited field's value when an edit mode opened: an unchanged submit writes nothing
	status  string    // one-line result of the last action

	detailID     string // task shown by modeDetail
	detailScroll int    // first visible body line in modeDetail
	detailFocus  int    // focused stop in an armed detail (index into detailStops)
	width        int    // terminal columns; 0 (no WindowSizeMsg yet) wraps nothing
	height       int    // terminal rows; 0 renders all
}

func (m topModel) Init() tea.Cmd {
	return tea.Batch(fetchCmd(m.c), tickCmd())
}

func (m topModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.mode {
		case modeNav:
			return m.updateNav(msg)
		case modeDetail:
			return m.updateDetail(msg)
		}
		return m.updateInput(msg)
	case tea.MouseMsg:
		return m.updateMouse(msg)
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tickMsg:
		return m, tea.Batch(fetchCmd(m.c), tickCmd())
	case snapMsg:
		m.err = msg.err
		if msg.snap != nil {
			// Keep the selection on the same row across refreshes; a row
			// that vanished (answered, cancelled, claimed…) drops the
			// cursor to the top.
			var sel string
			if m.cursor < len(m.rows) {
				sel = m.rows[m.cursor].id()
			}
			m.snap = msg.snap
			m.rows = m.activeRows(msg.snap)
			m.cursor = 0
			for i, r := range m.rows {
				if r.id() == sel {
					m.cursor = i
					break
				}
			}
		}
		return m, nil
	case actionMsg:
		if msg.err != nil {
			m.status = "error: " + msg.err.Error()
		} else {
			m.status = msg.desc
		}
		// Refresh immediately so the action's effect is on screen before
		// the next tick.
		return m, fetchCmd(m.c)
	}
	return m, nil
}

func (m topModel) updateNav(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "j", "down":
		if m.cursor < len(m.rows)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "enter":
		if r, ok := m.selected(); ok {
			return m.openRow(r)
		}
	case "h":
		// History (history view, 2026-08-02): the done/cancelled shelf,
		// on the dashboard's own list machinery. Browsing is reading, so
		// watch mode gets it too.
		if !m.history && m.snap != nil {
			m.history = true
			m.rows = buildHistoryRows(m.snap)
			m.cursor, m.status = 0, ""
		}
	case "esc":
		if m.history {
			m.history = false
			m.rows = buildRows(m.snap)
			m.cursor, m.status = 0, ""
		}
	case "p":
		if r, ok := m.selected(); m.armed && ok && r.kind == rowTask && !terminalStatus(r.task.Status) {
			m.mode, m.back, m.target, m.input, m.status = modePriority, modeNav, r, textInput{}, ""
		}
	case "c":
		// Cancel is c (status-vocabulary revision, 2026-08-01): the key
		// spells its verb again. It briefly toured "a archive" while the
		// verb was archive (2026-07-31–2026-08-01).
		if r, ok := m.selected(); m.armed && ok && r.kind == rowTask && !terminalStatus(r.task.Status) {
			m.mode, m.back, m.target, m.input, m.status = modeConfirmCancel, modeNav, r, textInput{}, ""
		}
	case "i":
		// Quick capture (2026-07-31): armed only — a watch pane never
		// writes. No row target: capture is about the idea in your head,
		// not the row under the cursor. A dashboard affordance: history
		// keeps no write keys at all.
		if m.armed && !m.history {
			m.mode, m.back, m.target, m.input, m.status = modeCapture, modeNav, topRow{}, textInput{}, ""
		}
	}
	return m, nil
}

// openRow is what enter — and a click on the already-selected row —
// does: open the task view. A Needs Input row opens its task's view
// with that escalation preselected (task-view rework, 2026-08-01 —
// superseding the 2026-07-30 inline answer prompt: dogfooding showed
// answering needs the task's context on screen, so the task view is
// where answering happens now); a plain task row opens its biography
// with the title stop focused at the top of the view. Watch mode gets
// the same view, read-only.
func (m topModel) openRow(r topRow) (tea.Model, tea.Cmd) {
	id := r.task.ID
	if r.kind == rowEscalation {
		id = r.esc.Task
	}
	m.mode, m.detailID, m.detailScroll, m.detailFocus, m.status = modeDetail, id, 0, 0, ""
	if r.kind == rowEscalation {
		for i, s := range m.detailStops() {
			if s.kind == stopEscalation && s.esc.ID == r.esc.ID {
				m.detailFocus = i
				break
			}
		}
		m.detailScroll = m.detailRevealScroll()
	}
	return m, nil
}

// updateMouse (dogfood steering, 2026-07-31): a single click moves the
// cursor to the row under the pointer; a click on the already-selected
// row acts as enter — which is also exactly what a double-click is: the
// first press selects, the second finds the row selected. The wheel
// falls out for free: the list scrolls by moving the cursor (windowing
// follows it), the detail scrolls its line window. Input modes ignore
// the mouse entirely, so a stray click never disturbs a pending answer
// or confirm. Watch mode normally never sees a MouseMsg — tracking is
// armed-only (see runTUI) — but if one arrives anyway, openRow keeps
// the read-only contract: a disarmed pane opens detail, never input.
func (m topModel) updateMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	wheelUp := msg.Button == tea.MouseButtonWheelUp
	wheelDown := msg.Button == tea.MouseButtonWheelDown
	click := msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress
	switch m.mode {
	case modeNav:
		switch {
		case wheelUp:
			if m.cursor > 0 {
				m.cursor--
			}
		case wheelDown:
			if m.cursor < len(m.rows)-1 {
				m.cursor++
			}
		case click:
			if i := m.rowAt(msg.Y); i >= 0 && i < len(m.rows) {
				if i == m.cursor {
					return m.openRow(m.rows[i])
				}
				m.cursor = i
			}
		}
	case modeDetail:
		switch {
		case wheelUp:
			if m.detailScroll > 0 {
				m.detailScroll--
			}
		case wheelDown:
			if m.detailScroll < m.detailMaxScroll() {
				m.detailScroll++
			}
		case click:
			// Same contract as the list: a click selects the stop under
			// the pointer; a click on the already-selected stop acts as
			// enter (opens its editor). detailStops is empty in watch
			// mode, so a stray click stays read-only.
			stops := m.detailStops()
			i := m.detailStopAt(msg.Y)
			if i < 0 || i >= len(stops) {
				break
			}
			if i == detailFocusIdx(m.detailFocus, len(stops)) {
				m.detailFocus = i
				return m.openStop(stops[i])
			}
			m.detailFocus = i
			m.detailScroll = m.detailRevealScroll()
		}
	}
	return m, nil
}

// detailStopAt maps a terminal row to the stop index rendered there,
// or -1 for everything else. Like rowAt, it replays the exact layout
// detailView draws — same header, same scroll window — off the same
// tagged lines, so the hit map is the layout.
func (m topModel) detailStopAt(y int) int {
	lines := m.detailLines()
	if lines == nil {
		return -1
	}
	width := m.width
	if width <= 0 {
		width = 80
	}
	head := strings.Count(m.listHead(width), "\n")
	scroll := m.detailScroll
	if max := m.detailMaxScroll(); scroll > max {
		scroll = max
	}
	if y < head {
		return -1
	}
	if m.height > 0 && y-head >= m.detailWindow() {
		return -1
	}
	i := y - head + scroll
	if i >= len(lines) {
		return -1
	}
	return lines[i].stop
}

// rowAt maps a terminal row (0-based, from the top of the screen) to
// the m.rows index rendered there, or -1 for chrome (header, bars,
// blanks, footer) and misses. It replays the exact layout View draws —
// the same header, the same chunks, the same cursor-following window —
// so variable-height rows and scroll offsets can never drift from what
// is on screen: the hit map IS the layout, not a re-derivation of it.
func (m topModel) rowAt(y int) int {
	if m.snap == nil || m.err != nil {
		return -1
	}
	width := m.width
	if width <= 0 {
		width = 80
	}
	line := strings.Count(m.listHead(width), "\n")
	footLines := strings.Count("\n"+m.footerView(width), "\n")
	for _, c := range visibleChunks(m.listChunks(width), m.height, line, footLines) {
		n := c.lines()
		if y >= line && y < line+n {
			return c.row
		}
		line += n
	}
	return -1
}

// updateInput drives every text-entry mode. It owns only the mode
// keys — quit, cancel, and submit (enter single-line, ctrl+s
// multi-line, per the widget's hint) — and hands every other key to
// the shared widget: no per-screen editing exists (textinput.go).
func (m topModel) updateInput(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.mode == modeConfirmCancel {
		switch k.String() {
		case "y":
			return m.submit()
		case "n", "esc":
			m.mode, m.input = m.back, textInput{}
		case "ctrl+c":
			return m, tea.Quit
		}
		return m, nil
	}
	switch k.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.mode, m.input = m.back, textInput{}
		return m, nil
	case "enter":
		if !m.input.multiline {
			return m.submit()
		}
	case "ctrl+s":
		if m.input.multiline {
			return m.submit()
		}
	}
	m.input = m.input.handleKey(k, m.width)
	return m, nil
}

// updateDetail: the armed task view steers the viewed task in place
// (dogfood steering, 2026-07-30 — no more esc → navigate → answer round
// trips): the focus ring walks every actionable field (field focus
// ring, 2026-08-02 — retiring the undiscoverable e/E chords), enter
// opens the focused stop's editor, p reprioritizes, c cancels, all
// with the same footers and confirms as the list. Watch mode keeps
// the read-only contract: no focus, no input, ↑/↓ and j/k scroll, esc
// steps back, q and ctrl+c quit.
//
// Focus vs scroll — the deliberate rule, generalized from the old
// escalation-only focus: j/k move focus when a further stop exists in
// that direction (the window scrolls just enough to reveal it) and
// scroll one line otherwise. The description is the last stop, so j
// below it line-scrolls the history into view; k above the title
// line-scrolls back up.
func (m topModel) updateDetail(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	stops := m.detailStops()
	focus := detailFocusIdx(m.detailFocus, len(stops))
	switch k.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.mode, m.detailID, m.detailScroll, m.detailFocus = modeNav, "", 0, 0
	case "j", "down":
		if focus >= 0 && focus < len(stops)-1 {
			m.detailFocus = focus + 1
			m.detailScroll = m.detailRevealScroll()
		} else if m.detailScroll < m.detailMaxScroll() {
			m.detailScroll++
		}
	case "k", "up":
		if focus > 0 {
			m.detailFocus = focus - 1
			m.detailScroll = m.detailRevealScroll()
		} else if m.detailScroll > 0 {
			m.detailScroll--
		}
	case "enter":
		if focus >= 0 { // armed; watch mode has no stops by construction
			m.detailFocus = focus
			return m.openStop(stops[focus])
		}
	case "p":
		// Dead on terminal tasks (history view, 2026-08-02): a closed
		// record is browsed, not steered — same for c below.
		if t, ok := m.viewedTask(); m.armed && ok && !terminalStatus(t.Status) {
			m.mode, m.back = modePriority, modeDetail
			m.target, m.input, m.status = topRow{kind: rowTask, task: t}, textInput{}, ""
		}
	case "c":
		if t, ok := m.viewedTask(); m.armed && ok && !terminalStatus(t.Status) {
			m.mode, m.back = modeConfirmCancel, modeDetail
			m.target, m.input, m.status = topRow{kind: rowTask, task: t}, textInput{}, ""
		}
	}
	return m, nil
}

// The task view's focusable stops, in render order. Bars and read-only
// meta lines are never stops; labels editing is a separate task
// (tuh-01KYXVSRVK2GFW439G1T0GBQKM).
const (
	stopTitle       = "title"
	stopPriority    = "priority"
	stopEscalation  = "escalation"
	stopDescription = "description"
)

// detailStop is one focusable field of the task view.
type detailStop struct {
	kind string
	esc  escalationJSON // set when kind == stopEscalation
}

// detailStops is the focus ring: title, the priority meta line, each
// open escalation, then the description body — the same order they
// render in, which is what lets detailLines tag lines by position.
// Empty in watch mode: the disarmed pane stays fully read-only, so it
// has no focus to act on. A terminal task loses the priority stop
// (history view, 2026-08-02): its editor is the same reprioritize
// write the dead p key refuses.
func (m topModel) detailStops() []detailStop {
	if !m.armed || m.snap == nil {
		return nil
	}
	h, ok := m.snap.tasks[m.detailID]
	if !ok {
		return nil
	}
	stops := []detailStop{{kind: stopTitle}}
	if !terminalStatus(h.Task.Status) {
		stops = append(stops, detailStop{kind: stopPriority})
	}
	for _, e := range m.detailEscalations() {
		stops = append(stops, detailStop{kind: stopEscalation, esc: e})
	}
	return append(stops, detailStop{kind: stopDescription})
}

// openStop opens the focused stop's editor: title and description
// prefilled through the shared widget (unchanged submit writes
// nothing — the editWas rule), priority as the numeric input, an
// escalation as answer entry. One opener for enter and the mouse.
func (m topModel) openStop(s detailStop) (tea.Model, tea.Cmd) {
	if s.kind == stopEscalation {
		m.mode, m.back = modeAnswer, modeDetail
		m.target, m.input, m.status = topRow{kind: rowEscalation, esc: s.esc}, textInput{}, ""
		return m, nil
	}
	t, ok := m.viewedTask()
	if !ok {
		return m, nil
	}
	switch s.kind {
	case stopTitle:
		title := m.snap.tasks[t.ID].Task.Title
		m.mode, m.back = modeEditTitle, modeDetail
		m.target, m.input, m.editWas, m.status = topRow{kind: rowTask, task: t}, editInput(title, false), title, ""
	case stopPriority:
		m.mode, m.back = modePriority, modeDetail
		m.target, m.input, m.status = topRow{kind: rowTask, task: t}, textInput{}, ""
	case stopDescription:
		// Multi-line in the shared widget itself, never a separate
		// editor or $EDITOR (Brandon, 2026-07-31).
		desc := m.snap.tasks[t.ID].Task.Description
		m.mode, m.back = modeEditDesc, modeDetail
		m.target, m.input, m.editWas, m.status = topRow{kind: rowTask, task: t}, editInput(desc, true), desc, ""
	}
	return m, nil
}

// detailEscalations lists the viewed task's unanswered escalations in
// ULID order: the rows of the task view's NEEDS INPUT section. Both
// modes render them — watch just never selects one. A terminal task
// has none by definition (history view, 2026-08-02): an unanswered
// escalation on a finished task is part of the record, so it renders
// in the History section instead of a section soliciting answers.
func (m topModel) detailEscalations() []escalationJSON {
	if m.snap == nil {
		return nil
	}
	h := m.snap.tasks[m.detailID]
	if terminalStatus(h.Task.Status) {
		return nil
	}
	var open []escalationJSON
	for _, e := range h.Escalations {
		if !e.Answered {
			open = append(open, e)
		}
	}
	sort.Slice(open, func(i, j int) bool { return open[i].ID < open[j].ID })
	return open
}

// detailFocusIdx clamps a stored focus to the current focusable set —
// content can shrink under a live refresh, exactly like detailScroll —
// returning -1 when nothing is focusable.
func detailFocusIdx(focus, n int) int {
	if n == 0 {
		return -1
	}
	if focus >= n {
		return n - 1
	}
	return focus
}

// viewedTask finds the detail screen's task in the state listing, for
// the p/c steering keys to target.
func (m topModel) viewedTask() (stateTask, bool) {
	if m.snap == nil {
		return stateTask{}, false
	}
	for _, t := range m.snap.state.Tasks {
		if t.ID == m.detailID {
			return t, true
		}
	}
	return stateTask{}, false
}

func (m topModel) selected() (topRow, bool) {
	if m.cursor >= len(m.rows) {
		return topRow{}, false
	}
	return m.rows[m.cursor], true
}

// submit turns the pending input into one steering write, run as a
// command so a slow daemon never freezes the view loop.
func (m topModel) submit() (tea.Model, tea.Cmd) {
	api, target, input := m.api, m.target, strings.TrimSpace(m.input.String())
	switch m.mode {
	case modeAnswer:
		if input == "" {
			m.status = "answer cannot be empty"
			return m, nil
		}
		m.mode, m.input, m.status = m.back, textInput{}, "answering…"
		return m, func() tea.Msg {
			if err := api.answerEscalation(target.esc.ID, input); err != nil {
				return actionMsg{err: err}
			}
			return actionMsg{desc: fmt.Sprintf("answered %q", oneLine(target.esc.Question))}
		}
	case modePriority:
		p, err := strconv.Atoi(input)
		if err != nil {
			m.status = fmt.Sprintf("priority must be an integer, got %q", input)
			return m, nil
		}
		m.mode, m.input, m.status = m.back, textInput{}, "updating…"
		return m, func() tea.Msg {
			if err := api.setPriority(target.task.ID, p); err != nil {
				return actionMsg{err: err}
			}
			return actionMsg{desc: fmt.Sprintf("set %s to p%d", shortID(target.task.ID), p)}
		}
	case modeConfirmCancel:
		m.mode, m.input, m.status = m.back, textInput{}, "cancelling…"
		return m, func() tea.Msg {
			if err := api.cancelTask(target.task.ID); err != nil {
				return actionMsg{err: err}
			}
			return actionMsg{desc: "cancelled " + shortID(target.task.ID)}
		}
	case modeCapture:
		if input == "" {
			m.status = "title cannot be empty"
			return m, nil
		}
		m.mode, m.input, m.status = m.back, textInput{}, "capturing…"
		return m, func() tea.Msg {
			if err := api.captureTask(input); err != nil {
				return actionMsg{err: err}
			}
			return actionMsg{desc: fmt.Sprintf("captured %q to inbox", input)}
		}
	case modeEditTitle:
		if input == "" {
			m.status = "title cannot be empty"
			return m, nil
		}
		if input == m.editWas { // unchanged: close without writing
			m.mode, m.input = m.back, textInput{}
			return m, nil
		}
		m.mode, m.input, m.status = m.back, textInput{}, "updating…"
		return m, func() tea.Msg {
			if err := api.setTitle(target.task.ID, input); err != nil {
				return actionMsg{err: err}
			}
			return actionMsg{desc: "updated title of " + shortID(target.task.ID)}
		}
	case modeEditDesc:
		// The description is kept byte-for-byte as typed — an editor
		// never trims — and emptying it is a legitimate clear, exactly
		// like `tuhdoo update --desc ""`.
		raw := m.input.String()
		if raw == m.editWas { // unchanged: close without writing
			m.mode, m.input = m.back, textInput{}
			return m, nil
		}
		m.mode, m.input, m.status = m.back, textInput{}, "updating…"
		return m, func() tea.Msg {
			if err := api.setDescription(target.task.ID, raw); err != nil {
				return actionMsg{err: err}
			}
			return actionMsg{desc: "updated description of " + shortID(target.task.ID)}
		}
	}
	return m, nil
}

// ---- rendering (pure over model state) ----

// detailLine is one rendered screen line of the task view, tagged with
// the stop index it belongs to (-1 for chrome and read-only lines) so
// selection reveal and click hit-testing work off the same bytes View
// draws.
type detailLine struct {
	text string
	stop int
}

// detailLines renders the task view on the dashboard's visual language
// (task-view rework, 2026-08-01): bold field names in the header block,
// dashboard section bars, and the task's open escalations as selectable
// rows under a NEEDS INPUT bar — the same gutter-bar/tint selection UI
// as the list. Open escalations live in that section alone; History
// keeps notes, runs, and answered escalations (the single-home rule the
// dashboard already follows). Returns nil if the task vanished under a
// refresh.
func (m topModel) detailLines() []detailLine {
	if m.snap == nil {
		return nil
	}
	h, ok := m.snap.tasks[m.detailID]
	if !ok {
		return nil
	}
	col, t := m.col, h.Task
	width := m.width
	if width <= 0 {
		width = 80 // no WindowSizeMsg yet: the mockup's design width
	}
	// The focus ring's stops share render order with this pass, so each
	// stop line takes the next index as it renders — the counter walks
	// detailStops' construction order exactly; watch mode has no stops
	// and every tag degrades to -1.
	stops := m.detailStops()
	nextIdx := 0
	nextStop := func() int {
		if len(stops) == 0 {
			return -1
		}
		i := nextIdx
		nextIdx++
		return i
	}
	var out []detailLine
	add := func(stop int, text string) {
		for _, l := range strings.Split(strings.TrimRight(wrapTo(text, width), "\n"), "\n") {
			out = append(out, detailLine{text: l, stop: stop})
		}
	}
	// Bars and escalation rows are already sized to the width; wrapping
	// them again would eat the bar's space fill.
	addRaw := func(stop int, text string) {
		for _, l := range strings.Split(text, "\n") {
			out = append(out, detailLine{text: l, stop: stop})
		}
	}
	// addBlock wraps to the mark column's inner width FIRST, then sets
	// every resulting screen line — wrapped continuations and blank
	// paragraph separators included — onto the two-cell mark column, so
	// a focused block's selection gutter is continuous and unfocused
	// continuations stay aligned (wrap-then-indent, 2026-08-02; the
	// escalationRow wrapIndent precedent). add() indents first and
	// wraps after, which strands continuations at column 0 — fine for
	// chrome, wrong for a stop.
	addBlock := func(stop int, text string) {
		inner := width - 2
		if inner < 10 {
			inner = 10
		}
		for _, l := range strings.Split(strings.TrimRight(wrapTo(text, inner), "\n"), "\n") {
			out = append(out, detailLine{text: "  " + l, stop: stop})
		}
	}

	// Header block: title line on the two-cell mark column (a focusable
	// stop carries the selection gutter there), then the field grid with
	// bold names.
	addBlock(nextStop(), sgr(col, col.bold, shortID(t.ID))+" — "+oneLine(t.Title))
	add(-1, "")
	field := func(stop int, name, value string) {
		add(stop, "  "+sgr(col, col.bold, name)+strings.Repeat(" ", 12-len(name))+value)
	}
	field(-1, "id", sgr(col, col.dim, shortID(t.ID)))
	status := views.HumanStatus(t.Status)
	switch {
	case h.Claim != nil:
		status += fmt.Sprintf(" — claimed by %s", h.Claim.Actor)
		if h.Claim.Expires != nil {
			status += fmt.Sprintf(" (lease expires %s)", stamp(*h.Claim.Expires))
		}
	// A terminal task's status line carries its close metadata (history
	// view, 2026-08-02) at day precision — the browse granularity; the
	// full instant lives on the ledger.
	case t.Status == "done" && t.ClosedAt != nil:
		status += fmt.Sprintf(" — finished %s by %s", dayStamp(*t.ClosedAt), t.ClosedBy)
	case t.Status == "cancelled" && t.ClosedAt != nil:
		status += fmt.Sprintf(" — %s by %s", dayStamp(*t.ClosedAt), t.ClosedBy)
	}
	field(-1, "status", status)
	prioStop := -1
	if !terminalStatus(t.Status) {
		prioStop = nextStop() // terminal tasks have no priority stop
	}
	field(prioStop, "priority", strconv.Itoa(t.Priority))
	if len(t.Labels) > 0 {
		field(-1, "labels", strings.Join(t.Labels, ", "))
	}
	if len(t.Parents) > 0 {
		field(-1, "parents", joinRefs(t.Parents, m.snap.taskRef))
	}
	if len(t.DependsOn) > 0 {
		field(-1, "depends on", joinRefs(t.DependsOn, m.snap.taskRef))
	}
	field(-1, "created", fmt.Sprintf("%s by %s", stamp(t.CreatedAt), t.CreatedBy))

	// The escalations section: every open escalation, selectable.
	if open := m.detailEscalations(); len(open) > 0 {
		add(-1, "")
		hint := ""
		if m.armed {
			hint = "enter answer "
		}
		addRaw(-1, barLine(col, col.bgMagenta, fmt.Sprintf(" NEEDS INPUT (%d)", len(open)), hint, width))
		for _, e := range open {
			addRaw(nextStop(), escalationRow(col, e, width))
		}
	}

	add(-1, "")
	addRaw(-1, barLine(col, col.rev+col.dim, " DESCRIPTION", "", width))
	if t.Description == "" {
		addBlock(nextStop(), sgr(col, col.dim, "none"))
	} else {
		addBlock(nextStop(), t.Description)
	}

	add(-1, "")
	addRaw(-1, barLine(col, col.rev+col.dim, " HISTORY", "", width))
	// History keeps answered escalations (the single-home rule) — and,
	// on a terminal task, the unanswered ones too (history view,
	// 2026-08-02): the NEEDS INPUT section only serves open work, and a
	// finished task's unanswered question is part of the record.
	hh := h
	hh.Escalations = nil
	for _, e := range h.Escalations {
		if e.Answered || terminalStatus(t.Status) {
			hh.Escalations = append(hh.Escalations, e)
		}
	}
	entries := historyOf(col, hh)
	if len(entries) == 0 {
		add(-1, "  "+sgr(col, col.dim, "no activity yet"))
	}
	for _, e := range entries {
		add(-1, strings.TrimRight(e.text, "\n"))
	}
	// The focused stop renders as the dashboard selection bar over its
	// full block — every line it tagged, wherever it landed.
	if focus := detailFocusIdx(m.detailFocus, len(stops)); focus >= 0 {
		for i, l := range out {
			if l.stop == focus {
				out[i].text = selectedText(col, l.text, width)
			}
		}
	}
	return out
}

// escalationRow renders one open escalation of the task view: badge
// cell and question (bold, wrapped), any context, then dim meta — all
// on the list's two-cell mark column, so the selection bar's ▌ gutter
// and tint read exactly like the dashboard's rows.
func escalationRow(col colors, e escalationJSON, width int) string {
	inner := width - 6
	if inner < 10 {
		inner = 10
	}
	wrapIndent := func(s string) []string {
		return strings.Split(strings.TrimRight(wrapTo(s, inner), "\n"), "\n")
	}
	badge, style := "", ""
	if e.Blocking {
		badge, style = "!", col.red+col.bold
	}
	var lines []string
	for i, q := range wrapIndent(oneLine(e.Question)) {
		if i == 0 {
			lines = append(lines, "  "+sgr(col, style, padTo(badge, 2))+"  "+sgr(col, col.bold, q))
			continue
		}
		lines = append(lines, "      "+sgr(col, col.bold, q))
	}
	if e.Context != "" {
		for _, c := range strings.Split(strings.TrimRight(e.Context, "\n"), "\n") {
			for _, l := range wrapIndent(c) {
				lines = append(lines, "      "+sgr(col, col.dim, l))
			}
		}
	}
	lines = append(lines, "      "+sgr(col, col.dim, fmt.Sprintf("%s · %s", e.Actor, stamp(e.RaisedAt))))
	return strings.Join(lines, "\n")
}

// detailBody is the task view's screen lines — detailLines stripped of
// its tags, for the scroll-window math.
func (m topModel) detailBody() []string {
	lines := m.detailLines()
	if lines == nil {
		return nil
	}
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = l.text
	}
	return out
}

// detailFocusLine is the first body line of the focused stop, or -1
// when nothing is focused.
func (m topModel) detailFocusLine() int {
	focus := detailFocusIdx(m.detailFocus, len(m.detailStops()))
	if focus < 0 {
		return -1
	}
	for i, l := range m.detailLines() {
		if l.stop == focus {
			return i
		}
	}
	return -1
}

// detailRevealScroll scrolls just enough — never more — to bring the
// focused stop's first line into the window.
func (m topModel) detailRevealScroll() int {
	scroll := m.detailScroll
	if m.height <= 0 {
		return scroll // no known height: everything renders anyway
	}
	line := m.detailFocusLine()
	if line < 0 {
		return scroll
	}
	if line < scroll {
		scroll = line
	}
	if w := m.detailWindow(); line >= scroll+w {
		scroll = line - w + 1
	}
	return scroll
}

// detailWindow is how many body lines fit: terminal height minus the
// lines detailView actually prints around the body — the header block
// (with its optional status line), the blank separator, and the footer,
// which is taller while an input box is open. Counting the rendered
// strings keeps the window honest against any footer shape.
func (m topModel) detailWindow() int {
	width := m.width
	if width <= 0 {
		width = 80
	}
	w := m.height - strings.Count(m.listHead(width), "\n") - 1 -
		strings.Count(m.detailFooter(), "\n")
	if w < 1 {
		w = 1
	}
	return w
}

// detailMaxScroll is the largest offset that still shows a full window
// (or the tail). Zero until the first WindowSizeMsg: with no known
// height everything renders and scrolling is meaningless.
func (m topModel) detailMaxScroll() int {
	if m.height <= 0 {
		return 0
	}
	max := len(m.detailBody()) - m.detailWindow()
	if max < 0 {
		return 0
	}
	return max
}

func (m topModel) detailView(body []string) string {
	width := m.width
	if width <= 0 {
		width = 80
	}
	var w strings.Builder
	// The same header bar as the list: one screen identity (task-view
	// rework, 2026-08-01).
	w.WriteString(m.listHead(width))
	scroll := m.detailScroll
	if max := m.detailMaxScroll(); scroll > max {
		scroll = max // content shrank under a live refresh
	}
	if m.height > 0 {
		end := scroll + m.detailWindow()
		if end > len(body) {
			end = len(body)
		}
		body = body[scroll:end]
	}
	w.WriteString(strings.Join(body, "\n"))
	w.WriteString("\n\n")
	w.WriteString(m.detailFooter())
	return w.String()
}

// detailFooter is the task view's bottom line: the live input prompt
// while one is open (so answering reads the same from either screen),
// else the same footer bar as the list — the armed legend advertises
// steering; watch mode keeps the read-only legend. An armed view
// always has a focus ring, so j/k always read "move"; "enter edit"
// covers the field stops, while the NEEDS INPUT bar keeps carrying
// "enter answer" for its own rows (the section-bar convention the
// dashboard uses).
func (m topModel) detailFooter() string {
	if f := m.inputFooter(); f != "" {
		return f
	}
	col := m.col
	width := m.width
	if width <= 0 {
		width = 80
	}
	legend := " ↑/↓ (j/k) scroll · esc back · q quit"
	if m.armed {
		legend = " ↑/↓ (j/k) move · enter edit · p priority · c cancel · esc back · q quit"
		if t, ok := m.viewedTask(); ok && terminalStatus(t.Status) {
			// p and c are dead on a closed record (history view,
			// 2026-08-02), so the legend stops advertising them.
			legend = " ↑/↓ (j/k) move · enter edit · esc back · q quit"
		}
	}
	return barLine(col, col.rev+col.dim, legend, "", width) + "\n"
}

func (m topModel) View() string {
	// The detail screen also owns the frame while an input mode opened
	// from it is live: the viewed task stays on screen, the prompt rides
	// the footer — same shape as answering from the list.
	if m.mode == modeDetail || (m.mode != modeNav && m.back == modeDetail) {
		if body := m.detailBody(); body != nil {
			return m.detailView(body)
		}
		// The task vanished under a refresh: fall through to the list.
	}
	col := m.col
	width := m.width
	if width <= 0 {
		width = 80 // no WindowSizeMsg yet: the mockup's design width
	}
	head := m.listHead(width)
	if m.err != nil {
		return head + wrapTo(fmt.Sprintf("%sdaemon unreachable:%s %v %s(retrying)%s\n",
			col.red, col.reset, m.err, col.dim, col.reset), m.width)
	}
	if m.snap == nil {
		return head + "loading...\n"
	}
	foot := "\n" + m.footerView(width)
	body := joinChunks(visibleChunks(m.listChunks(width), m.height,
		strings.Count(head, "\n"), strings.Count(foot, "\n")))
	return head + body + foot
}

// listHead renders the list screen's header block (header bar, optional
// status line, blank separator). One function feeds both View and the
// mouse hit test, so the body's first screen row is counted off the
// same bytes that get drawn.
func (m topModel) listHead(width int) string {
	col := m.col
	sync := "..."
	if m.snap != nil {
		sync = syncLine(m.snap.state.Sync)
	}
	badge := "watch mode"
	if m.armed {
		badge = "acting as " + m.actor
	}
	head := barLine(col, col.rev+col.bold, " tuhdoo · "+sync, badge+" ", width) + "\n"
	if m.status != "" {
		head += m.status + "\n"
	}
	return head + "\n"
}

// ---- the list screen (mock-a, 2026-07-31): bars and one column grid ----

// The shared column grid: mark(2) + id(6) + gap(2) + badge(2) + gap(2).
// Titles start at gridTitleCol; second lines indent to it.
const (
	gridIDW      = 6
	gridBadgeW   = 2
	gridTitleCol = 2 + gridIDW + 2 + gridBadgeW + 2
)

// topSection describes one dashboard section: which rows it collects,
// its bar color, whether its rows render dim, and the steering keys the
// bar advertises when the pane is armed.
type topSection struct {
	key   string
	label string
	bg    func(colors) string
	dim   bool // shelf sections: dim bar, dim rows
	hint  string
}

var topSections = []topSection{
	// "NEEDS INPUT", not "open escalations" (T7, 2026-07-30): the
	// entity keeps its name; the header alone softens the severity the
	// word overstates, and names no answerer — a future one may not be
	// a human.
	{"escalations", "NEEDS INPUT", func(c colors) string { return c.bgMagenta }, false, "enter answer"},
	{"ready", "READY", func(c colors) string { return c.bgGreen }, false, "p priority · c cancel"},
	{"inprogress", "IN PROGRESS", func(c colors) string { return c.bgYellow }, false, ""},
	{"blocked", "BLOCKED", func(c colors) string { return c.bgRed }, false, ""},
	// The shelves (2026-07-31): held above inbox, both dim — parked and
	// captured work sits below the live queue and never claims the eye.
	// No colored bars: reverse-dim reads as "present but not active".
	{"held", "ON HOLD", func(c colors) string { return c.rev + c.dim }, true, "c cancel"},
	{"inbox", "INBOX", func(c colors) string { return c.rev + c.dim }, true, "i capture · c cancel"},
}

// historySections are history mode's bars (history view, 2026-08-02):
// finished work first under the green bar, cancellations second under
// a reverse-dim one — closed, kept, never claiming the eye. No
// steering hints: the shelf is read-only in both panes.
var historySections = []topSection{
	{"done", "DONE", func(c colors) string { return c.bgGreen }, false, ""},
	{"cancelled", "CANCELLED", func(c colors) string { return c.rev + c.dim }, true, ""},
}

// sections is the on-screen list's section set.
func (m topModel) sections() []topSection {
	if m.history {
		return historySections
	}
	return topSections
}

// chunk is one atomic display unit — a bar, a one- or two-line row, a
// blank — that windowing never splits across the screen edge. row is
// the m.rows index a row chunk renders (-1 for chrome): it is stamped
// in the same pass that renders the row, which is what lets rowAt map
// a clicked line back to its row without re-deriving the layout.
type chunk struct {
	text   string // lines joined by \n, no trailing newline
	cursor bool
	row    int // index into m.rows, or -1 for bars, blanks, placeholders
}

func (c chunk) lines() int { return strings.Count(c.text, "\n") + 1 }

// sgr wraps text in one style, or returns it bare for zero-value
// colors (the NO_COLOR / non-TTY discipline).
func sgr(col colors, style, text string) string {
	if style == "" || text == "" {
		return text
	}
	return style + text + col.reset
}

// padTo left-justifies s in n cells (rune-counted).
func padTo(s string, n int) string {
	if d := n - len([]rune(s)); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

// barLine renders one full-width bar: left text, space fill, right
// text, the whole line under one style. Zero-value colors degrade to
// the plain text with the same geometry — no fill styling, layout
// intact. The right text is dropped before the left is truncated.
func barLine(col colors, style, left, right string, width int) string {
	l, r := []rune(left), []rune(right)
	if len(r) > 0 && len(l)+len(r)+1 > width {
		r = nil
	}
	if len(l) > width {
		l = []rune(ellipsize(string(l), width))
	}
	pad := width - len(l) - len(r)
	if pad < 0 {
		pad = 0
	}
	return sgr(col, style, string(l)+strings.Repeat(" ", pad)+string(r))
}

// fitTitle fits a title plus an optional suffix into width runes. The
// suffix loses first — dropped outright when the title is near-full,
// ellipsized into the remainder otherwise; the title wins and is
// ellipsized only once nothing else fits.
func fitTitle(title, suffix string, width int) (string, string) {
	t, s := []rune(title), []rune(suffix)
	if len(t)+len(s) <= width {
		return title, suffix
	}
	if len(t) >= width-10 {
		return ellipsize(title, width), ""
	}
	return title, ellipsize(suffix, width-len(t))
}

// gridRow renders one line on the shared column grid. suffix renders in
// suffixStyle (dim for labels and edges, yellow for holders) and is
// sacrificed for the title when the line is tight. Titles are bold in
// every section (visual hierarchy, 2026-07-31) — shelf rows keep their
// dim id, badge, and suffix but the title reads full-strength, an
// accepted consequence. Selection is no longer per-line styling: the
// chunk-level bar (selectedText) carries it.
func gridRow(col colors, id, badge, badgeStyle, title, suffix, suffixStyle string, width int) string {
	title, suffix = fitTitle(oneLine(title), suffix, width-gridTitleCol)
	return "  " +
		sgr(col, col.dim, padTo(id, gridIDW)) + "  " +
		sgr(col, badgeStyle, padTo(badge, gridBadgeW)) + "  " +
		sgr(col, col.bold, title) + sgr(col, suffixStyle, suffix)
}

// secondLine renders a row's indented second line: an optional colored
// lead ("waiting: ", "blocking") and dim text, ellipsized to the width.
func secondLine(col colors, lead, leadStyle, text string, width int) string {
	budget := width - gridTitleCol - len([]rune(lead))
	if budget < 2 {
		budget = 2
	}
	return strings.Repeat(" ", gridTitleCol) +
		sgr(col, leadStyle, lead) + sgr(col, col.dim, ellipsize(oneLine(text), budget))
}

// edgeText marks that a task is part of a structure — containment
// (parents) and scheduling (depends_on) — without imposing a tree on
// the flat list: edge semantics are still an open question, so rows
// only mark that edges exist.
func edgeText(s *snapshot, id string) string {
	t := s.tasks[id].Task
	var parts []string
	if n := len(t.Parents); n > 0 {
		p := "in " + shortID(t.Parents[0])
		if n > 1 {
			p += fmt.Sprintf(" +%d", n-1)
		}
		parts = append(parts, p)
	}
	if n := len(t.DependsOn); n > 0 {
		parts = append(parts, plural(n, "dep"))
	}
	if len(parts) == 0 {
		return ""
	}
	return "  · " + strings.Join(parts, " · ")
}

// closeSuffix is a history row's close stamp and closing actor, dim
// like the label suffix it follows, at day precision — the browse
// granularity; the full instant lives on the ledger. Empty when the
// snapshot carries no close metadata (a pre-upgrade daemon).
func closeSuffix(t stateTask) string {
	if t.ClosedAt == nil {
		return ""
	}
	s := "  · " + dayStamp(*t.ClosedAt)
	if t.ClosedBy != "" {
		s += " · " + t.ClosedBy
	}
	return s
}

// rowChunk renders one selectable row as an unsplittable chunk; the
// selected chunk is re-rendered as the full-height bar in one place,
// after its section shape is built.
func rowChunk(col colors, s *snapshot, r topRow, cursor bool, width int) chunk {
	var text string
	if r.kind == rowEscalation {
		// Task-shaped three-liner (grill cycle, 2026-07-31): title line
		// like every other section, the question on its own line, dim
		// meta. The red ! badge alone carries "blocking" — the word is
		// gone.
		e := r.esc
		badge, style := "", ""
		if e.Blocking {
			badge, style = "!", col.red+col.bold
		}
		et := s.tasks[e.Task].Task
		suffix := labelSuffix(et.Labels) + edgeText(s, e.Task)
		meta := fmt.Sprintf("%s · %s", e.Actor, stamp(e.RaisedAt))
		text = gridRow(col, shortID(e.Task), badge, style, et.Title, suffix, col.dim, width) +
			"\n" + secondLine(col, "question: ", col.magenta, e.Question, width) +
			"\n" + secondLine(col, "", "", meta, width)
	} else {
		t := r.task
		suffix := labelSuffix(t.Labels) + edgeText(s, t.ID)
		switch r.section {
		case "ready":
			badgeStyle := col.dim
			if t.Priority == 0 {
				badgeStyle = col.yellow
			}
			text = gridRow(col, shortID(t.ID), fmt.Sprintf("p%d", t.Priority),
				badgeStyle, t.Title, suffix, col.dim, width)
		case "inprogress":
			text = gridRow(col, shortID(t.ID), "", "", t.Title, "  ← "+t.Holder, col.yellow, width)
		case "held":
			// Priority is stored but inert while held (it bites again at
			// resume), so the badge renders — dim, like the rest of the row.
			text = gridRow(col, shortID(t.ID), fmt.Sprintf("p%d", t.Priority),
				col.dim, t.Title, suffix, col.dim, width)
		case "inbox":
			// No priority badge: an untriaged capture has no meaningful one.
			text = gridRow(col, shortID(t.ID), "", "", t.Title, suffix, col.dim, width)
		case "done", "cancelled":
			// History rows (history view, 2026-08-02): the ready-row
			// anatomy — title, dim labels, edge markers — plus a dim
			// close stamp and closing actor. No priority badge: recency
			// is the browse axis here, not priority.
			text = gridRow(col, shortID(t.ID), "", "", t.Title, suffix+closeSuffix(t), col.dim, width)
		default: // blocked
			text = gridRow(col, shortID(t.ID), "", "", t.Title, suffix, col.dim, width) +
				"\n" + secondLine(col, "waiting: ", col.red, s.blockedReasonTUI(t.ID, s.taskRef), width)
		}
	}
	if cursor {
		text = selectedText(col, text, width)
	}
	return chunk{text: text, cursor: cursor}
}

// listChunks builds the section bars and rows. The old summary-counts
// line is gone — the bars carry the counts.
func (m topModel) listChunks(width int) []chunk {
	col, s := m.col, m.snap
	var out []chunk
	if s.state.Degraded != "" {
		deg := wrapTo(fmt.Sprintf("%sDEGRADED (read-only):%s %s", col.red, col.reset, s.state.Degraded), width)
		out = append(out, chunk{text: strings.TrimRight(deg, "\n"), row: -1}, chunk{row: -1})
	}
	for si, sec := range m.sections() {
		if si > 0 {
			out = append(out, chunk{row: -1}) // blank line between sections
		}
		var idx []int
		for i, r := range m.rows {
			if r.section == sec.key {
				idx = append(idx, i)
			}
		}
		right := ""
		if m.armed && sec.hint != "" {
			right = sec.hint + " "
		}
		out = append(out, chunk{text: barLine(col, sec.bg(col),
			fmt.Sprintf(" %s (%d)", sec.label, len(idx)), right, width), row: -1})
		if len(idx) == 0 {
			out = append(out, chunk{text: "  " + sgr(col, col.dim, "none"), row: -1})
			continue
		}
		for _, i := range idx {
			c := rowChunk(col, s, m.rows[i], i == m.cursor, width)
			c.row = i
			out = append(out, c)
		}
	}
	return out
}

// visibleChunks slides a window over the chunks so the cursor's chunk
// is fully on screen (pinned toward the bottom edge once the list
// outgrows the terminal); chunks never split across the window edge.
// Unknown height shows everything. Returning chunks — not the joined
// string — is what lets View and rowAt consume the identical window.
func visibleChunks(chunks []chunk, height, headLines, footLines int) []chunk {
	total, cursorIdx := 0, 0
	for i, c := range chunks {
		total += c.lines()
		if c.cursor {
			cursorIdx = i
		}
	}
	if height <= 0 {
		return chunks
	}
	avail := height - headLines - footLines
	if avail < 1 {
		avail = 1
	}
	if total <= avail {
		return chunks
	}
	linesThrough := func(from, to int) int {
		n := 0
		for i := from; i <= to; i++ {
			n += chunks[i].lines()
		}
		return n
	}
	start := 0
	for start < cursorIdx && linesThrough(start, cursorIdx) > avail {
		start++
	}
	var out []chunk
	used := 0
	for i := start; i < len(chunks); i++ {
		n := chunks[i].lines()
		if used+n > avail && i > start {
			break
		}
		out = append(out, chunks[i])
		used += n
	}
	return out
}

// joinChunks renders a chunk window to screen bytes, one trailing
// newline per chunk line.
func joinChunks(cs []chunk) string {
	var b strings.Builder
	for _, c := range cs {
		b.WriteString(c.text)
		b.WriteString("\n")
	}
	return b.String()
}

// shortID abbreviates a task ID for TUI display: the ID's own type
// prefix (everything through the first hyphen — `tuh-` for tasks
// minted after the 2026-07-31 rebrand, `t-` for older ones) plus the
// ULID's last four characters, lowercased (`tuh-d83w`). The tail is
// where same-batch ULIDs actually differ — their timestamp prefixes
// match — so abbreviation comes from the right-hand end. Display and
// input sugar only (T7): stored and transmitted IDs stay full-length,
// and the full ULID has no TUI surface at all (2026-08-02 revision) —
// a human who needs it runs one-shot `tuhdoo task <fragment>`.
func shortID(id string) string {
	i := strings.Index(id, "-")
	tail := id[i+1:]
	if len(tail) <= 4 {
		return id
	}
	return id[:i+1] + strings.ToLower(tail[len(tail)-4:])
}

// inputFooter is the active input prompt, or "" when no input mode is
// live — shared by the list and detail footers so each steering write
// reads identically from either screen. Text entry renders through the
// shared widget's box (textinput.go): header bar naming the entry, the
// buffer with the cursor, and the hint fixed on its own line; only the
// cancel y/n confirm — not a text input — keeps its one-liner.
func (m topModel) inputFooter() string {
	col := m.col
	switch m.mode {
	case modeAnswer:
		return m.input.view(col, "answer · "+oneLine(m.target.esc.Question), "submits", m.width)
	case modePriority:
		label := fmt.Sprintf("priority %s (%s)", shortID(m.target.task.ID), oneLine(m.target.task.Title))
		return m.input.view(col, label, "submits", m.width)
	case modeConfirmCancel:
		return wrapTo(fmt.Sprintf("%scancel%s %s (%s)? y/n %s— history stays on the ledger%s\n",
			col.bold, col.reset, shortID(m.target.task.ID), oneLine(m.target.task.Title),
			col.dim, col.reset), m.width)
	case modeCapture:
		// No y/n: capture is cheap by design, and cancel reverses it.
		return m.input.view(col, "capture (to inbox)", "captures", m.width)
	case modeEditTitle:
		// The box carries the title being edited; the label only needs
		// to name whose it is.
		return m.input.view(col, "title "+shortID(m.target.task.ID), "saves", m.width)
	case modeEditDesc:
		label := fmt.Sprintf("description %s (%s)", shortID(m.target.task.ID), oneLine(m.target.task.Title))
		return m.input.view(col, label, "saves", m.width)
	}
	return ""
}

// footerView is the footer bar (key legend left, done tally right), or
// the active input prompt, full-width like the header.
func (m topModel) footerView(width int) string {
	col := m.col
	if f := m.inputFooter(); f != "" {
		return f
	}
	if m.history {
		// No steering keys and no done tally: the DONE bar above
		// already carries the count.
		return barLine(col, col.rev+col.dim,
			" ↑/↓ (j/k) move · enter open · esc back · q quit", "", width) + "\n"
	}
	// "enter open" on every row: a Needs Input row opens its task's view
	// with the question preselected — answering happens there, with the
	// task's context on screen (task-view rework, 2026-08-01).
	legend := " ↑/↓ (j/k) move · enter open · h history · q quit"
	if m.armed {
		legend = " ↑/↓ (j/k) move · enter open · p priority · c cancel · h history · q quit"
	}
	done := ""
	if m.snap != nil {
		done = fmt.Sprintf("%d done ", len(m.snap.classify().done))
	}
	return barLine(col, col.rev+col.dim, legend, done, width) + "\n"
}

// ---- entry point ----

// topActor resolves the acting human principal for steer mode: an
// explicit --as wins; otherwise it derives from git identity per D7
// (the local part of user.email). The TUI steers as a root human,
// never as an agent.
func topActor(as string) (string, error) {
	if as == "" {
		local, err := gitEmailLocalPart("")
		if err != nil {
			return "", fmt.Errorf("cannot derive your principal from git identity: %v; run: tuhdoo --as <human>", err)
		}
		as = local
	}
	if err := daemon.ValidateActor(as); err != nil {
		return "", err
	}
	if strings.Contains(as, "/") {
		return "", fmt.Errorf("the TUI steers as a human root principal, not an agent: want e.g. %q, got %q",
			strings.SplitN(as, "/", 2)[0], as)
	}
	return as, nil
}

// runTUI is bare `tuhdoo`: parse the flags, guard the launch, run the
// TUI. Watch mode never acts, so it needs no principal at all.
func runTUI(args []string) int {
	fs := flag.NewFlagSet("tuhdoo", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // errors get our usage line, not the pkg default
	var watch, w bool
	var as string
	fs.BoolVar(&watch, "watch", false, "")
	fs.BoolVar(&w, "w", false, "")
	fs.StringVar(&as, "as", "", "")
	if err := fs.Parse(args); err != nil || fs.NArg() > 0 {
		fmt.Fprintln(os.Stderr, "usage: tuhdoo [-w|--watch] [--as <human>]")
		return 1
	}
	watch = watch || w
	if watch && as != "" {
		fmt.Fprintln(os.Stderr, "tuhdoo: --as means nothing in watch mode; a disarmed pane never acts")
		return 1
	}
	// Guarded launch (T7): the TUI wants a terminal; a pipe, script, or
	// CI run gets usage instead — what bare invocation always printed.
	if !isTTY(os.Stdout) {
		usage(os.Stderr)
		return 1
	}
	m := topModel{armed: !watch, col: newColors(os.Stdout)}
	if m.col.reset != "" {
		// Colored TTY: resolve the selection-bar tint down the
		// capability ladder (selection.go), querying the terminal once —
		// before bubbletea owns stdin. NO_COLOR never reaches here: the
		// ▌ glyph alone marks selection there.
		ans, dark := queryTermBG(os.Stdout)
		m.col.selBG = selectionBG(ans, os.Getenv("TERM"), os.Getenv("COLORTERM"), dark)
	}
	if m.armed {
		actor, err := topActor(as)
		if err != nil {
			fmt.Fprintln(os.Stderr, "tuhdoo:", err)
			return 1
		}
		m.actor = actor
	}
	_, c, code := connect()
	if code != 0 {
		return code
	}
	m.c = c
	if m.armed {
		m.api = httpSteering{c: c, actor: m.actor}
	}
	// Mouse tracking is armed-only (T7, 2026-07-31): tracking captures
	// the pointer, so terminal-native text selection needs shift-click
	// while it is on — a real cost over SSH/tmux. The watch pane — the
	// one left open to read and copy from — never enables it, keeping
	// plain click-drag selection there; the model still guards clicks it
	// would never see (updateMouse via openRow: select and read-only
	// detail only, never input).
	opts := []tea.ProgramOption{tea.WithAltScreen()}
	if m.armed {
		opts = append(opts, tea.WithMouseCellMotion())
	}
	if _, err := tea.NewProgram(m, opts...).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "tuhdoo:", err)
		return 1
	}
	return 0
}
