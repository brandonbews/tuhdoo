package main

// The shared text-input widget (tasks tuh-…GXKYPWW, tuh-…TNDJHYED):
// every text entry in the TUI — answer, priority, capture, and any
// future editor — runs on this one struct. Hand-rolled on a []rune
// buffer and a cursor index, not bubbles textinput/textarea: the TUI's
// rendering is hand-rolled SGR (render.go's colors, no lipgloss), so
// the same-family widgets would drag in a second styling system for
// code no one here can audit line by line (T1: boring Go).
//
// The widget renders as a delineated box — a header bar naming the
// entry, the buffer lines behind a "> " gutter, and the hint on its
// own fixed line below the box, so the hint never rides the end of
// the typed text (dogfood capture, 2026-08-01).
//
// Multi-line is a mode of this same widget (Brandon, 2026-07-31):
// it adds hard wrapping at the box width, up/down movement across the
// wrapped rows, and enter-inserts-newline; submit moves to ctrl+s
// (see hint). Single-line mode keeps enter-submits — enter is the
// caller's key there, never an edit.

import (
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
)

// textInput is a value type: every edit builds a fresh buffer, so
// model copies (bubbletea passes models by value) never share bytes.
type textInput struct {
	buf       []rune
	cursor    int // rune index into buf, 0..len(buf)
	multiline bool
}

func (in textInput) String() string { return string(in.buf) }

// inputInnerWidth is the wrap width inside the box: the terminal
// width minus the two-cell "> " gutter and one cell for the cursor
// glyph, so no rendered line ever exceeds the terminal. Zero width
// (no WindowSizeMsg yet) falls back to the mockup's 80 columns.
func inputInnerWidth(width int) int {
	if width <= 0 {
		width = 80
	}
	w := width - 3
	if w < 1 {
		w = 1
	}
	return w
}

// handleKey applies one editing key and returns the updated input.
// width is the terminal width — up/down movement walks the same
// wrapped rows view renders at that width. The caller owns the mode
// keys — submit (enter single-line, ctrl+s multi-line), esc, ctrl+c —
// and never routes them here; everything else lands here, so no
// screen hand-rolls editing.
//
// The switch is over key *types*, not k.String(): a fast typist's
// batched KeyRunes message can spell a key name ("home"), and the
// alt chords arrive as distinct forms — the sequences a terminal
// actually sends map (bubbletea key tables) to:
//
//	ESC b / ESC f       → KeyRunes{'b'/'f'}, Alt: true
//	ESC backspace       → KeyBackspace, Alt: true
//	CSI 1;3D / CSI 1;3C → KeyLeft / KeyRight, Alt: true
func (in textInput) handleKey(k tea.KeyMsg, width int) textInput {
	switch k.Type {
	case tea.KeyLeft:
		if k.Alt {
			in.cursor = prevWordStart(in.buf, in.cursor)
		} else if in.cursor > 0 {
			in.cursor--
		}
	case tea.KeyRight:
		if k.Alt {
			in.cursor = nextWordEnd(in.buf, in.cursor)
		} else if in.cursor < len(in.buf) {
			in.cursor++
		}
	case tea.KeyHome, tea.KeyCtrlA:
		in.cursor = lineStart(in.buf, in.cursor)
	case tea.KeyEnd, tea.KeyCtrlE:
		in.cursor = lineEnd(in.buf, in.cursor)
	case tea.KeyUp:
		in = in.moveRow(-1, inputInnerWidth(width))
	case tea.KeyDown:
		in = in.moveRow(+1, inputInnerWidth(width))
	case tea.KeyBackspace:
		if k.Alt { // ESC backspace: delete the previous word
			return in.deleteRange(prevWordStart(in.buf, in.cursor), in.cursor)
		}
		if in.cursor > 0 {
			return in.deleteRange(in.cursor-1, in.cursor)
		}
	case tea.KeyDelete:
		if in.cursor < len(in.buf) {
			return in.deleteRange(in.cursor, in.cursor+1)
		}
	case tea.KeyCtrlW:
		return in.deleteRange(prevWordStart(in.buf, in.cursor), in.cursor)
	case tea.KeyCtrlK:
		// Kill to line end; at the line end, kill the newline instead
		// (the emacs rule) — a no-op in single-line mode.
		if end := lineEnd(in.buf, in.cursor); end > in.cursor {
			return in.deleteRange(in.cursor, end)
		} else if in.cursor < len(in.buf) {
			return in.deleteRange(in.cursor, in.cursor+1)
		}
	case tea.KeyCtrlU:
		return in.deleteRange(lineStart(in.buf, in.cursor), in.cursor)
	case tea.KeyEnter:
		// Editing only in multi-line mode; single-line enter is the
		// caller's submit and never reaches here.
		if in.multiline {
			return in.insert([]rune{'\n'})
		}
	case tea.KeySpace:
		return in.insert([]rune{' '})
	case tea.KeyRunes:
		if k.Alt { // ESC b / ESC f: word motion; other alt chords are dead
			switch string(k.Runes) {
			case "b":
				in.cursor = prevWordStart(in.buf, in.cursor)
			case "f":
				in.cursor = nextWordEnd(in.buf, in.cursor)
			}
			return in
		}
		return in.insert(k.Runes)
	}
	return in
}

