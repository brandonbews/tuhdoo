package main

// The prompt overlay (2026-09-11): the compositor and the box renderer
// are pure functions — table-driven tests over rendered lines (T1).

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// frameOf builds a base frame of n identical lines.
func frameOf(line string, n int) string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}

// overlay centers the box on even and odd sizes, keeps the frame at
// exactly height lines, replaces cells rather than inserting them, and
// never lets a line exceed the width.
func TestOverlayCentersAndReplaces(t *testing.T) {
	tests := []struct {
		name          string
		base          string
		box           []string
		width, height int
		want          []string
	}{
		{
			name: "even width and height", base: frameOf("0123456789", 4),
			box: []string{"ab", "cd"}, width: 10, height: 4,
			want: []string{"0123456789", "0123ab6789", "0123cd6789", "0123456789"},
		},
		{
			name: "odd width and height", base: frameOf("012345678", 5),
			box: []string{"abc"}, width: 9, height: 5,
			want: []string{"012345678", "012345678", "012abc678", "012345678", "012345678"},
		},
		{
			name: "short base lines are padded up to the box edge", base: frameOf("", 3),
			box: []string{"ab"}, width: 10, height: 3,
			want: []string{"", "    ab", ""},
		},
		{
			name: "base lines ending under the box lose no tail", base: frameOf("01234", 3),
			box: []string{"ab"}, width: 10, height: 3,
			want: []string{"01234", "0123ab", "01234"},
		},
		{
			name: "ragged box lines are padded to the box width", base: frameOf("0123456789", 3),
			box: []string{"abc", "d"}, width: 10, height: 3,
			want: []string{"012abc6789", "012d  6789", "0123456789"},
		},
		{
			name: "box taller than the frame is clipped to it", base: frameOf("0123456789", 3),
			box: []string{"a", "b", "c", "d", "e"}, width: 10, height: 3,
			want: []string{"0123a56789", "0123b56789", "0123c56789"},
		},
		{
			name: "base with fewer lines than height is padded", base: "0123456789",
			box: []string{"ab"}, width: 10, height: 3,
			want: []string{"0123456789", "    ab", ""},
		},
		{
			name: "base with more lines than height is clipped", base: frameOf("0123456789", 6),
			box: []string{"ab"}, width: 10, height: 3,
			want: []string{"0123456789", "0123ab6789", "0123456789"},
		},
		{
			name: "empty box leaves the frame alone", base: frameOf("0123456789", 2),
			box: nil, width: 10, height: 2,
			want: []string{"0123456789", "0123456789"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := overlay(tt.base, tt.box, tt.width, tt.height)
			lines := strings.Split(got, "\n")
			if len(lines) != tt.height {
				t.Fatalf("frame is %d lines, want exactly %d:\n%s", len(lines), tt.height, got)
			}
			for i, l := range lines {
				if l != tt.want[i] {
					t.Errorf("line %d = %q, want %q", i, l, tt.want[i])
				}
				if w := ansi.StringWidth(l); w > tt.width {
					t.Errorf("line %d is %d cells, wider than %d: %q", i, w, tt.width, l)
				}
			}
			if strings.Contains(got, "\x1b") {
				t.Errorf("plain input grew an escape sequence:\n%q", got)
			}
		})
	}
}

