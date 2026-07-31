package main

// Terminal rendering shared by the read commands and the TUI.

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// colors holds ANSI escape codes, or all-empty strings for plain
// output. Color is on only when out is a terminal and NO_COLOR is unset
// (https://no-color.org). 16-color ANSI only — user themes must
// survive. The bg* codes are black-on-color section bars (TUI only);
// their zero values degrade bars to plain text with the same geometry.
type colors struct {
	reset, bold, dim, rev, green, yellow, red string
	bgMagenta, bgGreen, bgYellow, bgRed       string
}

// isTTY reports whether f is a character device (a real terminal).
func isTTY(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

func newColors(out *os.File) colors {
	if os.Getenv("NO_COLOR") != "" || !isTTY(out) {
		return colors{}
	}
	return colors{
		reset: "\x1b[0m", bold: "\x1b[1m", dim: "\x1b[2m", rev: "\x1b[7m",
		green: "\x1b[32m", yellow: "\x1b[33m", red: "\x1b[31m",
		bgMagenta: "\x1b[30;45m", bgGreen: "\x1b[30;42m",
		bgYellow: "\x1b[30;43m", bgRed: "\x1b[30;41m",
	}
}

// syncLine renders the daemon's sync status (B7) as one line.
// local-only is a normal state, never an error (T2).
func syncLine(s syncJSON) string {
	switch s.Mode {
	case "local-only":
		return "local-only"
	case "syncing":
		line := fmt.Sprintf("syncing with %q", s.Remote)
		if t, err := time.Parse(time.RFC3339, s.LastFetch); err == nil {
			line += " · last fetch " + stamp(t)
		}
		return line
	case "error":
		return fmt.Sprintf("sync error (remote %q): %s", s.Remote, s.LastError)
	default:
		return s.Mode // "starting": first cycle hasn't finished
	}
}

// stamp renders an absolute UTC instant — same discipline as
// internal/views: relative times rot the moment they are written.
func stamp(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04 UTC")
}

// humanStatus maps a plumbing status value to the human-facing word
// (T7, 2026-07-31): the ledger event and API value stay "cancelled"
// (T3), but humans read "archived" — the verb that says curation, not
// deletion, because nothing is deleted and history stays on the ledger.
func humanStatus(s string) string {
	if s == "cancelled" {
		return "archived"
	}
	return s
}

// oneLine flattens text for a single-line rendering.
func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	return strings.ReplaceAll(s, "\n", " ")
}

// ellipsize truncates s to at most n runes, marking the cut with "…".
func ellipsize(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// indent prefixes every line of body.
func indent(body, prefix string) string {
	lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}

func labelSuffix(labels []string) string {
	if len(labels) == 0 {
		return ""
	}
	return "  [" + strings.Join(labels, ", ") + "]"
}

func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// ---- shared sections (the one-shot commands render these buckets) ----

func renderReady(w io.Writer, col colors, tasks []stateTask) {
	fmt.Fprintf(w, "%s%sReady%s (%d)\n", col.bold, col.green, col.reset, len(tasks))
	if len(tasks) == 0 {
		fmt.Fprintf(w, "  %snone%s\n", col.dim, col.reset)
		return
	}
	for _, t := range tasks {
		fmt.Fprintf(w, "  %s%s%s  p%d  %s%s\n",
			col.dim, t.ID, col.reset, t.Priority, oneLine(t.Title), labelSuffix(t.Labels))
	}
}

func renderInProgress(w io.Writer, col colors, tasks []stateTask) {
	fmt.Fprintf(w, "%s%sIn progress%s (%d)\n", col.bold, col.yellow, col.reset, len(tasks))
	if len(tasks) == 0 {
		fmt.Fprintf(w, "  %snone%s\n", col.dim, col.reset)
		return
	}
	for _, t := range tasks {
		fmt.Fprintf(w, "  %s%s%s  %s  %s← %s%s\n",
			col.dim, t.ID, col.reset, oneLine(t.Title), col.yellow, t.Holder, col.reset)
	}
}

func renderBlocked(w io.Writer, col colors, s *snapshot, tasks []stateTask) {
	fmt.Fprintf(w, "%s%sBlocked%s (%d)\n", col.bold, col.red, col.reset, len(tasks))
	if len(tasks) == 0 {
		fmt.Fprintf(w, "  %snone%s\n", col.dim, col.reset)
		return
	}
	for _, t := range tasks {
		fmt.Fprintf(w, "  %s%s%s  %s\n      %swaiting:%s %s\n",
			col.dim, t.ID, col.reset, oneLine(t.Title), col.red, col.reset, s.blockedReason(t.ID))
	}
}

func renderOpenEscalations(w io.Writer, col colors, s *snapshot, escs []escalationJSON) {
	// Same header as the TUI's escalations section (T7, 2026-07-30):
	// "Needs Input" softens the severity without renaming the entity.
	fmt.Fprintf(w, "%sNeeds Input%s (%d)\n", col.bold, col.reset, len(escs))
	if len(escs) == 0 {
		fmt.Fprintf(w, "  %snone — the fleet is unblocked%s\n", col.dim, col.reset)
		return
	}
	for _, e := range escs {
		mark := ""
		if e.Blocking {
			mark = fmt.Sprintf("  %s[blocking]%s", col.red, col.reset)
		}
		fmt.Fprintf(w, "  %s%s%s%s\n", col.bold, oneLine(e.Question), col.reset, mark)
		title := ""
		if h, ok := s.tasks[e.Task]; ok {
			title = " (" + oneLine(h.Task.Title) + ")"
		}
		fmt.Fprintf(w, "    task %s%s · asked by %s · raised %s\n",
			e.Task, title, e.Actor, stamp(e.RaisedAt))
	}
}