// insert places runes at the cursor. Single-line mode flattens
// newlines (a bracketed paste can carry them) to spaces.
func (in textInput) insert(rs []rune) textInput {
	if !in.multiline {
		flat := make([]rune, len(rs))
		for i, r := range rs {
			if r == '\n' || r == '\r' {
				r = ' '
			}
			flat[i] = r
		}
		rs = flat
	}
	out := make([]rune, 0, len(in.buf)+len(rs))
	out = append(out, in.buf[:in.cursor]...)
	out = append(out, rs...)
	out = append(out, in.buf[in.cursor:]...)
	in.buf = out
	in.cursor += len(rs)
	return in
}

// deleteRange removes buf[from:to) into a fresh buffer, leaving the
// cursor at the cut.
func (in textInput) deleteRange(from, to int) textInput {
	out := make([]rune, 0, len(in.buf)-(to-from))
	out = append(out, in.buf[:from]...)
	out = append(out, in.buf[to:]...)
	in.buf, in.cursor = out, from
	return in
}

// Word boundaries are whitespace runs (the shell's ctrl+w rule, one
// definition for every word operation): backward skips spaces then
// the word; forward mirrors it. Newlines are whitespace, so word
// motion crosses lines in multi-line mode.
func prevWordStart(buf []rune, i int) int {
	for i > 0 && unicode.IsSpace(buf[i-1]) {
		i--
	}
	for i > 0 && !unicode.IsSpace(buf[i-1]) {
		i--
	}
	return i
}

func nextWordEnd(buf []rune, i int) int {
	for i < len(buf) && unicode.IsSpace(buf[i]) {
		i++
	}
	for i < len(buf) && !unicode.IsSpace(buf[i]) {
		i++
	}
	return i
}

// lineStart/lineEnd bound the logical line around i. Single-line
// buffers have no newlines, so these are 0 and len — which makes
// home/end/ctrl+a/ctrl+e/ctrl+k/ctrl+u correct in both modes.
func lineStart(buf []rune, i int) int {
	for i > 0 && buf[i-1] != '\n' {
		i--
	}
	return i
}

func lineEnd(buf []rune, i int) int {
	for i < len(buf) && buf[i] != '\n' {
		i++
	}
	return i
}

// inputRow is one rendered row of the buffer: a [start, end) rune
// range. Rows are the single geometry both rendering and up/down
// movement consume, so the cursor can never disagree with the screen.
type inputRow struct{ start, end int }

