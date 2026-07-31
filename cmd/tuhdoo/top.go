package main

// The interactive TUI (002 T7, revised by Cycle 4): the single live
// human surface. Reads poll the daemon on a tick; the three steering
// writes (answer an escalation, reprioritize, archive) go through the
// daemon HTTP API only, stamped with the acting human principal.
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
	archiveTask(task string) error
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

// archiveTask is the human archive verb over the plumbing status
// vocabulary (T7, 2026-07-31): the task model has no separate archived
// state (D5) — "cancelled" is the terminal curation status the ledger
// records, and "archive" is what humans read and type for it.
func (s httpSteering) archiveTask(task string) error {
	return s.c.write("PATCH", "/v0/tasks/"+task, s.actor, map[string]any{"status": "cancelled"})
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
// order: open escalations, then ready, in-progress, and blocked tasks.
// Done and cancelled tasks are not steerable and get no rows.
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
		rows = append(rows, topRow{kind: rowTask, section: "blocked", task: t})
	}
	return rows
}

// ---- the model ----

// Input modes. Nav is the resting state; the others capture keys until
// enter/esc (or y/n for the archive confirmation). Detail is the
// in-place task screen: armed it steers the viewed task (enter answers
// the focused open escalation, p/c reprioritize/archive), disarmed it
// is read-only; esc steps back to the list either way.
const (
	modeNav = iota
	modeAnswer
	modePriority
	modeConfirmArchive
	modeDetail
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

	snap   *snapshot
	err    error
	rows   []topRow
	cursor int

	mode   int
	back   int // mode an input mode returns to on esc/submit: the list or the detail it was opened from
	input  string
	target topRow // row a pending answer/priority/archive applies to
	status string // one-line result of the last action

	detailID     string // task shown by modeDetail
	detailScroll int    // first visible body line in modeDetail
	detailFocus  int    // focused open escalation in an armed detail (index into detailOpenEscalations)
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
			// that vanished (answered, archived, claimed…) drops the
			// cursor to the top.
			var sel string
			if m.cursor < len(m.rows) {
				sel = m.rows[m.cursor].id()
			}
			m.snap = msg.snap
			m.rows = buildRows(msg.snap)
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
	case "p":
		if r, ok := m.selected(); m.armed && ok && r.kind == rowTask {
			m.mode, m.back, m.target, m.input, m.status = modePriority, modeNav, r, "", ""
		}
	case "c":
		if r, ok := m.selected(); m.armed && ok && r.kind == rowTask {
			m.mode, m.back, m.target, m.input, m.status = modeConfirmArchive, modeNav, r, "", ""
		}
	}
	return m, nil
}

// openRow is what enter — and a click on the already-selected row —
// does. It acts on what the row is for: a Needs Input row goes straight
// into answering (dogfood steering, 2026-07-30 — the old `a` key is
// gone, one documented behavior per action); task rows open their
// biography. A disarmed pane never opens input: watch mode falls
// through to the read-only detail of the escalation's task.
func (m topModel) openRow(r topRow) (tea.Model, tea.Cmd) {
	if m.armed && r.kind == rowEscalation {
		m.mode, m.back, m.target, m.input, m.status = modeAnswer, modeNav, r, "", ""
		return m, nil
	}
	id := r.task.ID
	if r.kind == rowEscalation {
		id = r.esc.Task
	}
	m.mode, m.detailID, m.detailScroll, m.detailFocus, m.status = modeDetail, id, 0, 0, ""
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
		}
	}
	return m, nil
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

func (m topModel) updateInput(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.mode == modeConfirmArchive {
		switch k.String() {
		case "y":
			return m.submit()
		case "n", "esc":
			m.mode, m.input = m.back, ""
		case "ctrl+c":
			return m, tea.Quit
		}
		return m, nil
	}
	switch k.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.mode, m.input = m.back, ""
	case "enter":
		return m.submit()
	case "backspace":
		if r := []rune(m.input); len(r) > 0 {
			m.input = string(r[:len(r)-1])
		}
	case " ":
		m.input += " "
	default:
		if k.Type == tea.KeyRunes {
			m.input += string(k.Runes)
		}
	}
	return m, nil
}

