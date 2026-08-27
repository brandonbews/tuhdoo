package main

// Terminal rendering shared by the read commands and the TUI.

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// colors holds ANSI escape codes, or all-empty strings for plain
// output. Color is on only when out is a terminal and NO_COLOR is
// unset (https://no-color.org). The 16-color law: baseline styling is
// 16-color ANSI — user themes must survive — with the 256-color rung
// as the one sanctioned exception ladder. (Revised 2026-07-31: selBG,
// the TUI selection-bar background, may carry a truecolor or 256-color
// tint from the capability ladder in selection.go. Revised 2026-08-21,
// priority-badge ramp: orange has no 16-color slot, so it exists only
// from the 256-color rung up and stays empty on the floor, where p1
// falls back to yellow. Third revision, 2026-08-25, contrast ramp: the
// old "never by newColors" boundary is retired — newColors itself
// resolves the rung, because mosh renders SGR-2 faint as a no-op
// (empirical), which turned every faint surface normal-weight there.
// On a 256color TERM, dimRed swaps SGR-2 for a 256-indexed code; on
// the floor it keeps its 16-color bytes. Indexed codes exist only on
// the rung, never faked on the floor, and COLORTERM is never
// consulted — the mosh finding of 2026-07-31 stands. selBG — and,
// since 2026-08-27, bgDarkGray — still resolve in runTUI: not as a
// boundary, but because their ladders want the OSC 11 answer, an
// interaction only the TUI can make. Fourth revision, 2026-08-26, steering: dim is
// ANSI 90 — bright-black foreground, the slot themes themselves
// designate as the muted gray (gruvbox dark: #928374) — on every
// rung, replacing both the floor's SGR-2 faint and the rung's fixed
// gray 245, which no theme informs. Accepted consequence: themes that
// repurpose slot 8 as a near-background tone (solarized) mute harder
// than intended.) The bg* codes are the TUI's section bars; their
// zero values degrade bars to plain text with the same geometry. Bar
// recolors II (2026-08-25, steering): every dashboard bar is
// black-on-color — bgRed is black on bright red, slot 101, the
// background twin of the p0 badge's 91 (reversing the 2026-08-04
// drop-the-alarm muting), and bgGray is black on gray, slot 7: bright
// enough to carry black text, and distinct from bgWhite's bright-white
// INBOX bar, slot 15. bgDarkGray is the quiet chrome for surfaces
// that are shelf, not queue — the CANCELLED history bar and the task
// view's section bars, which dropped reverse-dim for it: default
// foreground on bright-black here (the floor and the CLI-facing
// value), re-resolved by runTUI down the background ladder (chromeBG,
// 2026-08-27) so it tints the user's actual theme where the terminal
// answers OSC 11. dimRed is the blocked row's waiting: lead — a
// reason line, so it stays muted even under the loud bar.
type colors struct {
	reset, bold, dim, rev, green, yellow, red, magenta string
	brightRed                                          string // p0/negative badge (contrast ramp, 2026-08-25): ANSI 91 — in-palette bright; normal red reads low-contrast on dark themes
	dimRed                                             string
	bgMagenta, bgGreen, bgYellow, bgRed, bgGray        string
	bgWhite, bgDarkGray                                string
	selBG                                              string // selection bar; set by runTUI only — its ladder needs the OSC 11 query
	orange                                             string // p1 badge; 256-color rung only, empty on the floor (yellow fallback)
}

// isTTY reports whether f is a character device (a real terminal).
func isTTY(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// termColors resolves the palette for a TERM, floor first: the
// 16-color set is the baseline every terminal gets — dim included,
// since ANSI 90 is theme-derived and mosh-safe on every rung (fourth
// revision, 2026-08-26). A TERM advertising 256color swaps the one
// faint survivor for an indexed equivalent — mosh renders SGR-2 faint
// as a no-op (empirical, 2026-08-25), so on the rung "muted" must be
// a real color, not an attribute: 131 is the muted red of the
// waiting: lead, distinct from both col.red and plain text. Orange
// rides the same rung check (it lived in runTUI before this ladder
// existed). bgDarkGray stays plain 100 here on every rung: it is
// TUI-only chrome, and runTUI re-resolves it down the background
// ladder (chromeBG, 2026-08-27 — a truecolor theme tint when the
// terminal answers OSC 11, a dark/light-picked neutral bg with the
// theme's own foreground under mosh). COLORTERM is deliberately not
// consulted — the mosh finding (2026-07-31) stands.
func termColors(term string) colors {
	c := colors{
		reset: "\x1b[0m", bold: "\x1b[1m", dim: "\x1b[90m", rev: "\x1b[7m",
		green: "\x1b[32m", yellow: "\x1b[33m", red: "\x1b[31m", magenta: "\x1b[35m",
		brightRed: "\x1b[91m",
		dimRed:    "\x1b[2;31m",
		bgMagenta: "\x1b[30;45m", bgGreen: "\x1b[30;42m",
		bgYellow: "\x1b[30;43m", bgRed: "\x1b[30;101m",
		bgGray: "\x1b[30;47m", bgWhite: "\x1b[30;107m",
		bgDarkGray: "\x1b[100m",
	}
	if strings.Contains(term, "256color") {
		c.dimRed = "\x1b[38;5;131m"
		c.orange = "\x1b[38;5;208m"
	}
	return c
}

func newColors(out *os.File) colors {
	if os.Getenv("NO_COLOR") != "" || !isTTY(out) {
		return colors{}
	}
	return termColors(os.Getenv("TERM"))
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