// rows splits the buffer into rendered rows: logical lines broken at
// newlines, hard-wrapped at w runes. Always at least one row.
func (in textInput) rows(w int) []inputRow {
	if w < 1 {
		w = 1
	}
	var out []inputRow
	start := 0
	for i, r := range in.buf {
		if r == '\n' {
			out = append(out, inputRow{start, i})
			start = i + 1
			continue
		}
		if i-start == w {
			out = append(out, inputRow{start, i})
			start = i
		}
	}
	return append(out, inputRow{start, len(in.buf)})
}

// cursorRow finds the row the cursor renders on. A cursor sitting on
// a soft-wrap boundary belongs to the following row (where the next
// typed rune lands); at a newline or the buffer end it stays on the
// row it terminates.
func (in textInput) cursorRow(rows []inputRow) int {
	for i, r := range rows {
		hard := r.end == len(in.buf) || in.buf[r.end] == '\n'
		if in.cursor < r.end || (hard && in.cursor == r.end) {
			return i
		}
	}
	return len(rows) - 1
}

// moveRow moves the cursor up or down one rendered row (multi-line
// only), clamping the column to the target row's length; at the top
// or bottom edge it does nothing. w is the wrap width view renders at.
func (in textInput) moveRow(delta, w int) textInput {
	if !in.multiline {
		return in
	}
	rows := in.rows(w)
	r := in.cursorRow(rows)
	nr := r + delta
	if nr < 0 || nr >= len(rows) {
		return in
	}
	col := in.cursor - rows[r].start
	if max := rows[nr].end - rows[nr].start; col > max {
		col = max
	}
	in.cursor = rows[nr].start + col
	return in
}

// hint is the fixed hint-line text: single-line submits on enter,
// multi-line moves submit to ctrl+s so enter can insert newlines.
// verb is the submit word the screen advertises ("submits",
// "captures"). Owning the wording here is what keeps every entry's
// hint — including the submit chord — one convention, not per-screen
// prose.
func (in textInput) hint(verb string) string {
	if in.multiline {
		return "ctrl+s " + verb + " · enter newline · esc cancels"
	}
	return "enter " + verb + " · esc cancels"
}

// view renders the input box at the terminal width: a header bar
// carrying the label, the buffer behind the "> " gutter with the █
// cursor glyph, and the dim hint on its own line below the box. Every
// line fits the width; single-line mode scrolls a window over the
// buffer instead of wrapping, so the box stays exactly three lines
// tall no matter what is typed.
func (in textInput) view(col colors, label, verb string, width int) string {
	if width <= 0 {
		width = 80
	}
	inner := inputInnerWidth(width)
	var b strings.Builder
	b.WriteString(barLine(col, col.rev+col.bold, " "+label, "", width))
	b.WriteString("\n")
	if in.multiline {
		rows := in.rows(inner)
		cr := in.cursorRow(rows)
		for i, r := range rows {
			line := string(in.buf[r.start:r.end])
			if i == cr {
				line = string(in.buf[r.start:in.cursor]) + "█" + string(in.buf[in.cursor:r.end])
			}
			b.WriteString("> " + line + "\n")
		}
	} else {
		// The cursor glyph sits at the cursor; the window over the
		// glyph-bearing line keeps it in view when the text outgrows
		// the box.
		disp := make([]rune, 0, len(in.buf)+1)
		disp = append(disp, in.buf[:in.cursor]...)
		disp = append(disp, '█')
		disp = append(disp, in.buf[in.cursor:]...)
		off := 0
		if in.cursor+1 > inner {
			off = in.cursor + 1 - inner
		}
		end := off + inner
		if end > len(disp) {
			end = len(disp)
		}
		b.WriteString("> " + string(disp[off:end]) + "\n")
	}
	b.WriteString("  " + sgr(col, col.dim, ellipsize(in.hint(verb), width-2)) + "\n")
	return b.String()
}