// A styled base line cut at the box edges: the head ends in a reset at
// the cut, so the style open there never bleeds into the box, and the
// tail resumes with its own styling intact — the escapes the cut
// skipped are carried through, never dropped.
func TestOverlayCutResetsStyle(t *testing.T) {
	red := "\x1b[31m0123456789\x1b[0m"
	got := overlay(frameOf(red, 3), []string{"XX"}, 10, 3)
	lines := strings.Split(got, "\n")
	if lines[0] != red || lines[2] != red {
		t.Errorf("lines outside the box changed: %q", lines)
	}
	cut := lines[1]
	if !strings.Contains(cut, "0123\x1b[0mXX") {
		t.Errorf("no reset at the cut before the box: %q", cut)
	}
	if !strings.Contains(cut, "XX\x1b[31m6789\x1b[0m") {
		t.Errorf("tail after the box lost its red: %q", cut)
	}
	if ansi.Strip(cut) != "0123XX6789" {
		t.Errorf("visible cells = %q, want 0123XX6789", ansi.Strip(cut))
	}
	if strings.Contains(cut, "\x1b[0m\x1b[0m") {
		t.Errorf("doubled reset at the cut: %q", cut)
	}
	// A style still open at the cut (no reset anywhere on the line) is
	// closed by the compositor — the one case where it must add bytes.
	open := "\x1b[1m0123456789"
	cut = strings.Split(overlay(frameOf(open, 1), []string{"XX"}, 10, 1), "\n")[0]
	if !strings.HasPrefix(cut, "\x1b[1m0123\x1b[0mXX") {
		t.Errorf("open bold not reset at the cut: %q", cut)
	}
	// The selection bar's shape — bg re-applied after an internal reset,
	// full-width — keeps its bg on both sides of the box.
	bar := "\x1b[48;5;236m▌ \x1b[1mab\x1b[0m\x1b[48;5;236m      \x1b[0m"
	cut = strings.Split(overlay(frameOf(bar, 1), []string{"XX"}, 10, 1), "\n")[0]
	if ansi.Strip(cut) != "▌ abXX    " {
		t.Errorf("selection bar cells = %q", ansi.Strip(cut))
	}
	if !strings.Contains(cut, "\x1b[0mXX\x1b[48;5;236m") {
		t.Errorf("bar tail after the box lost its bg: %q", cut)
	}
}

// promptFrame: the box is min(80, width−4) wide with a four-cell
// border allowance, and a terminal too small for a bordered box gets
// the full-width borderless fallback; an unknown height never forces
// the fallback.
func TestPromptFrameGeometry(t *testing.T) {
	tests := []struct {
		width, height int
		boxW, inner   int
		bordered      bool
	}{
		{80, 40, 76, 72, true},
		{120, 40, 80, 76, true},
		{40, 40, 36, 32, true},
		{20, 6, 16, 12, true},
		{19, 40, 19, 19, false}, // too narrow
		{80, 5, 80, 80, false},  // too short
		{18, 3, 18, 18, false},
		{80, 0, 76, 72, true}, // height unknown: bordered by width alone
		{0, 0, 1, 1, false},
	}
	for _, tt := range tests {
		boxW, inner, bordered := promptFrame(tt.width, tt.height)
		if boxW != tt.boxW || inner != tt.inner || bordered != tt.bordered {
			t.Errorf("promptFrame(%d, %d) = (%d, %d, %v), want (%d, %d, %v)",
				tt.width, tt.height, boxW, inner, bordered, tt.boxW, tt.inner, tt.bordered)
		}
	}
	for _, tt := range []struct{ height, capB, capU int }{
		{40, 24, 24}, {10, 6, 6}, {6, 5, 3}, {5, 5, 3}, {3, 5, 3},
	} {
		if got := promptCap(tt.height, true); got != tt.capB {
			t.Errorf("promptCap(%d, bordered) = %d, want %d", tt.height, got, tt.capB)
		}
		if got := promptCap(tt.height, false); got != tt.capU {
			t.Errorf("promptCap(%d, borderless) = %d, want %d", tt.height, got, tt.capU)
		}
	}
}

