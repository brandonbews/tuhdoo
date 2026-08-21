package main

// Terminal rendering shared by the read commands and the TUI.

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// colors holds ANSI escape codes, or all-empty strings for plain
// output. Color is on only when out is a terminal and NO_COLOR is unset
// (https://no-color.org). 16-color ANSI only — user themes must
// survive. (Revised 2026-07-31: selBG, the TUI selection-bar
// background, is the one sanctioned exception — the capability ladder
// in selection.go may hand it a truecolor or 256-color code. Revised
// 2026-08-21, priority-badge ramp: orange is the second — it has no
// 16-color slot, so it exists only from the 256-color rung up and
// stays empty on the floor, where p1 falls back to yellow.) The bg*
// codes are the TUI's section
// bars; their zero values degrade bars to plain text with the same
// geometry. Most are black-on-color; bgGray is the shelf bar (chrome
// hierarchy, 2026-08-03): dim foreground on the bright-black
// background — palette slot 8. Bar recolors (2026-08-04): bgRed is
// dim foreground on red — BLOCKED holds only unmet-dependency tasks,
// ordinary sequencing, so it keeps the hue family and drops the
// alarm — and bgWhite (black on bright-white, slot 15) is the INBOX
// bar, replacing reverse-dim. dimRed is the matching foreground for
// the blocked row's waiting: lead. All still inside the 16-color law.
type colors struct {
	reset, bold, dim, rev, green, yellow, red, magenta string
	dimRed                                             string
	bgMagenta, bgGreen, bgYellow, bgRed, bgGray        string
	bgWhite                                            string
	selBG                                              string // selection bar; set by runTUI only, never by newColors
	orange                                             string // p1 badge; set by runTUI on the 256-color rung only, never by newColors
}

// orangeFG resolves the p1 badge's orange down the same capability
// posture as the selection ladder (2026-08-21 ramp): the 256-color
// rung earns indexed orange, everything else gets "" and the badge
// ramp falls back to yellow. COLORTERM is deliberately not consulted —
// the mosh finding (2026-07-31) stands.
func orangeFG(term string) string {
	if strings.Contains(term, "256color") {
		return "\x1b[38;5;208m"
	}
	return ""
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
		green: "\x1b[32m", yellow: "\x1b[33m", red: "\x1b[31m", magenta: "\x1b[35m",
		dimRed:    "\x1b[2;31m",
		bgMagenta: "\x1b[30;45m", bgGreen: "\x1b[30;42m",
		bgYellow: "\x1b[30;43m", bgRed: "\x1b[2;41m",
		bgGray: "\x1b[2;100m", bgWhite: "\x1b[30;107m",
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

// dayStamp is stamp at day precision, for the history surfaces
// (history view, 2026-08-02): when a task closed matters to the day;
// the exact instant lives on the ledger.
func dayStamp(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}

// stampCompact is stamp for serialized column output (T7, 2026-07-31):
// the same UTC instant at the same minute precision, with no interior
// spaces, so a timestamp stays one awk/grep token. The trailing Z is a
// literal — the instant is always UTC.
func stampCompact(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04") + "Z"
}

// The status display words live in views.HumanStatus — the mapping's
// single definition since the status-vocabulary revision (2026-08-01).

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

func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}
