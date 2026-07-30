package main

// The interactive TUI (002 T7, revised by Cycle 4): the single live
// human surface. Reads poll the daemon on a tick; the three steering
// writes (answer an escalation, reprioritize, cancel) go through the
// daemon HTTP API only, stamped with the acting human principal.
// Watch mode is the same screen disarmed: steering keys dead, fixed at
// launch — no keypress can re-arm a disarmed pane.

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/brandonbews/tuhdoo/internal/daemon"
)

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

// cancelTask is "cancel/archive": the task model has no separate
// archived state (D5), so cancelled is the terminal curation status.
func (s httpSteering) cancelTask(task string) error {
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
// enter/esc (or y/n for the cancel confirmation).
const (
	modeNav = iota
	modeAnswer
	modePriority
	modeConfirmCancel
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
	input  string
	target topRow // row a pending answer/priority/cancel applies to
	status string // one-line result of the last action
}

func (m topModel) Init() tea.Cmd {
	return tea.Batch(fetchCmd(m.c), tickCmd())
}

func (m topModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.mode == modeNav {
			return m.updateNav(msg)
		}
		return m.updateInput(msg)
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
	case "a":
		if r, ok := m.selected(); m.armed && ok && r.kind == rowEscalation {
			m.mode, m.target, m.input, m.status = modeAnswer, r, "", ""
		}
	case "p":
		if r, ok := m.selected(); m.armed && ok && r.kind == rowTask {
			m.mode, m.target, m.input, m.status = modePriority, r, "", ""
		}
	case "c":
		if r, ok := m.selected(); m.armed && ok && r.kind == rowTask {
			m.mode, m.target, m.input, m.status = modeConfirmCancel, r, "", ""
		}
	}
	return m, nil
}