// promptBox, plain colors: the border rules and sides degrade to bare
// glyphs, the label sits on line 1, the body follows, the hint is the
// last inner line, and every line is exactly the box width.
func TestPromptBoxPlain(t *testing.T) {
	p := promptContent{label: "capture (to inbox)", body: []string{"> idea█"}, hint: "enter captures · esc cancels"}
	got := promptBox(colors{}, p, 80, 40)
	want := []string{
		"┌" + strings.Repeat("─", 74) + "┐",
		"│ capture (to inbox)" + strings.Repeat(" ", 54) + " │",
		"│ > idea█" + strings.Repeat(" ", 65) + " │",
		"│ enter captures · esc cancels" + strings.Repeat(" ", 44) + " │",
		"└" + strings.Repeat("─", 74) + "┘",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("plain box diverged.\ngot:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	for i, l := range got {
		if w := ansi.StringWidth(l); w != 76 {
			t.Errorf("line %d is %d cells, want 76: %q", i, w, l)
		}
	}
	// A label or hint wider than the inner width ellipsizes; the body
	// is truncated (the widget already fits it).
	long := promptBox(colors{}, promptContent{label: strings.Repeat("L", 100), body: []string{strings.Repeat("b", 100)}, hint: strings.Repeat("h", 100)}, 80, 40)
	for i, l := range long {
		if w := ansi.StringWidth(l); w != 76 {
			t.Errorf("long line %d is %d cells, want 76: %q", i, w, l)
		}
	}
	if !strings.HasPrefix(long[1], "│ "+strings.Repeat("L", 71)+"…") {
		t.Errorf("long label not ellipsized: %q", long[1])
	}
}

// promptBox with real colors: dim border, bold label, dim hint — and a
// validation error replaces the hint in red. All 16-color.
func TestPromptBoxStyled(t *testing.T) {
	p := promptContent{label: "title t-flak", body: []string{"> x█"}, hint: "enter saves · esc cancels"}
	got := promptBox(ansiColors, p, 80, 40)
	rule := strings.Repeat("─", 74)
	for i, want := range []string{
		"\x1b[90m┌" + rule + "┐\x1b[0m",
		"\x1b[90m│\x1b[0m \x1b[1mtitle t-flak\x1b[0m" + strings.Repeat(" ", 60) + " \x1b[90m│\x1b[0m",
		"\x1b[90m│\x1b[0m > x█" + strings.Repeat(" ", 68) + " \x1b[90m│\x1b[0m",
		"\x1b[90m│\x1b[0m \x1b[90menter saves · esc cancels\x1b[0m" + strings.Repeat(" ", 47) + " \x1b[90m│\x1b[0m",
		"\x1b[90m└" + rule + "┘\x1b[0m",
	} {
		if got[i] != want {
			t.Errorf("styled box line %d = %q, want %q", i, got[i], want)
		}
	}
	p.err = "title cannot be empty"
	got = promptBox(ansiColors, p, 80, 40)
	if want := "\x1b[90m│\x1b[0m \x1b[31mtitle cannot be empty\x1b[0m" + strings.Repeat(" ", 51) + " \x1b[90m│\x1b[0m"; got[3] != want {
		t.Errorf("error hint line = %q, want %q", got[3], want)
	}
	if strings.Contains(strings.Join(got, ""), "enter saves") {
		t.Error("the hint survives beside the error; the error replaces it")
	}
}

// The body scrolls inside the box when it exceeds the cap, keeping the
// cursor row visible: at 80x10 the cap is 6 lines — two body lines
// between the chrome.
func TestPromptBoxScrollsToCursor(t *testing.T) {
	body := []string{"> l1", "> l2", "> l3", "> l4", "> l5█"}
	box := promptBox(colors{}, promptContent{label: "d", body: body, cursor: 4, hint: "h"}, 80, 10)
	if len(box) != 6 {
		t.Fatalf("box is %d lines, want the 6-line cap:\n%s", len(box), strings.Join(box, "\n"))
	}
	joined := strings.Join(box, "\n")
	for _, want := range []string{"> l4", "> l5█"} {
		if !strings.Contains(joined, want) {
			t.Errorf("box missing %q:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "> l1") || strings.Contains(joined, "> l3") {
		t.Errorf("rows above the window leaked:\n%s", joined)
	}
	// Cursor near the top: the window starts at row 0.
	box = promptBox(colors{}, promptContent{label: "d", body: body, cursor: 0, hint: "h"}, 80, 10)
	joined = strings.Join(box, "\n")
	if !strings.Contains(joined, "> l1") || !strings.Contains(joined, "> l2") || strings.Contains(joined, "> l3") {
		t.Errorf("top window wrong:\n%s", joined)
	}
	// Mid-body cursor: the window ends on the cursor row.
	box = promptBox(colors{}, promptContent{label: "d", body: body, cursor: 2, hint: "h"}, 80, 10)
	joined = strings.Join(box, "\n")
	if !strings.Contains(joined, "> l2") || !strings.Contains(joined, "> l3") || strings.Contains(joined, "> l4") {
		t.Errorf("mid window wrong:\n%s", joined)
	}
}

// The borderless fallback below the minimum terminal: full width, no
// box glyphs, label / body / hint only, nothing wider than the terminal.
func TestPromptBoxBorderlessFallback(t *testing.T) {
	p := promptContent{label: "capture (to inbox)", body: []string{"> idea█"}, hint: "enter captures · esc cancels"}
	for _, tt := range []struct{ width, height int }{{18, 40}, {80, 5}, {18, 3}} {
		box := promptBox(colors{}, p, tt.width, tt.height)
		joined := strings.Join(box, "\n")
		for _, glyph := range []string{"┌", "│", "└", "─"} {
			if strings.Contains(joined, glyph) {
				t.Errorf("%dx%d: borderless fallback drew %q:\n%s", tt.width, tt.height, glyph, joined)
			}
		}
		if len(box) < 3 {
			t.Errorf("%dx%d: fallback box has %d lines, want label, body, hint", tt.width, tt.height, len(box))
		}
		for _, l := range box {
			if w := ansi.StringWidth(l); w != tt.width {
				t.Errorf("%dx%d: fallback line is %d cells, want the full %d: %q", tt.width, tt.height, w, tt.width, l)
			}
		}
	}
}

// assertOverlay checks a prompt render against the no-prompt render of
// the same screen (plain colors, width×height): outside the box's rows
// the frame is byte-identical, and inside them the base's cells
// flanking the box are byte-identical while the box's cells are
// exactly box — centered as overlay places it.
func assertOverlay(t *testing.T, before, after string, box []string, width, height int) {
	t.Helper()
	bl, al := strings.Split(before, "\n"), strings.Split(after, "\n")
	if len(bl) != height || len(al) != height {
		t.Fatalf("frames are %d and %d lines, want %d each", len(bl), len(al), height)
	}
	boxW := ansi.StringWidth(box[0])
	left, top := (width-boxW)/2, (height-len(box))/2
	for i := range al {
		if i < top || i >= top+len(box) {
			if al[i] != bl[i] {
				t.Errorf("row %d outside the box changed:\n  before %q\n  after  %q", i, bl[i], al[i])
			}
			continue
		}
		// The base's cells left of the box (padded up to its edge when
		// the line is shorter) and right of it (only what the line had).
		b := []rune(bl[i])
		head := padTo(string(b[:min(len(b), left)]), left)
		tail := ""
		if len(b) > left+boxW {
			tail = string(b[left+boxW:])
		}
		want := head + box[i-top] + tail
		if al[i] != want {
			t.Errorf("row %d under the box diverged:\n  got  %q\n  want %q", i, al[i], want)
		}
	}
}

// plainBox renders the expected plain-color box lines for inner lines
// at 80 columns (box 76 wide, inner 72), the shape the goldens pin.
func plainBox(inner ...string) []string {
	rule := strings.Repeat("─", 74)
	out := []string{"┌" + rule + "┐"}
	for _, l := range inner {
		out = append(out, "│ "+padTo(l, 72)+" │")
	}
	return append(out, "└"+rule+"┘")
}
