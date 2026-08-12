package main

// The interactive TUI (002 T7, revised by Cycle 4): the single live
// human surface. Reads poll the daemon on a tick; the steering writes
// (answer an escalation, reprioritize, cancel, capture, edit
// title/description/labels) go through the daemon HTTP API only,
// stamped with the acting human principal.
// Watch mode is the same screen disarmed: steering keys dead, fixed at
// launch — no keypress can re-arm a disarmed pane.

import (
	"flag"
	"fmt"
	"github.com/brandonbews/tuhdoo/internal/event"
	"io"
	"os"
	"slices"
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
	setLabels(task string, labels []string) error
}

// httpSteering implements steeringAPI over the daemon's JSON HTTP API,
// reusing the admin tools that already exist there: no new write paths.
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

// setLabels writes the full replacement list — the same PATCH, and so
// the same task.updated ledger event, as `tuhdoo update --labels`
// (labels editable, 2026-08-05). An empty list is a legitimate clear;
// no value is weight-bearing to the platform (D5 label agnosticism).
func (s httpSteering) setLabels(task string, labels []string) error {
	return s.c.write("PATCH", "/v0/tasks/"+task, s.actor, map[string]any{"labels": labels})
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
		if len(t.UnmetDeps) == 0 {
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
	modeEditTitle  // task-view edits (2026-08-01): the widget prefilled
	modeEditDesc   // with the current value; unchanged submit writes nothing
	modeEditLabels // labels as one comma-joined line (2026-08-05), same rules
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

	detailID     string   // task shown by modeDetail
	detailScroll int      // first visible body line in modeDetail
	detailFocus  int      // focused stop in an armed detail (index into detailStops)
	detailBack   []string // task views walked through by edge hops (edge rows, 2026-08-11): esc pops, newest last — a plain slice, boring Go
	// escExpanded holds the escalation contexts toggled open in the task
	// view (escalation readability, 2026-08-11), keyed by escalation ID —
	// ephemeral view state: cleared when the view closes, never persisted.
	escExpanded map[string]bool
	width       int // terminal columns; 0 (no WindowSizeMsg yet) wraps nothing
	height      int // terminal rows; 0 renders all
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
		// spells its verb again.
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
	m.detailBack = nil  // a fresh entry from the list starts a fresh hop stack
	m.escExpanded = nil // …and every context back at its collapsed stub
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

// renderWidth is the width every render pass draws to: the terminal's,
// or 80 — the mockup's design width — before the first WindowSizeMsg.
func (m topModel) renderWidth() int {
	if m.width <= 0 {
		return 80
	}
	return m.width
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
	width := m.renderWidth()
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
	width := m.renderWidth()
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
		// The edge-hop back stack (edge rows, 2026-08-11): esc pops to
		// the task view the hop came from; from the first task it steps
		// back to the list — dashboard or history, whichever is under it.
		if n := len(m.detailBack); n > 0 {
			m.detailID, m.detailBack = m.detailBack[n-1], m.detailBack[:n-1]
			m.detailScroll, m.detailFocus = 0, 0
		} else {
			m.mode, m.detailID, m.detailScroll, m.detailFocus = modeNav, "", 0, 0
			m.escExpanded = nil // the toggle state dies with the view
		}
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
	case "e":
		// The context toggle (escalation readability, 2026-08-11): an
		// escalation's context block trades places with its collapsed
		// stub. With an escalation stop focused, e flips that one; watch
		// mode has no focus ring, so e flips every open escalation of the
		// view — expansion is reading, and a disarmed pane still reads.
		// Armed with focus elsewhere, e does nothing: the ring is the
		// selector. (The one-day e/E edit chords retired 2026-08-02 stay
		// retired; this is a new binding on the freed key.)
		var ids []string
		if focus >= 0 && stops[focus].kind == stopEscalation {
			ids = []string{stops[focus].esc.ID}
		} else if !m.armed {
			for _, e := range m.detailEscalations() {
				ids = append(ids, e.ID)
			}
		}
		if len(ids) > 0 && m.escExpanded == nil {
			m.escExpanded = map[string]bool{}
		}
		for _, id := range ids {
			m.escExpanded[id] = !m.escExpanded[id]
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

// The task view's focusable stops, in render order: title, the
// priority and labels meta lines, each open escalation, each edge row
// (DEPENDS ON, then NEEDED BY — edge rows, 2026-08-11), then the
// description body. Bars and the read-only meta lines (id, status,
// created) are never stops.
const (
	stopTitle       = "title"
	stopPriority    = "priority"
	stopLabels      = "labels"
	stopEscalation  = "escalation"
	stopDep         = "dep"      // one DEPENDS ON row; enter opens the target task's view
	stopNeededBy    = "neededby" // one NEEDED BY row; the same hop along the reverse edge
	stopDescription = "description"
)

// detailStop is one focusable field of the task view.
type detailStop struct {
	kind string
	esc  escalationJSON // set when kind == stopEscalation
	task string         // edge target, set when kind == stopDep or stopNeededBy
}

// detailStops is the focus ring: title, the priority and labels meta
// lines, each open escalation, each edge row, then the description
// body — the same order they render in, which is what lets detailLines
// tag lines by position. Empty in watch mode: the disarmed pane stays
// fully read-only, so it has no focus to act on. A terminal task loses
// the priority and labels stops (history view, 2026-08-02; labels
// editable, 2026-08-05): a closed record is browsed, not steered —
// but keeps its edge stops, because an edge hop is navigation, not
// steering. An edge that does not resolve in the snapshot is never a
// stop: it has no view to open, so its row stays plain (the taskRef
// never-invent rule, applied to selection).
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
		stops = append(stops, detailStop{kind: stopPriority}, detailStop{kind: stopLabels})
	}
	for _, e := range m.detailEscalations() {
		stops = append(stops, detailStop{kind: stopEscalation, esc: e})
	}
	for _, dep := range h.Task.DependsOn {
		if _, ok := m.snap.findTask(dep); ok {
			stops = append(stops, detailStop{kind: stopDep, task: dep})
		}
	}
	for _, id := range m.snap.dependentsOf(m.detailID) {
		stops = append(stops, detailStop{kind: stopNeededBy, task: id})
	}
	return append(stops, detailStop{kind: stopDescription})
}

// openStop opens the focused stop's editor: title and description
// prefilled through the shared widget (unchanged submit writes
// nothing — the editWas rule), priority as the numeric input, an
// escalation as answer entry. One opener for enter and the mouse —
// which is how click-to-open on edge rows falls out of the existing
// machinery for free.
func (m topModel) openStop(s detailStop) (tea.Model, tea.Cmd) {
	// An edge stop navigates instead of editing (edge rows, 2026-08-11):
	// push the current task and open the target's view in place; esc
	// pops back through the visited views (updateDetail).
	if s.kind == stopDep || s.kind == stopNeededBy {
		m.detailBack = append(m.detailBack, m.detailID)
		m.detailID, m.detailScroll, m.detailFocus, m.status = s.task, 0, 0, ""
		return m, nil
	}
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
	case stopLabels:
		// One comma-joined line ("tui, design"): the same list syntax as
		// `tuhdoo update --labels`, edited in place. A comma inside a
		// label is unrepresentable on every surface — accepted.
		labels := strings.Join(m.snap.tasks[t.ID].Task.Labels, ", ")
		m.mode, m.back = modeEditLabels, modeDetail
		m.target, m.input, m.editWas, m.status = topRow{kind: rowTask, task: t}, editInput(labels, false), labels, ""
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
			return actionMsg{desc: fmt.Sprintf("set %s to p%d", event.ShortID(target.task.ID), p)}
		}
	case modeConfirmCancel:
		m.mode, m.input, m.status = m.back, textInput{}, "cancelling…"
		return m, func() tea.Msg {
			if err := api.cancelTask(target.task.ID); err != nil {
				return actionMsg{err: err}
			}
			return actionMsg{desc: "cancelled " + event.ShortID(target.task.ID)}
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
			return actionMsg{desc: "updated title of " + event.ShortID(target.task.ID)}
		}
	case modeEditLabels:
		// The raw editWas guard first, like every edit mode; then the
		// semantic one: parse with splitList (split on commas, trim, drop
		// empties — no dedup, no case-folding: store what was typed) and
		// compare element-wise to the task's current list, so respacing
		// is a no-op while reordering is a real edit. An empty submit
		// clears every label — the CLI's explicit-empty --labels
		// precedent.
		if input == m.editWas {
			m.mode, m.input = m.back, textInput{}
			return m, nil
		}
		labels := splitList(input)
		if slices.Equal(labels, m.snap.tasks[target.task.ID].Task.Labels) {
			m.mode, m.input = m.back, textInput{}
			return m, nil
		}
		m.mode, m.input, m.status = m.back, textInput{}, "updating…"
		return m, func() tea.Msg {
			if err := api.setLabels(target.task.ID, labels); err != nil {
				return actionMsg{err: err}
			}
			return actionMsg{desc: "updated labels of " + event.ShortID(target.task.ID)}
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
			return actionMsg{desc: "updated description of " + event.ShortID(target.task.ID)}
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
	width := m.renderWidth()
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
	addBlock(nextStop(), sgr(col, col.bold, event.ShortID(t.ID))+" — "+oneLine(t.Title))
	add(-1, "")
	field := func(stop int, name, value string) {
		add(stop, "  "+sgr(col, col.bold, name)+strings.Repeat(" ", 12-len(name))+value)
	}
	field(-1, "id", sgr(col, col.dim, event.ShortID(t.ID)))
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
	// The labels line always renders — dim none placeholder when empty,
	// the description-body pattern — armed, watch, and terminal alike
	// (labels editable, 2026-08-05): one uniform rule.
	labelsStop := -1
	if !terminalStatus(t.Status) {
		labelsStop = nextStop() // terminal tasks: rendered, never a stop
	}
	labels := sgr(col, col.dim, "none")
	if len(t.Labels) > 0 {
		labels = strings.Join(t.Labels, ", ")
	}
	field(labelsStop, "labels", labels)
	// Loud blockage annotations only (2026-08-05 edge grill): the line
	// renders when a human must act — loop membership, cancelled deps —
	// never for ordinary waiting, which the depends-on refs already tell.
	if wn := waitingNote(m.snap.stateTaskOf(t.ID), m.snap.taskRef); wn != "" {
		field(-1, "waiting", wn)
	}
	field(-1, "created", fmt.Sprintf("%s by %s", stamp(t.CreatedAt), t.CreatedBy))

	// The escalations section: every open escalation, selectable. The
	// context toggle is a section key, so its hint rides the section bar
	// (the section-bar convention the dashboard uses) — the bottom footer
	// legend is already full at the 80-column design width. Watch mode
	// keeps the toggle (expansion is reading), so its bar carries the
	// hint too, alone.
	if open := m.detailEscalations(); len(open) > 0 {
		add(-1, "")
		hint := "e context "
		if m.armed {
			hint = "enter answer · e context "
		}
		addRaw(-1, barLine(col, col.bgMagenta, fmt.Sprintf(" NEEDS INPUT (%d)", len(open)), hint, width))
		for _, e := range open {
			addRaw(nextStop(), escalationRow(col, e, m.escExpanded[e.ID], width))
		}
	}

	// The edge sections (2026-08-11 grill): DEPENDS ON — the old
	// comma-joined field line de-blobbed into one row per edge, because
	// a container's dep list read top-to-bottom is its progress
	// checklist and the status word is the glanceable signal — then
	// NEEDED BY, the reverse edges computed at render from the snapshot
	// (no stored reverse index). Every dependent renders regardless of
	// status (accuracy over noise; the dim status word carries the
	// story), in ULID order. Rows whose target resolves are selectable
	// stops: enter hops to the target's view, esc walks back.
	deps, dependents := t.DependsOn, m.snap.dependentsOf(t.ID)
	if len(deps) > 0 || len(dependents) > 0 {
		idW, statusW := edgeCols(m.snap, deps, dependents)
		hint := ""
		if m.armed {
			hint = "enter open "
		}
		section := func(label string, ids []string) {
			if len(ids) == 0 {
				return
			}
			add(-1, "")
			addRaw(-1, barLine(col, col.rev+col.dim, fmt.Sprintf(" %s (%d)", label, len(ids)), hint, width))
			for _, id := range ids {
				et, ok := m.snap.findTask(id)
				stop := -1
				if ok {
					stop = nextStop()
				}
				addRaw(stop, edgeRow(col, idW, statusW, id, et, ok, width))
			}
		}
		section("DEPENDS ON", deps)
		section("NEEDED BY", dependents)
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
// cell and question (bold, wrapped, line structure kept — escalation
// readability, 2026-08-11: the agent's A)/B) options and recommendation
// survive, where oneLine used to flatten them), any context, then dim
// meta — all on the list's two-cell mark column, so the selection bar's
// ▌ gutter and tint read exactly like the dashboard's rows.
//
// The context collapses to a stub by default — its first screen line
// plus a hidden-line count — so a fat context (old-style escalations
// included: no migration, no special case) never buries the view;
// expanded renders the full block. Contexts that fit one screen line
// render as-is: nothing to hide, no stub. The count is screen lines at
// this width — exactly what expanding reveals.
// blockingBadge is the red ! badge (and its style) an escalation row
// carries when blocking — the badge alone carries "blocking"; the word
// is banned on this surface. Empty for non-blocking.
func blockingBadge(col colors, blocking bool) (badge, style string) {
	if blocking {
		return "!", col.red + col.bold
	}
	return "", ""
}

func escalationRow(col colors, e escalationJSON, expanded bool, width int) string {
	inner := width - 6
	if inner < 10 {
		inner = 10
	}
	wrapIndent := func(s string) []string {
		return strings.Split(strings.TrimRight(wrapTo(s, inner), "\n"), "\n")
	}
	badge, style := blockingBadge(col, e.Blocking)
	var lines []string
	first := true
	for _, q := range strings.Split(strings.TrimRight(e.Question, "\n"), "\n") {
		for _, l := range wrapIndent(q) {
			if first {
				lines = append(lines, "  "+sgr(col, style, padTo(badge, 2))+"  "+sgr(col, col.bold, l))
				first = false
				continue
			}
			lines = append(lines, "      "+sgr(col, col.bold, l))
		}
	}
	if e.Context != "" {
		var ctx []string
		for _, c := range strings.Split(strings.TrimRight(e.Context, "\n"), "\n") {
			ctx = append(ctx, wrapIndent(c)...)
		}
		if !expanded && len(ctx) > 1 {
			ctx = []string{ctx[0], fmt.Sprintf("(+%s — e to expand)", plural(len(ctx)-1, "line"))}
		}
		for _, l := range ctx {
			lines = append(lines, "      "+sgr(col, col.dim, l))
		}
	}
	lines = append(lines, "      "+sgr(col, col.dim, fmt.Sprintf("%s · %s", e.Actor, stamp(e.RaisedAt))))
	return strings.Join(lines, "\n")
}

// edgeCols derives the shared columns of the task view's edge rows —
// the widest short ID and human status word across both sections — so
// DEPENDS ON and NEEDED BY read as one aligned grid.
func edgeCols(s *snapshot, deps, dependents []string) (idW, statusW int) {
	for _, ids := range [][]string{deps, dependents} {
		for _, id := range ids {
			if n := len([]rune(event.ShortID(id))); n > idW {
				idW = n
			}
			if t, ok := s.findTask(id); ok {
				if n := len([]rune(views.HumanStatus(t.Status))); n > statusW {
					statusW = n
				}
			}
		}
	}
	return idW, statusW
}

// edgeRow renders one edge of the task view on the two-cell mark
// column: bold short ID, dim status word, plain title — hard-truncated
// to one line with an ellipsis, never wrapped (edge rows, 2026-08-11;
// chosen over id+title alone because the status word is the glanceable
// signal of a dep checklist). An edge the snapshot cannot resolve
// renders its bare short ID — never an invented status (the taskRef
// rule).
func edgeRow(col colors, idW, statusW int, id string, t stateTask, ok bool, width int) string {
	short := event.ShortID(id)
	if !ok {
		return "  " + sgr(col, col.bold, short)
	}
	budget := width - 2 - idW - 2 - statusW - 2
	if budget < 2 {
		budget = 2
	}
	return "  " + sgr(col, col.bold, padTo(short, idW)) + "  " +
		sgr(col, col.dim, padTo(views.HumanStatus(t.Status), statusW)) + "  " +
		ellipsize(oneLine(t.Title), budget)
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
	width := m.renderWidth()
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
	width := m.renderWidth()
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
	// Same bottom-pinning as the list (chrome hierarchy, 2026-08-03):
	// detailWindow already reserves the footer rows, so only the
	// shortfall of a short body pads. detailStopAt needs no padding
	// awareness — clicks below the body already miss every stop.
	return pinFrame(w.String(), m.detailWindow()-len(body), m.detailFooter(), m.height)
}

// pinFrame finishes a screen: content, pad blank lines so the footer
// pins to the bottom row, footer last. The pinned frame must be exactly
// height split-lines with the footer unterminated: bubbletea splits the
// view on \n — a trailing newline is an extra (empty) line — and drops
// any overflow from the TOP, so an unstripped final newline costs the
// header row. Before the first WindowSizeMsg (height <= 0) the footer
// floats unpadded and the frame keeps its trailing newline.
func pinFrame(content string, pad int, footer string, height int) string {
	if height > 0 {
		if pad > 0 {
			content += strings.Repeat("\n", pad)
		}
		return strings.TrimSuffix(content+footer, "\n")
	}
	return content + footer
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
	width := m.renderWidth()
	legend := [][2]string{{"↑/↓ (j/k)", "scroll"}, {"esc", "back"}, {"q", "quit"}}
	if m.armed {
		legend = [][2]string{{"↑/↓ (j/k)", "move"}, {"enter", "edit"},
			{"p", "priority"}, {"c", "cancel"}, {"esc", "back"}, {"q", "quit"}}
		if t, ok := m.viewedTask(); ok && terminalStatus(t.Status) {
			// p and c are dead on a closed record (history view,
			// 2026-08-02), so the legend stops advertising them.
			legend = [][2]string{{"↑/↓ (j/k)", "move"}, {"enter", "edit"},
				{"esc", "back"}, {"q", "quit"}}
		}
	}
	return legendLine(col, legend, "", width) + "\n"
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
	width := m.renderWidth()
	head := m.listHead(width)
	if m.err != nil {
		return head + wrapTo(fmt.Sprintf("%sdaemon unreachable:%s %v %s(retrying)%s\n",
			col.red, col.reset, m.err, col.dim, col.reset), m.width)
	}
	if m.snap == nil {
		return head + "loading...\n"
	}
	foot := "\n" + m.footerView(width)
	headN, footN := strings.Count(head, "\n"), strings.Count(foot, "\n")
	body := joinChunks(visibleChunks(m.listChunks(width), m.height, headN, footN))
	// The footer — or the live input prompt riding in its place — pins
	// to the bottom row (chrome hierarchy, 2026-08-03). rowAt needs no
	// padding awareness: the pad sits below every row chunk, where
	// clicks already miss.
	return pinFrame(head+body, m.height-headN-footN-strings.Count(body, "\n"), foot, m.height)
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
	// Unfilled header (chrome hierarchy, 2026-08-03): the frame stops
	// competing with content. tuhdoo bold, the sync text dim, the badge
	// right-aligned — watch mode dim, acting-as at normal weight (armed
	// must be glanceable).
	badge := []seg{{col.dim, "watch mode "}}
	if m.armed {
		badge = []seg{{"", "acting as " + m.actor + " "}}
	}
	head := segLine(col,
		[]seg{{col.bold, " tuhdoo"}, {col.dim, " · " + sync}},
		badge, width) + "\n"
	if m.status != "" {
		head += m.status + "\n"
	}
	return head + "\n"
}

// ---- the list screen (mock-a, 2026-07-31): bars and one column grid ----

// The shared column grid: mark(2) + id + gap(2) + badge(2) + gap(2).
// The ID column is derived per snapshot (idColWidth) with gridIDW as
// its floor; titles start at gridTitleCol(idW) and every second line
// indents to it.
const (
	gridIDW    = 6
	gridBadgeW = 2
)

// gridTitleCol is the column titles start at — derived, not const
// (two-line rows, 2026-08-05): the ID column widens with the
// snapshot's widest short ID.
func gridTitleCol(idW int) int { return 2 + idW + 2 + gridBadgeW + 2 }

// idColWidth derives the ID column width: the widest short ID across
// every task in the snapshot, floored at gridIDW. Snapshot-stable —
// never per-window — so the column cannot jitter while scrolling:
// migrated t- IDs render 6 wide, minted tuh- IDs 8, and a mixed list
// keeps one title column (gutter alignment, 2026-08-05).
func idColWidth(s *snapshot) int {
	w := gridIDW
	for _, t := range s.state.Tasks {
		if n := len([]rune(event.ShortID(t.ID))); n > w {
			w = n
		}
	}
	return w
}

// topSection describes one dashboard section: which rows it collects,
// its bar color, and the steering keys the bar advertises when the
// pane is armed.
type topSection struct {
	key   string
	label string
	bg    func(colors) string
	hint  string
}

var topSections = []topSection{
	// "NEEDS INPUT", not "open escalations" (T7, 2026-07-30): the
	// entity keeps its name; the header alone softens the severity the
	// word overstates, and names no answerer — a future one may not be
	// a human.
	{"escalations", "NEEDS INPUT", func(c colors) string { return c.bgMagenta }, "enter answer"},
	{"ready", "READY", func(c colors) string { return c.bgGreen }, "p priority · c cancel"},
	{"inprogress", "IN PROGRESS", func(c colors) string { return c.bgYellow }, ""},
	// BLOCKED's bgRed went dim red (bar recolors, 2026-08-04): the
	// section holds only unmet-dependency tasks — ordinary sequencing,
	// not a fire — so the bar keeps the hue and drops the alarm.
	{"blocked", "BLOCKED", func(c colors) string { return c.bgRed }, ""},
	// The shelves (2026-07-31): held above inbox, both dim rows — parked
	// and captured work sits below the live queue and never claims the
	// eye. Chrome hierarchy (2026-08-03): held is shelved and takes the
	// dark-gray bgGray bar. Bar recolors (2026-08-04): inbox awaits
	// attention and takes the bright-white bgWhite bar (was reverse-dim)
	// — its rows stay dim, but the bar no longer reads as shelf chrome.
	{"held", "ON HOLD", func(c colors) string { return c.bgGray }, "c cancel"},
	{"inbox", "INBOX", func(c colors) string { return c.bgWhite }, "i capture · c cancel"},
}

// historySections are history mode's bars (history view, 2026-08-02):
// finished work first under the green bar, cancellations second under
// the shelf's dark-gray bar (chrome hierarchy, 2026-08-03) — closed,
// kept, never claiming the eye. No steering hints: the shelf is
// read-only in both panes.
var historySections = []topSection{
	{"done", "DONE", func(c colors) string { return c.bgGreen }, ""},
	{"cancelled", "CANCELLED", func(c colors) string { return c.bgGray }, ""},
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

// seg is one styled span of a composed chrome line. barLine styles a
// whole line at once and pads by rune count over the raw string, so
// embedding SGRs in its text would wreck the geometry; the unfilled
// chrome (header, footer — chrome hierarchy, 2026-08-03) mixes styles
// on one line, so segLine tracks visible width per segment instead.
type seg struct{ style, text string }

// segWidth is the visible width of a segment list in cells
// (rune-counted; styles occupy none).
func segWidth(segs []seg) int {
	n := 0
	for _, s := range segs {
		n += len([]rune(s.text))
	}
	return n
}

// segLine composes one full-width unfilled line: left segments, a plain
// space fill, right segments. barLine's fitting rules hold — the right
// group is dropped whole before the left is truncated — and zero-value
// colors degrade every segment to bare text with identical geometry.
func segLine(col colors, left, right []seg, width int) string {
	lw, rw := segWidth(left), segWidth(right)
	if rw > 0 && lw+rw+1 > width {
		right, rw = nil, 0
	}
	if lw > width {
		left = truncSegs(left, width)
		lw = segWidth(left)
	}
	pad := width - lw - rw
	if pad < 0 {
		pad = 0
	}
	var b strings.Builder
	for _, s := range left {
		b.WriteString(sgr(col, s.style, s.text))
	}
	b.WriteString(strings.Repeat(" ", pad))
	for _, s := range right {
		b.WriteString(sgr(col, s.style, s.text))
	}
	return b.String()
}

// truncSegs cuts a segment list to width cells, ellipsizing inside the
// segment the cut lands in and dropping the rest — barLine's left-edge
// truncation, segment-wise.
func truncSegs(segs []seg, width int) []seg {
	var out []seg
	used := 0
	for _, s := range segs {
		n := len([]rune(s.text))
		if used+n <= width {
			out = append(out, s)
			used += n
			continue
		}
		if width-used > 0 {
			out = append(out, seg{s.style, ellipsize(s.text, width-used)})
		}
		break
	}
	return out
}

// legendLine renders the unfilled footer legend (chrome hierarchy,
// 2026-08-03): key tokens bold, labels and the · separators dim, the
// optional right text (the done tally) dim and right-aligned. items are
// {key, label} pairs. Bold keys live here only — the colored section
// bars stay single-style because barLine pads by rune count.
func legendLine(col colors, items [][2]string, right string, width int) string {
	left := []seg{{"", " "}}
	for i, it := range items {
		if i > 0 {
			left = append(left, seg{col.dim, " · "})
		}
		left = append(left, seg{col.bold, it[0]}, seg{col.dim, " " + it[1]})
	}
	var r []seg
	if right != "" {
		r = []seg{{col.dim, right}}
	}
	return segLine(col, left, r, width)
}

// gridRow renders a row's first line on the shared column grid: dim
// id, badge, bold title — plain-ellipsized to the width, no suffix
// fight (two-line rows, 2026-08-05: metadata lives on the meta line
// now, so titles and metadata both win). Titles are bold in every
// section (visual hierarchy, 2026-07-31) — shelf rows keep their dim
// id and badge but the title reads full-strength, an accepted
// consequence. Selection is no longer per-line styling: the
// chunk-level bar (selectedText) carries it.
func gridRow(col colors, idW int, id, badge, badgeStyle, title string, width int) string {
	budget := width - gridTitleCol(idW)
	if budget < 2 {
		budget = 2
	}
	return "  " +
		sgr(col, col.dim, padTo(id, idW)) + "  " +
		sgr(col, badgeStyle, padTo(badge, gridBadgeW)) + "  " +
		sgr(col, col.bold, ellipsize(oneLine(title), budget))
}

// secondLine renders a row's indented second line: an optional colored
// lead ("waiting: ", "question: ") and dim text, ellipsized to the
// width. tcol is the derived title column the line indents to.
func secondLine(col colors, tcol int, lead, leadStyle, text string, width int) string {
	budget := width - tcol - len([]rune(lead))
	if budget < 2 {
		budget = 2
	}
	return strings.Repeat(" ", tcol) +
		sgr(col, leadStyle, lead) + sgr(col, col.dim, ellipsize(oneLine(text), budget))
}

// metaLine renders a row's dim metadata second line — parts joined by
// " · ", indented to the title column, with an optional styled mode
// tail (the in-progress "← holder" in yellow on the otherwise dim
// line). Returns "" when there is nothing to say: skip-when-empty is
// deliberate — a one-line row signals "no labels, no edges" at a
// glance, and list rows are not editors, so no `none` placeholder.
func metaLine(col colors, tcol int, parts []string, tail, tailStyle string, width int) string {
	text := strings.Join(parts, " · ")
	if text == "" && tail == "" {
		return ""
	}
	if text != "" && tail != "" {
		text += " · "
	}
	budget := width - tcol - len([]rune(tail))
	if budget < 2 {
		budget = 2
	}
	return strings.Repeat(" ", tcol) +
		sgr(col, col.dim, ellipsize(oneLine(text), budget)) + sgr(col, tailStyle, tail)
}

// metaParts collects the parts every section's meta line shares, in
// rule order: the [labels] block, then the edge markers. Mode tails
// (holder, close stamp, escalation actor · stamp) are appended by the
// caller — one meta-line rule across every section.
func metaParts(s *snapshot, id string) []string {
	var parts []string
	if ls := s.tasks[id].Task.Labels; len(ls) > 0 {
		parts = append(parts, "["+strings.Join(ls, ", ")+"]")
	}
	if e := edgeText(s, id); e != "" {
		parts = append(parts, e)
	}
	return parts
}

// edgeText marks that a task is part of a structure — scheduling
// (depends_on) — without imposing a tree on the flat list: rows only
// mark that edges exist.
func edgeText(s *snapshot, id string) string {
	if n := len(s.tasks[id].Task.DependsOn); n > 0 {
		return plural(n, "dep")
	}
	return ""
}

// closeText is a history row's close stamp and closing actor — the
// done/cancelled mode tail of the meta line — at day precision, the
// browse granularity; the full instant lives on the ledger. Empty when
// the snapshot carries no close metadata (a pre-upgrade daemon).
func closeText(t stateTask) string {
	if t.ClosedAt == nil {
		return ""
	}
	s := dayStamp(*t.ClosedAt)
	if t.ClosedBy != "" {
		s += " · " + t.ClosedBy
	}
	return s
}

// rowChunk renders one selectable row as an unsplittable chunk; the
// selected chunk is re-rendered as the full-height bar in one place,
// after its section shape is built. Task rows are two-line (grill
// 2026-08-05): full bold title, then a dim meta line — one rule across
// every section, `[labels] · edges · <mode tail>` — rendered only when
// non-empty, so a one-line row signals "no labels, no edges".
func rowChunk(col colors, s *snapshot, r topRow, cursor bool, idW, width int) chunk {
	tcol := gridTitleCol(idW)
	var text string
	if r.kind == rowEscalation {
		// Task-shaped three-liner (grill cycle, 2026-07-31): title line
		// like every other section, the question on its own line, then
		// the meta line with actor · stamp as its mode tail — the
		// metadata sits on line 3 because the question outranks it. The
		// red ! badge alone carries "blocking" — the word is gone.
		e := r.esc
		badge, style := blockingBadge(col, e.Blocking)
		meta := append(metaParts(s, e.Task), e.Actor, stamp(e.RaisedAt))
		text = gridRow(col, idW, event.ShortID(e.Task), badge, style, s.tasks[e.Task].Task.Title, width) +
			"\n" + secondLine(col, tcol, "question: ", col.magenta, e.Question, width) +
			"\n" + metaLine(col, tcol, meta, "", "", width)
	} else {
		t := r.task
		meta := metaParts(s, t.ID)
		badge, badgeStyle, tail, tailStyle := "", "", "", ""
		switch r.section {
		case "ready":
			badge, badgeStyle = fmt.Sprintf("p%d", t.Priority), col.dim
			if t.Priority == 0 {
				badgeStyle = col.yellow
			}
		case "inprogress":
			// The holder is the mode tail: yellow on the otherwise dim
			// meta line.
			tail, tailStyle = "← "+t.Holder, col.yellow
		case "held":
			// Priority is stored but inert while held (it bites again at
			// resume), so the badge renders — dim, like the rest of the row.
			badge, badgeStyle = fmt.Sprintf("p%d", t.Priority), col.dim
		case "done", "cancelled":
			// History rows (history view, 2026-08-02): the close stamp
			// and closing actor are the mode tail, dim with the rest of
			// the meta line. No priority badge: recency is the browse
			// axis here, not priority.
			if c := closeText(t); c != "" {
				meta = append(meta, c)
			}
		}
		// inbox and blocked carry no badge: an untriaged capture has no
		// meaningful priority, and a blocked row's story is its waiting:
		// line.
		text = gridRow(col, idW, event.ShortID(t.ID), badge, badgeStyle, t.Title, width)
		if ml := metaLine(col, tcol, meta, tail, tailStyle, width); ml != "" {
			text += "\n" + ml
		}
		if r.section == "blocked" {
			// The waiting: line stays its own line below the meta line —
			// a reason, not metadata. Its lead is dim red (bar recolors,
			// 2026-08-04): full-brightness red would be louder than the
			// section's own dim-red bar.
			text += "\n" + secondLine(col, tcol, "waiting: ", col.dimRed, blockedReasonTUI(t, s.taskRef), width)
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
	idW := idColWidth(s)
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
			c := rowChunk(col, s, m.rows[i], i == m.cursor, idW, width)
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
		label := fmt.Sprintf("priority %s (%s)", event.ShortID(m.target.task.ID), oneLine(m.target.task.Title))
		return m.input.view(col, label, "submits", m.width)
	case modeConfirmCancel:
		return wrapTo(fmt.Sprintf("%scancel%s %s (%s)? y/n %s— history stays on the ledger%s\n",
			col.bold, col.reset, event.ShortID(m.target.task.ID), oneLine(m.target.task.Title),
			col.dim, col.reset), m.width)
	case modeCapture:
		// No y/n: capture is cheap by design, and cancel reverses it.
		return m.input.view(col, "capture (to inbox)", "captures", m.width)
	case modeEditTitle:
		// The box carries the title being edited; the label only needs
		// to name whose it is.
		return m.input.view(col, "title "+event.ShortID(m.target.task.ID), "saves", m.width)
	case modeEditDesc:
		label := fmt.Sprintf("description %s (%s)", event.ShortID(m.target.task.ID), oneLine(m.target.task.Title))
		return m.input.view(col, label, "saves", m.width)
	case modeEditLabels:
		label := fmt.Sprintf("labels %s (%s)", event.ShortID(m.target.task.ID), oneLine(m.target.task.Title))
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
		return legendLine(col, [][2]string{
			{"↑/↓ (j/k)", "move"}, {"enter", "open"}, {"esc", "back"}, {"q", "quit"},
		}, "", width) + "\n"
	}
	// "enter open" on every row: a Needs Input row opens its task's view
	// with the question preselected — answering happens there, with the
	// task's context on screen (task-view rework, 2026-08-01).
	legend := [][2]string{{"↑/↓ (j/k)", "move"}, {"enter", "open"}}
	if m.armed {
		legend = append(legend, [2]string{"p", "priority"}, [2]string{"c", "cancel"})
	}
	legend = append(legend, [2]string{"h", "history"}, [2]string{"q", "quit"})
	done := ""
	if m.snap != nil {
		done = fmt.Sprintf("%d done ", len(m.snap.classify().done))
	}
	return legendLine(col, legend, done, width) + "\n"
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