func (m topModel) updateInput(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.mode == modeConfirmCancel {
		switch k.String() {
		case "y":
			return m.submit()
		case "n", "esc":
			m.mode, m.input = modeNav, ""
		case "ctrl+c":
			return m, tea.Quit
		}
		return m, nil
	}
	switch k.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.mode, m.input = modeNav, ""
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
		m.mode, m.input, m.status = modeNav, "", "answering…"
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
		m.mode, m.input, m.status = modeNav, "", "updating…"
		return m, func() tea.Msg {
			if err := api.setPriority(target.task.ID, p); err != nil {
				return actionMsg{err: err}
			}
			return actionMsg{desc: fmt.Sprintf("set %s to p%d", target.task.ID, p)}
		}
	case modeConfirmCancel:
		m.mode, m.input, m.status = modeNav, "", "cancelling…"
		return m, func() tea.Msg {
			if err := api.cancelTask(target.task.ID); err != nil {
				return actionMsg{err: err}
			}
			return actionMsg{desc: "cancelled " + target.task.ID}
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

func (m topModel) View() string {
	col := m.col
	var w strings.Builder
	sync := "..."
	if m.snap != nil {
		sync = syncLine(m.snap.state.Sync)
	}
	fmt.Fprintf(&w, "%stuhdoo%s · sync: %s · %s\n", col.bold, col.reset, sync, m.badge())
	if m.status != "" {
		fmt.Fprintf(&w, "%s\n", m.status)
	}
	w.WriteString("\n")
	if m.err != nil {
		fmt.Fprintf(&w, "%sdaemon unreachable:%s %v %s(retrying)%s\n",
			col.red, col.reset, m.err, col.dim, col.reset)
		return w.String()
	}
	if m.snap == nil {
		w.WriteString("loading...\n")
		return w.String()
	}
	s := m.snap
	if s.state.Degraded != "" {
		fmt.Fprintf(&w, "%sDEGRADED (read-only):%s %s\n\n", col.red, col.reset, s.state.Degraded)
	}
	b := s.classify()
	fmt.Fprintf(&w, "%s%d ready%s · %s%d in progress%s · %s%d blocked%s · %d done · %d cancelled · %s open\n\n",
		col.green, len(b.ready), col.reset,
		col.yellow, len(b.inProgress), col.reset,
		col.red, len(b.blocked), col.reset,
		len(b.done), len(b.cancelled),
		plural(len(s.state.OpenEscalations), "escalation"))
	renderTopRows(&w, col, s, m.rows, m.cursor)
	w.WriteString("\n")
	w.WriteString(m.footer())
	return w.String()
}

// renderTopRows renders the selectable sections with the cursor marker.
func renderTopRows(w io.Writer, col colors, s *snapshot, rows []topRow, cursor int) {
	secs := []struct {
		key, title, color string
	}{
		{"escalations", "Open escalations", ""},
		{"ready", "Ready", col.green},
		{"inprogress", "In progress", col.yellow},
		{"blocked", "Blocked", col.red},
	}
	for si, sec := range secs {
		if si > 0 {
			fmt.Fprintln(w)
		}
		var idx []int
		for i, r := range rows {
			if r.section == sec.key {
				idx = append(idx, i)
			}
		}
		fmt.Fprintf(w, "%s%s%s%s (%d)\n", col.bold, sec.color, sec.title, col.reset, len(idx))
		if len(idx) == 0 {
			fmt.Fprintf(w, "  %snone%s\n", col.dim, col.reset)
			continue
		}
		for _, i := range idx {
			renderTopRow(w, col, s, rows[i], i == cursor)
		}
	}
}

func renderTopRow(w io.Writer, col colors, s *snapshot, r topRow, selected bool) {
	mark := "  "
	if selected {
		mark = "▸ "
	}
	if r.kind == rowEscalation {
		e := r.esc
		blocking := ""
		if e.Blocking {
			blocking = fmt.Sprintf("  %s[blocking]%s", col.red, col.reset)
		}
		title := ""
		if h, ok := s.tasks[e.Task]; ok {
			title = " (" + oneLine(h.Task.Title) + ")"
		}
		fmt.Fprintf(w, "%s%s%s%s%s\n", mark, col.bold, oneLine(e.Question), col.reset, blocking)
		fmt.Fprintf(w, "      task %s%s · asked by %s · raised %s\n",
			e.Task, title, e.Actor, stamp(e.RaisedAt))
		return
	}
	t := r.task
	switch r.section {
	case "ready":
		fmt.Fprintf(w, "%s%s%s%s  p%d  %s%s\n",
			mark, col.dim, t.ID, col.reset, t.Priority, oneLine(t.Title), labelSuffix(t.Labels))
	case "inprogress":
		fmt.Fprintf(w, "%s%s%s%s  %s  %s← %s%s\n",
			mark, col.dim, t.ID, col.reset, oneLine(t.Title), col.yellow, t.Holder, col.reset)
	default: // blocked
		fmt.Fprintf(w, "%s%s%s%s  %s\n      %swaiting:%s %s\n",
			mark, col.dim, t.ID, col.reset, oneLine(t.Title), col.red, col.reset, s.blockedReason(t.ID))
	}
}

// footer is the key legend, or the active input prompt.
func (m topModel) footer() string {
	col := m.col
	switch m.mode {
	case modeAnswer:
		return fmt.Sprintf("%sanswer%s %s > %s█  %senter submits · esc cancels%s\n",
			col.bold, col.reset, oneLine(m.target.esc.Question), m.input, col.dim, col.reset)
	case modePriority:
		return fmt.Sprintf("%spriority%s %s (%s) > %s█  %senter submits · esc cancels%s\n",
			col.bold, col.reset, m.target.task.ID, oneLine(m.target.task.Title),
			m.input, col.dim, col.reset)
	case modeConfirmCancel:
		return fmt.Sprintf("%scancel%s %s (%s)? y/n\n",
			col.bold, col.reset, m.target.task.ID, oneLine(m.target.task.Title))
	}
	if !m.armed {
		return fmt.Sprintf("%sj/k move · q quit%s\n", col.dim, col.reset)
	}
	return fmt.Sprintf("%sj/k move · a answer · p priority · c cancel · q quit%s\n", col.dim, col.reset)
}

// ---- entry point ----

// topActor resolves the acting human principal: --as wins; otherwise
// it derives from git identity per D7 (the local part of user.email).
// top steers as a root human, never as an agent.
func topActor(args []string) (string, error) {
	as := ""
	switch {
	case len(args) == 0:
	case len(args) == 2 && args[0] == "--as":
		as = args[1]
	case len(args) == 1 && strings.HasPrefix(args[0], "--as="):
		as = strings.TrimPrefix(args[0], "--as=")
	default:
		return "", errors.New("usage: tuhdoo top [--as <human>]")
	}
	if as == "" {
		local, err := gitEmailLocalPart("")
		if err != nil {
			return "", fmt.Errorf("cannot derive your principal from git identity: %v; run: tuhdoo top --as <human>", err)
		}
		as = local
	}
	if err := daemon.ValidateActor(as); err != nil {
		return "", err
	}
	if strings.Contains(as, "/") {
		return "", fmt.Errorf("top steers as a human root principal, not an agent: want e.g. %q, got %q",
			strings.SplitN(as, "/", 2)[0], as)
	}
	return as, nil
}

func runTop(args []string) int {
	actor, err := topActor(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tuhdoo:", err)
		return 1
	}
	_, c, code := connect()
	if code != 0 {
		return code
	}
	m := topModel{c: c, api: httpSteering{c: c, actor: actor}, actor: actor, armed: true, col: newColors(os.Stdout)}
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "tuhdoo:", err)
		return 1
	}
	return 0
}