// updateDetail: the armed detail screen steers the viewed task in place
// (dogfood steering, 2026-07-30 — no more esc → navigate → answer round
// trips): enter answers the focused open escalation, p reprioritizes and
// c archives the viewed task, all with the same footers and confirms as
// the list. Watch mode keeps the read-only contract: no focus, no input,
// ↑/↓ and j/k scroll, esc steps back, q and ctrl+c quit.
//
// Focus vs scroll — the deliberate rule: j/k move focus when a further
// open escalation exists in that direction (the window scrolls just
// enough to reveal it) and scroll one line otherwise. With at most one
// open escalation — the overwhelmingly common case — that fallback IS
// the behavior: plain line scrolling, exactly as before, with the single
// escalation permanently focused. Only a multi-escalation task trades
// some upward line-scrolling for focus jumps.
func (m topModel) updateDetail(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	open := m.detailOpenEscalations()
	focus := detailFocusIdx(m.detailFocus, len(open))
	switch k.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.mode, m.detailID, m.detailScroll, m.detailFocus = modeNav, "", 0, 0
	case "j", "down":
		if focus >= 0 && focus < len(open)-1 {
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
		if focus >= 0 { // armed with an open escalation; watch has none by construction
			m.detailFocus = focus
			m.mode, m.back = modeAnswer, modeDetail
			m.target, m.input, m.status = topRow{kind: rowEscalation, esc: open[focus]}, "", ""
		}
	case "p":
		if t, ok := m.viewedTask(); m.armed && ok {
			m.mode, m.back = modePriority, modeDetail
			m.target, m.input, m.status = topRow{kind: rowTask, task: t}, "", ""
		}
	case "c":
		if t, ok := m.viewedTask(); m.armed && ok {
			m.mode, m.back = modeConfirmArchive, modeDetail
			m.target, m.input, m.status = topRow{kind: rowTask, task: t}, "", ""
		}
	}
	return m, nil
}

