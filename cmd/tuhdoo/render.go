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
// in selection.go may hand it a truecolor or 256-color code; every
// other code stays 16-color.) The bg* codes are black-on-color section
// bars (TUI only); their zero values degrade bars to plain text with
// the same geometry.
type colors struct {
	reset, bold, dim, rev, green, yellow, red, magenta string
	bgMagenta, bgGreen, bgYellow, bgRed                string
	selBG                                              string // selection bar; set by runTUI only, never by newColors
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

// stampCompact is stamp for serialized column output (T7, 2026-07-31):
// the same UTC instant at the same minute precision, with no interior
// spaces, so a timestamp stays one awk/grep token. The trailing Z is a
// literal — the instant is always UTC.
func stampCompact(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04") + "Z"
}

// humanStatus maps a plumbing status value to the human-facing word
// (T7, 2026-07-31): the ledger event and API value stay "cancelled"
// (T3), but humans read "archived" — the verb that says curation, not
// deletion, because nothing is deleted and history stays on the ledger.
// Same split for "held": stored value and --status flag stay "held",
// humans read "on hold" — the phrase says paused-on-purpose where the
// bare participle read ambiguously.
func humanStatus(s string) string {
	switch s {
	case "cancelled":
		return "archived"
	case "held":
		return "on hold"
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

// The digest section renderers (Ready/In progress/Blocked/On hold/
// Inbox/Needs Input) lived here until 2026-07-31, when the one-shot
// commands moved to serialized column output (T7: serialization, not
// design) — see printBacklog / printEscalations in commands.go.
