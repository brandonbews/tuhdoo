package main

// Table-driven tests for the selection-bar capability ladder and its
// pure helpers (T1). The one impure seam, queryTermBG, is exercised
// only through its pure downstream.

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// The ladder: an answered query earns the exact tint; an unanswered
// one on a 256-color TERM earns the indexed gray — including when
// COLORTERM advertises truecolor, the mosh case the ladder must never
// trust; everything else gets bright-black.
func TestSelectionBGLadder(t *testing.T) {
	tests := []struct {
		name      string
		ans       bgAnswer
		term      string
		colorterm string
		dark      bool
		want      string
	}{
		{"answered dark theme lightens ~8%",
			bgAnswer{ok: true, r: 0x1e, g: 0x1e, b: 0x2e}, "xterm-256color", "truecolor", true,
			"\x1b[48;2;48;48;62m"},
		{"answered light theme darkens ~8%",
			bgAnswer{ok: true, r: 0xff, g: 0xff, b: 0xff}, "xterm-256color", "", false,
			"\x1b[48;2;234;234;234m"},
		{"answered pure black still lightens",
			bgAnswer{ok: true}, "xterm", "", true,
			"\x1b[48;2;20;20;20m"},
		{"unanswered + COLORTERM=truecolor lands on indexed (mosh)",
			bgAnswer{}, "xterm-256color", "truecolor", true,
			"\x1b[48;5;236m"},
		{"unanswered 256-color light theme",
			bgAnswer{}, "screen-256color", "", false,
			"\x1b[48;5;253m"},
		{"unanswered 16-color TERM",
			bgAnswer{}, "xterm", "", true,
			"\x1b[100m"},
		{"no TERM at all",
			bgAnswer{}, "", "", true,
			"\x1b[100m"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := selectionBG(tt.ans, tt.term, tt.colorterm, tt.dark); got != tt.want {
				t.Errorf("selectionBG(%+v, %q, %q, %v) = %q, want %q",
					tt.ans, tt.term, tt.colorterm, tt.dark, got, tt.want)
			}
		})
	}
}

func TestDarkRGB(t *testing.T) {
	tests := []struct {
		r, g, b uint8
		want    bool
	}{
		{0, 0, 0, true},
		{255, 255, 255, false},
		{0x1e, 0x1e, 0x2e, true},  // a typical dark theme bg
		{0xfa, 0xfa, 0xfa, false}, // a typical light theme bg
		{128, 128, 128, false},    // mid-gray sits on the light side
		{0, 0, 255, true},         // pure blue is perceptually dark
		{255, 255, 0, false},      // yellow is perceptually light
	}
	for _, tt := range tests {
		if got := darkRGB(tt.r, tt.g, tt.b); got != tt.want {
			t.Errorf("darkRGB(%d, %d, %d) = %v, want %v", tt.r, tt.g, tt.b, got, tt.want)
		}
	}
}

func TestDarkANSIIndex(t *testing.T) {
	tests := []struct {
		i    int
		want bool
	}{
		{0, true},   // black — the classic COLORFGBG dark value
		{7, false},  // white
		{8, true},   // bright black
		{15, false}, // bright white — the classic light value
		{3, true},
		{12, false},
	}
	for _, tt := range tests {
		if got := darkANSIIndex(tt.i); got != tt.want {
			t.Errorf("darkANSIIndex(%d) = %v, want %v", tt.i, got, tt.want)
		}
	}
}

// selectedText under real colors: gutter in the mark column on every
// line, bg from edge to edge with resets re-applied, full-width pad.
func TestSelectedText(t *testing.T) {
	col := ansiColors
	col.selBG = "\x1b[48;5;236m"
	text := "  " + sgr(col, col.dim, "t-lic ") + "  title\n" +
		strings.Repeat(" ", gridTitleCol(gridIDW)) + "plain second line"
	got := strings.Split(selectedText(col, text, 40), "\n")
	if len(got) != 2 {
		t.Fatalf("line count changed: %d, want 2", len(got))
	}
	for _, l := range got {
		if !strings.HasPrefix(l, "\x1b[48;5;236m▌ ") {
			t.Errorf("line does not open with bg+gutter: %q", l)
		}
		if !strings.HasSuffix(l, "\x1b[0m") {
			t.Errorf("line does not close with a reset: %q", l)
		}
		if w := ansi.StringWidth(l); w != 40 {
			t.Errorf("line is %d cells, want 40: %q", w, l)
		}
	}
	if !strings.Contains(got[0], "\x1b[0m\x1b[48;5;236m") {
		t.Errorf("bg not re-applied after the styled span's reset: %q", got[0])
	}
}

// Zero-value colors (NO_COLOR / non-TTY): the ▌ glyph alone marks
// selection — no bg, no padding, zero escape bytes.
func TestSelectedTextPlain(t *testing.T) {
	text := "  t-lic   !   choose a license\n" +
		strings.Repeat(" ", gridTitleCol(gridIDW)) + "question: Which license?"
	got := selectedText(colors{}, text, 80)
	want := "▌ t-lic   !   choose a license\n" +
		"▌ " + strings.Repeat(" ", gridTitleCol(gridIDW)-2) + "question: Which license?"
	if got != want {
		t.Errorf("plain selection:\ngot  %q\nwant %q", got, want)
	}
	if strings.Contains(got, "\x1b") {
		t.Errorf("plain selection leaked ANSI escapes: %q", got)
	}
}
