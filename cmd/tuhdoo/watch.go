package main

// tuhdoo watch: the live read-only dashboard (T7) — mechanically the v1
// TUI's skeleton with interactivity amputated. A Bubble Tea view loop
// with ZERO input handling except quit; state arrives by polling the
// daemon on a tick.

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const watchRefresh = 2 * time.Second

type tickMsg time.Time

// snapMsg carries one poll result; err is shown and retried, never fatal.
type snapMsg struct {
	snap *snapshot
	err  error
}

// watchModel renders from its last snapshot; the client is only used by
// commands, so a model built directly from a snapshot is testable
// without a TTY or a daemon.
type watchModel struct {
	c    *client
	col  colors
	snap *snapshot
	err  error
}

func (m watchModel) Init() tea.Cmd {
	return tea.Batch(fetchCmd(m.c), tickCmd())
}

func tickCmd() tea.Cmd {
	return tea.Tick(watchRefresh, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func fetchCmd(c *client) tea.Cmd {
	return func() tea.Msg {
		s, err := fetchSnapshot(c)
		return snapMsg{snap: s, err: err}
	}
}

func (m watchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case tickMsg:
		return m, tea.Batch(fetchCmd(m.c), tickCmd())
	case snapMsg:
		m.err = msg.err
		if msg.snap != nil {
			m.snap = msg.snap
		}
	}
	return m, nil
}

func (m watchModel) View() string {
	col := m.col
	var w strings.Builder
	sync := "..."
	if m.snap != nil {
		sync = syncLine(m.snap.state.Sync)
	}
	fmt.Fprintf(&w, "%stuhdoo watch%s · sync: %s · refresh %s · q quits\n\n",
		col.bold, col.reset, sync, watchRefresh)
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
	renderReady(&w, col, b.ready)
	w.WriteString("\n")
	renderInProgress(&w, col, b.inProgress)
	w.WriteString("\n")
	renderOpenEscalations(&w, col, s, s.state.OpenEscalations)
	return w.String()
}

func runWatch() int {
	_, c, code := connect()
	if code != 0 {
		return code
	}
	m := watchModel{c: c, col: newColors(os.Stdout)}
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "tuhdoo watch:", err)
		return 1
	}
	return 0
}
