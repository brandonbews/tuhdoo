package main

// The interactive TUI (002 T7, revised by Cycle 4): the single live
// human surface. Reads poll the daemon on a tick; the three steering
// writes (answer an escalation, reprioritize, cancel) go through the
// daemon HTTP API only, stamped with the acting human principal.
// Watch mode is the same screen disarmed: steering keys dead, fixed at
// launch — no keypress can re-arm a disarmed pane.

import (
	"flag"
	"fmt"
	"io"
	"os"
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
// enter/esc (or y/n for the cancel confirmation). Detail is the
// in-place task screen: read-only, esc steps back to the list.
const (
	modeNav = iota
	modeAnswer
	modePriority
	modeConfirmCancel
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
	input  string
	target topRow // row a pending answer/priority/cancel applies to
	status string // one-line result of the last action

	detailID     string // task shown by modeDetail
	detailScroll int    // first visible body line in modeDetail
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
			id := r.task.ID
			if r.kind == rowEscalation {
				id = r.esc.Task // an escalation opens its task's biography
			}
			m.mode, m.detailID, m.detailScroll, m.status = modeDetail, id, 0, ""
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

// updateDetail: the detail screen is read-only — ↑/↓ and j/k scroll, esc steps
// back to the list, q and ctrl+c quit (one meaning per key, T7).
func (m topModel) updateDetail(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.mode, m.detailID, m.detailScroll = modeNav, "", 0
	case "j", "down":
		if m.detailScroll < m.detailMaxScroll() {
			m.detailScroll++
		}
	case "k", "up":
		if m.detailScroll > 0 {
			m.detailScroll--
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
			return actionMsg{desc: fmt.Sprintf("set %s to p%d", shortID(target.task.ID), p)}
		}
	case modeConfirmCancel:
		m.mode, m.input, m.status = modeNav, "", "cancelling…"
		return m, func() tea.Msg {
			if err := api.cancelTask(target.task.ID); err != nil {
				return actionMsg{err: err}
			}
			return actionMsg{desc: "cancelled " + shortID(target.task.ID)}
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
	// Wrap before splitting so the scroll window counts screen lines,
	// not logical ones.
	return strings.Split(strings.TrimRight(wrapTo(b.String(), m.width), "\n"), "\n")
}

// detailWindow is how many body lines fit: terminal height minus the
// header and footer lines detailView prints around the body.
func (m topModel) detailWindow() int {
	w := m.height - 4
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
	w.WriteString(wrapTo(fmt.Sprintf("%s↑/↓ (j/k) scroll · esc back · q quit%s\n", col.dim, col.reset), m.width))
	return w.String()
}

func (m topModel) View() string {
	if m.mode == modeDetail {
		if body := m.detailBody(); body != nil {
			return m.detailView(body)
		}
		// The task vanished under a refresh: fall through to the list.
	}
	col := m.col
	var head strings.Builder
	sync := "..."
	if m.snap != nil {
		sync = syncLine(m.snap.state.Sync)
	}
	fmt.Fprintf(&head, "%stuhdoo%s · sync: %s · %s\n", col.bold, col.reset, sync, m.badge())
	if m.status != "" {
		fmt.Fprintf(&head, "%s\n", m.status)
	}
	head.WriteString("\n")
	hs := wrapTo(head.String(), m.width)
	if m.err != nil {
		return hs + wrapTo(fmt.Sprintf("%sdaemon unreachable:%s %v %s(retrying)%s\n",
			col.red, col.reset, m.err, col.dim, col.reset), m.width)
	}
	if m.snap == nil {
		return hs + "loading...\n"
	}
	s := m.snap
	var body strings.Builder
	if s.state.Degraded != "" {
		fmt.Fprintf(&body, "%sDEGRADED (read-only):%s %s\n\n", col.red, col.reset, s.state.Degraded)
	}
	b := s.classify()
	fmt.Fprintf(&body, "%s%d ready%s · %s%d in progress%s · %s%d blocked%s · %d done · %d cancelled · %s open\n\n",
		col.green, len(b.ready), col.reset,
		col.yellow, len(b.inProgress), col.reset,
		col.red, len(b.blocked), col.reset,
		len(b.done), len(b.cancelled),
		plural(len(s.state.OpenEscalations), "escalation"))
	renderTopRows(&body, col, s, m.rows, m.cursor)
	foot := "\n" + wrapTo(m.footer(), m.width)
	list := m.windowList(wrapTo(body.String(), m.width),
		strings.Count(hs, "\n"), strings.Count(foot, "\n"))
	return hs + list + foot
}

// windowList slides a window over the wrapped list body so the cursor
// row is always on screen (pinned to the bottom edge once the list
// outgrows the terminal). Unknown height renders everything.
func (m topModel) windowList(body string, headLines, footLines int) string {
	body = strings.TrimRight(body, "\n")
	if m.height <= 0 {
		return body + "\n"
	}
	lines := strings.Split(body, "\n")
	avail := m.height - headLines - footLines
	if avail < 1 {
		avail = 1
	}
	if len(lines) <= avail {
		return body + "\n"
	}
	scroll := 0
	for i, l := range lines {
		if strings.Contains(l, "▸ ") {
			if i >= avail {
				scroll = i - avail + 1
			}
			break
		}
	}
	if scroll+avail > len(lines) {
		scroll = len(lines) - avail
	}
	return strings.Join(lines[scroll:scroll+avail], "\n") + "\n"
}

// renderTopRows renders the selectable sections with the cursor marker.
func renderTopRows(w io.Writer, col colors, s *snapshot, rows []topRow, cursor int) {
	secs := []struct {
		key, title, color string
	}{
		// "Needs Input", not "Open escalations" (T7, 2026-07-30): the
		// entity keeps its name; the header alone softens the severity
		// the word overstates, and names no answerer — a future one may
		// not be a human.
		{"escalations", "Needs Input", ""},
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
		for n, i := range idx {
			if n > 0 {
				fmt.Fprintln(w) // breathing room: rows wrap to multiple lines
			}
			renderTopRow(w, col, s, rows[i], i == cursor)
		}
	}
}

// shortID abbreviates a task ID for TUI display: the type prefix plus
// the ULID's last four characters, lowercased (`t-d83w`). The tail is
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

// edgeSuffix surfaces that a task is part of a structure — containment
// (parents) and scheduling (depends_on) — without imposing a tree on
// the flat, status-grouped list: edge semantics are still an open
// question, so the list only marks that edges exist.
func edgeSuffix(col colors, s *snapshot, id string) string {
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
	return fmt.Sprintf("  %s· %s%s", col.dim, strings.Join(parts, " · "), col.reset)
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
			shortID(e.Task), title, e.Actor, stamp(e.RaisedAt))
		return
	}
	t := r.task
	edges := edgeSuffix(col, s, t.ID)
	switch r.section {
	case "ready":
		fmt.Fprintf(w, "%s%s%s%s  p%d  %s%s%s\n",
			mark, col.dim, shortID(t.ID), col.reset, t.Priority, oneLine(t.Title), labelSuffix(t.Labels), edges)
	case "inprogress":
		fmt.Fprintf(w, "%s%s%s%s  %s  %s← %s%s%s\n",
			mark, col.dim, shortID(t.ID), col.reset, oneLine(t.Title), col.yellow, t.Holder, col.reset, edges)
	default: // blocked
		fmt.Fprintf(w, "%s%s%s%s  %s%s\n      %swaiting:%s %s\n",
			mark, col.dim, shortID(t.ID), col.reset, oneLine(t.Title), edges, col.red, col.reset,
			s.blockedReasonDisp(t.ID, s.taskRef))
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
			col.bold, col.reset, shortID(m.target.task.ID), oneLine(m.target.task.Title),
			m.input, col.dim, col.reset)
	case modeConfirmCancel:
		return fmt.Sprintf("%scancel%s %s (%s)? y/n\n",
			col.bold, col.reset, shortID(m.target.task.ID), oneLine(m.target.task.Title))
	}
	if !m.armed {
		return fmt.Sprintf("%s↑/↓ (j/k) move · enter open · q quit%s\n", col.dim, col.reset)
	}
	return fmt.Sprintf("%s↑/↓ (j/k) move · enter open · a answer · p priority · c cancel · q quit%s\n", col.dim, col.reset)
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
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "tuhdoo:", err)
		return 1
	}
	return 0
}
