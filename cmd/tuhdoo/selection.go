package main

// Selection-bar styling for the TUI (2026-07-31): the selected chunk
// renders as a full-height bar — a ▌ gutter on every line plus an
// adaptive background tint resolved down a capability ladder. The
// ladder and tint are pure functions; the terminal interaction lives
// in the one queryTermBG seam, called once before the bubbletea
// program starts. Since 2026-08-27 the quiet-chrome bars ride the
// same answer down their own rung set (chromeBG).

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

// bgAnswer is the terminal's reply to the OSC 11 background-color
// query, or its absence.
type bgAnswer struct {
	ok      bool  // the terminal actually answered
	r, g, b uint8 // the answered background, valid only when ok
}

// selectionBG resolves the selection-bar background SGR down the
// capability ladder, best first: an answered background query earns
// the exact truecolor tint; an unanswered query on a 256-color TERM
// earns the indexed grayscale (the mosh day-to-day case — visually
// Claude Code parity); anything else gets bright-black, the themed
// gray. COLORTERM is deliberately an input the ladder never consults:
// mosh advertises truecolor while swallowing the OSC 11 query
// (empirical, 2026-07-31), so only an actual answer can unlock the
// exact-tint rung.
func selectionBG(ans bgAnswer, term, colorterm string, dark bool) string {
	switch {
	case ans.ok:
		return tintSGR(ans.r, ans.g, ans.b, dark, 8)
	case strings.Contains(term, "256color"):
		if dark {
			return "\x1b[48;5;236m"
		}
		return "\x1b[48;5;253m"
	default:
		return "\x1b[100m"
	}
}

// chromeBG resolves the quiet-chrome bar background — the task view's
// section bars and the CANCELLED history bar — down the same ladder as
// the selection bar, one register stronger (steering, 2026-08-27:
// pinned indexed colors read foreign next to the user's theme). An
// answered query earns a truecolor tint of the actual theme background
// at ~15% versus the selection's 8%, so the two surfaces never read as
// one another; an unanswered query on a 256-color TERM (the mosh
// day-to-day case, where the palette is unreadable) earns a neutral
// indexed background picked by the dark/light verdict; anything else
// keeps the floor's bright-black bar. No foreground is pinned on any
// rung: the theme's own default fg rides the bar. COLORTERM is never
// consulted (2026-07-31).
func chromeBG(ans bgAnswer, term string, dark bool) string {
	switch {
	case ans.ok:
		return tintSGR(ans.r, ans.g, ans.b, dark, 15)
	case strings.Contains(term, "256color"):
		if dark {
			return "\x1b[48;5;238m"
		}
		return "\x1b[48;5;251m"
	default:
		return "\x1b[100m"
	}
}

// tintSGR nudges the terminal's own background pct% toward the
// opposite extreme — lighter on dark themes, darker on light — so the
// bar reads as a tint of the user's theme, never a foreign color. The
// selection bar tints at 8, the chrome bars at 15.
func tintSGR(r, g, b uint8, dark bool, pct int) string {
	tint := func(v uint8) uint8 {
		if dark {
			return v + uint8((255-int(v))*pct/100)
		}
		return uint8(int(v) * (100 - pct) / 100)
	}
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", tint(r), tint(g), tint(b))
}

// darkRGB classifies an answered background by perceived luminance
// (BT.601 weights).
func darkRGB(r, g, b uint8) bool {
	return (299*int(r)+587*int(g)+114*int(b))/1000 < 128
}

// darkANSIIndex classifies a COLORFGBG-style ANSI background index:
// white (7) and the bright colors 9–15 read light; the normal colors
// and bright black read dark.
func darkANSIIndex(i int) bool {
	return i != 7 && (i < 9 || i > 15)
}

// queryTermBG is the ladder's one terminal interaction: ask for the
// background color (OSC 11, via termenv) exactly once, before the
// bubbletea program starts — the query reads the tty, which would
// fight bubbletea's input loop once it owns stdin. termenv pairs the
// query with a cursor-position report, so a terminal that swallows
// OSC 11 (mosh) fails the query fast instead of stalling launch.
// Returns the answer (or its absence) and the dark/light verdict:
// from the answer itself when one arrives, from the COLORFGBG
// fallback termenv surfaces as an ANSI index otherwise, defaulting to
// dark — HasDarkBackground's chain without a second query.
func queryTermBG(out *os.File) (bgAnswer, bool) {
	switch c := termenv.NewOutput(out).BackgroundColor().(type) {
	case termenv.RGBColor:
		var r, g, b uint8
		if _, err := fmt.Sscanf(string(c), "#%02x%02x%02x", &r, &g, &b); err == nil {
			return bgAnswer{ok: true, r: r, g: g, b: b}, darkRGB(r, g, b)
		}
	case termenv.ANSIColor:
		return bgAnswer{}, darkANSIIndex(int(c))
	}
	return bgAnswer{}, true
}

// selectedText re-renders a chunk's lines as the selection bar: a ▌
// gutter replaces the two-cell mark column on every line — glyph, not
// color, so it survives NO_COLOR — and selBG wraps each line,
// re-applied after every internal reset and space-padded to the full
// width so it reads as one continuous bar. Zero-value colors get the
// gutter alone: no padding, no escapes.
func selectedText(col colors, text string, width int) string {
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		if rest, ok := strings.CutPrefix(l, "  "); ok {
			l = "▌ " + rest
		}
		if col.selBG != "" {
			pad := width - ansi.StringWidth(l)
			if pad < 0 {
				pad = 0
			}
			l = col.selBG + strings.ReplaceAll(l, col.reset, col.reset+col.selBG) +
				strings.Repeat(" ", pad) + col.reset
		}
		lines[i] = l
	}
	return strings.Join(lines, "\n")
}