// detailOpenEscalations lists the viewed task's unanswered escalations
// in ULID order — the same order historyOf renders them — as the
// focusable items of an armed detail. Watch mode focuses nothing: the
// disarmed pane stays fully read-only.
func (m topModel) detailOpenEscalations() []escalationJSON {
	if !m.armed || m.snap == nil {
		return nil
	}
	var open []escalationJSON
	for _, e := range m.snap.tasks[m.detailID].Escalations {
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
	api, target, input := m.api, m.target, strings.TrimSpace(m.input)
	switch m.mode {
	case modeAnswer:
		if input == "" {
			m.status = "answer cannot be empty"
			return m, nil
		}
		m.mode, m.input, m.status = m.back, "", "answering…"
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
		m.mode, m.input, m.status = m.back, "", "updating…"
		return m, func() tea.Msg {
			if err := api.setPriority(target.task.ID, p); err != nil {
				return actionMsg{err: err}
			}
			return actionMsg{desc: fmt.Sprintf("set %s to p%d", shortID(target.task.ID), p)}
		}
	case modeConfirmArchive:
		m.mode, m.input, m.status = m.back, "", "archiving…"
		return m, func() tea.Msg {
			if err := api.archiveTask(target.task.ID); err != nil {
				return actionMsg{err: err}
			}
			return actionMsg{desc: "archived " + shortID(target.task.ID)}
		}
	}
	return m, nil
}

// ---- rendering (pure over model state) ----

// badge names the mode in the header: who steering acts as, or a
// visible marker that this pane cannot act at all.
func (m topModel) badge() string {
	col := m.col
	if !m.armed {
		return fmt.Sprintf("%s%swatch mode%s", col.bold, col.yellow, col.reset)
	}
	return fmt.Sprintf("acting as %s%s%s", col.bold, m.actor, col.reset)
}

// detailBody renders the full task biography (the same layout as
// `tuhdoo task <id>`, with references shortened and annotated for the
// screen) as lines, or nil if the task vanished.
func (m topModel) detailBody() []string {
	if m.snap == nil {
		return nil
	}
	h, ok := m.snap.tasks[m.detailID]
	if !ok {
		return nil
	}
	var b strings.Builder
	printTaskRef(&b, m.col, h, m.snap.taskRef)
	body := b.String()
	if open := m.detailOpenEscalations(); len(open) > 0 {
		body = markFocusedEscalation(body, m.col, detailFocusIdx(m.detailFocus, len(open)))
	}
	// Wrap before splitting so the scroll window counts screen lines,
	// not logical ones.
	return strings.Split(strings.TrimRight(wrapTo(body, m.width), "\n"), "\n")
}

// markFocusedEscalation rewrites the focus-th "unanswered" line of a
// rendered biography into the focused-item marker. The k-th unanswered
// line belongs to the k-th open escalation because historyOf renders
// entries in ULID order and detailOpenEscalations sorts the same way.
// Pure string-in string-out; the one-shot rendering never passes
// through here and stays byte-identical.
func markFocusedEscalation(body string, col colors, focus int) string {
	plain := "    " + sgr(col, col.dim, "unanswered")
	lines := strings.Split(body, "\n")
	k := 0
	for i, l := range lines {
		if l != plain {
			continue
		}
		if k == focus {
			lines[i] = "  ▸ " + sgr(col, col.bold, "unanswered — enter to answer")
		}
		k++
	}
	return strings.Join(lines, "\n")
}

// detailFocusLine is the wrapped-body line index of the focus marker,
// or -1 when nothing is focused.
func (m topModel) detailFocusLine() int {
	for i, l := range m.detailBody() {
		if strings.HasPrefix(l, "  ▸") {
			return i
		}
	}
	return -1
}

// detailRevealScroll scrolls just enough — never more — to bring the
// focused escalation's marker line into the window.
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
// header, footer, and optional status lines detailView prints around
// the body.
func (m topModel) detailWindow() int {
	w := m.height - 4
	if m.status != "" {
		w--
	}
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
	col := m.col
	var w strings.Builder
	w.WriteString(wrapTo(fmt.Sprintf("%stuhdoo%s · sync: %s · %s\n", col.bold, col.reset,
		syncLine(m.snap.state.Sync), m.badge()), m.width))
	if m.status != "" {
		w.WriteString(m.status + "\n")
	}
	w.WriteString("\n")
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

// detailFooter is the detail screen's bottom line: the live input
// prompt while one is open (so answering reads the same from either
// screen), else the key legend for what this pane can do — the armed
// legend advertises steering, and only mentions enter when there is an
// open escalation to answer; watch mode keeps the read-only legend.
func (m topModel) detailFooter() string {
	if f := m.inputFooter(); f != "" {
		return f
	}
	col := m.col
	legend := "↑/↓ (j/k) scroll · esc back · q quit"
	if m.armed {
		legend = "↑/↓ (j/k) scroll · p priority · c archive · esc back · q quit"
		if len(m.detailOpenEscalations()) > 0 {
			legend = "↑/↓ (j/k) move · enter answer · p priority · c archive · esc back · q quit"
		}
	}
	return wrapTo(fmt.Sprintf("%s%s%s\n", col.dim, legend, col.reset), m.width)
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
// its bar color, and the steering keys the bar advertises when the pane
// is armed. Sections are data so the growing status list (inbox,
// on-hold are being designed) adds entries here, not rendering code.
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
	{"ready", "READY", func(c colors) string { return c.bgGreen }, "p priority · c archive"},
	{"inprogress", "IN PROGRESS", func(c colors) string { return c.bgYellow }, ""},
	{"blocked", "BLOCKED", func(c colors) string { return c.bgRed }, ""},
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
// sacrificed for the title when the line is tight.
func gridRow(col colors, cursor bool, id, badge, badgeStyle, title, suffix, suffixStyle string, width int) string {
	mark, markStyle, titleStyle := "  ", "", ""
	if cursor {
		mark, markStyle, titleStyle = "▸ ", col.bold, col.bold
	}
	title, suffix = fitTitle(oneLine(title), suffix, width-gridTitleCol)
	return sgr(col, markStyle, mark) +
		sgr(col, col.dim, padTo(id, gridIDW)) + "  " +
		sgr(col, badgeStyle, padTo(badge, gridBadgeW)) + "  " +
		sgr(col, titleStyle, title) + sgr(col, suffixStyle, suffix)
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

// rowChunk renders one selectable row as an unsplittable chunk.
func rowChunk(col colors, s *snapshot, r topRow, cursor bool, width int) chunk {
	if r.kind == rowEscalation {
		e := r.esc
		badge, style := "", ""
		lead, leadStyle := "", ""
		if e.Blocking {
			badge, style = "!", col.red+col.bold
			lead, leadStyle = "blocking", col.red
		}
		meta := fmt.Sprintf(" · %s · %s", e.Actor, stamp(e.RaisedAt))
		if !e.Blocking {
			meta = fmt.Sprintf("%s · %s", e.Actor, stamp(e.RaisedAt))
		}
		return chunk{
			text: gridRow(col, cursor, shortID(e.Task), badge, style, e.Question, "", "", width) +
				"\n" + secondLine(col, lead, leadStyle, meta, width),
			cursor: cursor,
		}
	}
	t := r.task
	suffix := labelSuffix(t.Labels) + edgeText(s, t.ID)
	switch r.section {
	case "ready":
		badgeStyle := col.dim
		if t.Priority == 0 {
			badgeStyle = col.yellow
		}
		return chunk{text: gridRow(col, cursor, shortID(t.ID), fmt.Sprintf("p%d", t.Priority),
			badgeStyle, t.Title, suffix, col.dim, width), cursor: cursor}
	case "inprogress":
		return chunk{text: gridRow(col, cursor, shortID(t.ID), "", "",
			t.Title, "  ← "+t.Holder, col.yellow, width), cursor: cursor}
	default: // blocked
		return chunk{
			text: gridRow(col, cursor, shortID(t.ID), "", "", t.Title, suffix, col.dim, width) +
				"\n" + secondLine(col, "waiting: ", col.red, s.blockedReasonTUI(t.ID, s.taskRef), width),
			cursor: cursor,
		}
	}
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
	for si, sec := range topSections {
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
// and the detail screen keeps the full ULID once, on its canonical
// `id` line.
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
// reads identically from either screen.
func (m topModel) inputFooter() string {
	col := m.col
	switch m.mode {
	case modeAnswer:
		return wrapTo(fmt.Sprintf("%sanswer%s %s > %s█  %senter submits · esc cancels%s\n",
			col.bold, col.reset, oneLine(m.target.esc.Question), m.input, col.dim, col.reset), m.width)
	case modePriority:
		return wrapTo(fmt.Sprintf("%spriority%s %s (%s) > %s█  %senter submits · esc cancels%s\n",
			col.bold, col.reset, shortID(m.target.task.ID), oneLine(m.target.task.Title),
			m.input, col.dim, col.reset), m.width)
	case modeConfirmArchive:
		return wrapTo(fmt.Sprintf("%sarchive%s %s (%s)? y/n %s— history stays on the ledger%s\n",
			col.bold, col.reset, shortID(m.target.task.ID), oneLine(m.target.task.Title),
			col.dim, col.reset), m.width)
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
	// "enter answer/open" because enter acts on what the row is for:
	// answering on a Needs Input row, the biography elsewhere. Folding
	// the old `a` key into enter also bought back the tally's trailing
	// margin space at 80 columns.
	legend := " ↑/↓ (j/k) move · enter open · q quit"
	if m.armed {
		legend = " ↑/↓ (j/k) move · enter answer/open · p priority · c archive · q quit"
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
