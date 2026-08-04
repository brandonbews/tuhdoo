package main

// Tests for the shared text-input widget (textinput.go): table-driven
// editing operations in both modes, rendering goldens for the
// fixed-hint box, and model-level checks that every TUI entry —
// answer, priority, capture — runs on the same widget through Update
// (T1: deterministic, table-driven).

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// The tea.KeyMsg forms bubbletea (v1 key tables) delivers for the
// escape sequences a terminal actually sends for the alt/option
// chords — tests drive Update and handleKey with these exact values:
//
//	ESC b     → KeyRunes{'b'}, Alt      ESC f     → KeyRunes{'f'}, Alt
//	ESC DEL   → KeyBackspace, Alt
//	CSI 1;3D  → KeyLeft, Alt            CSI 1;3C  → KeyRight, Alt
var (
	altB         = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}, Alt: true}
	altF         = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}, Alt: true}
	altBackspace = tea.KeyMsg{Type: tea.KeyBackspace, Alt: true}
	altLeft      = tea.KeyMsg{Type: tea.KeyLeft, Alt: true}
	altRight     = tea.KeyMsg{Type: tea.KeyRight, Alt: true}
)

// inputAt builds a textInput from a fixture where | marks the cursor.
func inputAt(t *testing.T, s string, multiline bool) textInput {
	t.Helper()
	i := strings.IndexRune(s, '|')
	if i < 0 {
		t.Fatalf("fixture %q has no | cursor marker", s)
	}
	return textInput{
		buf:       []rune(strings.Replace(s, "|", "", 1)),
		cursor:    len([]rune(s[:i])),
		multiline: multiline,
	}
}

// withCursor renders a textInput back into | fixture form.
func withCursor(in textInput) string {
	return string(in.buf[:in.cursor]) + "|" + string(in.buf[in.cursor:])
}

func applyKeys(in textInput, width int, keys ...tea.KeyMsg) textInput {
	for _, k := range keys {
		in = in.handleKey(k, width)
	}
	return in
}

func TestTextInputEditing(t *testing.T) {
	tests := []struct {
		name      string
		multiline bool
		width     int // terminal width; 0 = 80
		start     string
		keys      []tea.KeyMsg
		want      string
	}{
		// ---- single-line ----
		{name: "insert at cursor mid-string", start: "he|lo",
			keys: runes("l"), want: "hel|lo"},
		{name: "left moves and clamps", start: "ab|",
			keys: []tea.KeyMsg{keyOf(tea.KeyLeft), keyOf(tea.KeyLeft), keyOf(tea.KeyLeft)}, want: "|ab"},
		{name: "right moves and clamps", start: "|ab",
			keys: []tea.KeyMsg{keyOf(tea.KeyRight), keyOf(tea.KeyRight), keyOf(tea.KeyRight)}, want: "ab|"},
		{name: "home to start", start: "abc|", keys: []tea.KeyMsg{keyOf(tea.KeyHome)}, want: "|abc"},
		{name: "ctrl+a to start", start: "abc|", keys: []tea.KeyMsg{keyOf(tea.KeyCtrlA)}, want: "|abc"},
		{name: "end to end", start: "|abc", keys: []tea.KeyMsg{keyOf(tea.KeyEnd)}, want: "abc|"},
		{name: "ctrl+e to end", start: "|abc", keys: []tea.KeyMsg{keyOf(tea.KeyCtrlE)}, want: "abc|"},
		{name: "backspace deletes before cursor", start: "ab|c",
			keys: []tea.KeyMsg{keyOf(tea.KeyBackspace)}, want: "a|c"},
		{name: "backspace at start is a no-op", start: "|abc",
			keys: []tea.KeyMsg{keyOf(tea.KeyBackspace)}, want: "|abc"},
		{name: "delete removes at cursor", start: "a|bc",
			keys: []tea.KeyMsg{keyOf(tea.KeyDelete)}, want: "a|c"},
		{name: "delete at end is a no-op", start: "abc|",
			keys: []tea.KeyMsg{keyOf(tea.KeyDelete)}, want: "abc|"},
		{name: "ctrl+k kills to line end", start: "ab|cd",
			keys: []tea.KeyMsg{keyOf(tea.KeyCtrlK)}, want: "ab|"},
		{name: "ctrl+u kills to line start", start: "ab|cd",
			keys: []tea.KeyMsg{keyOf(tea.KeyCtrlU)}, want: "|cd"},
		{name: "ctrl+w deletes previous word with trailing space", start: "one two |",
			keys: []tea.KeyMsg{keyOf(tea.KeyCtrlW)}, want: "one |"},
		{name: "ctrl+w mid-word deletes to word start", start: "one tw|o",
			keys: []tea.KeyMsg{keyOf(tea.KeyCtrlW)}, want: "one |o"},
		{name: "alt+backspace (ESC DEL) deletes previous word", start: "one two|",
			keys: []tea.KeyMsg{altBackspace}, want: "one |"},
		{name: "alt+left (CSI 1;3D) jumps to word start", start: "one two|",
			keys: []tea.KeyMsg{altLeft}, want: "one |two"},
		{name: "alt+b (ESC b) jumps to word start", start: "one two|",
			keys: []tea.KeyMsg{altB}, want: "one |two"},
		{name: "alt+left twice reaches the front", start: "one two|",
			keys: []tea.KeyMsg{altLeft, altLeft}, want: "|one two"},
		{name: "alt+right (CSI 1;3C) jumps past the word", start: "|one two",
			keys: []tea.KeyMsg{altRight}, want: "one| two"},
		{name: "alt+f (ESC f) jumps past the next word", start: "one| two",
			keys: []tea.KeyMsg{altF}, want: "one two|"},
		{name: "up and down are dead in single-line", start: "a|b",
			keys: []tea.KeyMsg{keyOf(tea.KeyUp), keyOf(tea.KeyDown)}, want: "a|b"},
		{name: "enter is not an edit in single-line", start: "a|b",
			keys: []tea.KeyMsg{keyOf(tea.KeyEnter)}, want: "a|b"},
		{name: "space inserts at cursor", start: "a|b",
			keys: []tea.KeyMsg{{Type: tea.KeySpace, Runes: []rune{' '}}}, want: "a |b"},
		{name: "paste flattens newlines in single-line", start: "|",
			keys: []tea.KeyMsg{{Type: tea.KeyRunes, Runes: []rune("x\ny"), Paste: true}}, want: "x y|"},
		{name: "multibyte runes edit by rune, not byte", start: "é|é",
			keys: runes("ü"), want: "éü|é"},

		// ---- multi-line ----
		{name: "enter inserts a newline", multiline: true, start: "ab|",
			keys: []tea.KeyMsg{keyOf(tea.KeyEnter)}, want: "ab\n|"},
		{name: "up clamps the column", multiline: true, start: "ab\ncdef|",
			keys: []tea.KeyMsg{keyOf(tea.KeyUp)}, want: "ab|\ncdef"},
		{name: "down clamps the column", multiline: true, start: "abcd|\nef",
			keys: []tea.KeyMsg{keyOf(tea.KeyDown)}, want: "abcd\nef|"},
		{name: "up at the top is a no-op", multiline: true, start: "a|b\ncd",
			keys: []tea.KeyMsg{keyOf(tea.KeyUp)}, want: "a|b\ncd"},
		{name: "down at the bottom is a no-op", multiline: true, start: "ab\nc|d",
			keys: []tea.KeyMsg{keyOf(tea.KeyDown)}, want: "ab\nc|d"},
		// width 7 → wrap width 4: "aaaabb" renders as rows aaaa / bb,
		// and up/down walk those rendered rows, not logical lines.
		{name: "up crosses a wrapped row", multiline: true, width: 7, start: "aaaabb|",
			keys: []tea.KeyMsg{keyOf(tea.KeyUp)}, want: "aa|aabb"},
		{name: "down crosses a wrapped row", multiline: true, width: 7, start: "aa|aabb",
			keys: []tea.KeyMsg{keyOf(tea.KeyDown)}, want: "aaaabb|"},
		{name: "ctrl+k at line end joins the next line", multiline: true, start: "ab|\ncd",
			keys: []tea.KeyMsg{keyOf(tea.KeyCtrlK)}, want: "ab|cd"},
		{name: "ctrl+k kills only the line tail", multiline: true, start: "a|bc\nd",
			keys: []tea.KeyMsg{keyOf(tea.KeyCtrlK)}, want: "a|\nd"},
		{name: "ctrl+a stops at the line start", multiline: true, start: "ab\ncd|",
			keys: []tea.KeyMsg{keyOf(tea.KeyCtrlA)}, want: "ab\n|cd"},
		{name: "ctrl+e stops at the line end", multiline: true, start: "a|b\ncd",
			keys: []tea.KeyMsg{keyOf(tea.KeyCtrlE)}, want: "ab|\ncd"},
		{name: "ctrl+u kills to the line start only", multiline: true, start: "ab\ncd|ef",
			keys: []tea.KeyMsg{keyOf(tea.KeyCtrlU)}, want: "ab\n|ef"},
		{name: "alt+backspace crosses a newline", multiline: true, start: "one\n|",
			keys: []tea.KeyMsg{altBackspace}, want: "|"},
		{name: "word motion crosses newlines", multiline: true, start: "one\ntwo|",
			keys: []tea.KeyMsg{altB, altB}, want: "|one\ntwo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			width := tt.width
			if width == 0 {
				width = 80
			}
			in := inputAt(t, tt.start, tt.multiline)
			if got := withCursor(applyKeys(in, width, tt.keys...)); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// Edits build fresh buffers: a model copy holding the pre-edit value
// never sees bytes move under it.
func TestTextInputEditsNeverAlias(t *testing.T) {
	a := inputAt(t, "one |two", false)
	b := applyKeys(a, 80, runes("X")...)
	c := applyKeys(a, 80, keyOf(tea.KeyCtrlW))
	if got := withCursor(a); got != "one |two" {
		t.Errorf("original mutated by edits on copies: %q", got)
	}
	if got := withCursor(b); got != "one X|two" {
		t.Errorf("insert on copy = %q, want %q", got, "one X|two")
	}
	if got := withCursor(c); got != "|two" {
		t.Errorf("ctrl+w on copy = %q, want %q", got, "|two")
	}
}

func TestTextInputRows(t *testing.T) {
	eq := func(a, b []inputRow) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}
	tests := []struct {
		text string
		w    int
		want []inputRow
	}{
		{"", 4, []inputRow{{0, 0}}},
		{"abcd", 4, []inputRow{{0, 4}}},
		{"abcde", 4, []inputRow{{0, 4}, {4, 5}}},
		{"ab\ncd", 4, []inputRow{{0, 2}, {3, 5}}},
		{"ab\n\ncd", 4, []inputRow{{0, 2}, {3, 3}, {4, 6}}},
		{"abcd\nef", 4, []inputRow{{0, 4}, {5, 7}}},
		{"abcd\n", 4, []inputRow{{0, 4}, {5, 5}}},
	}
	for _, tt := range tests {
		in := textInput{buf: []rune(tt.text), multiline: true}
		if got := in.rows(tt.w); !eq(got, tt.want) {
			t.Errorf("rows(%q, %d) = %v, want %v", tt.text, tt.w, got, tt.want)
		}
	}
}

// The box at 80 columns, plain colors: header bar geometry, the cursor
// glyph mid-string, and the hint on its own line — byte-exact.
func TestTextInputViewSingleLinePlain(t *testing.T) {
	in := inputAt(t, "id|ea", false)
	got := in.view(colors{}, "capture (to inbox)", "captures", 80)
	want := strings.Join([]string{
		" capture (to inbox)" + strings.Repeat(" ", 61),
		"> id█ea",
		"  enter captures · esc cancels",
		"",
	}, "\n")
	if got != want {
		t.Errorf("single-line box diverged.\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// Single-line mode scrolls a window instead of wrapping: the box stays
// three lines tall, the cursor stays visible, and no line exceeds the
// width — at the tail and back at the head.
func TestTextInputViewSingleLineScrolls(t *testing.T) {
	in := inputAt(t, "abcdefghij|", false)
	v := in.view(colors{}, "x", "submits", 10)
	lines := strings.Split(strings.TrimRight(v, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("box is %d lines, want 3:\n%s", len(lines), v)
	}
	if lines[1] != "> efghij█" {
		t.Errorf("scrolled input line = %q, want %q", lines[1], "> efghij█")
	}
	for _, l := range lines {
		if n := len([]rune(l)); n > 10 {
			t.Errorf("line is %d runes wide (>10): %q", n, l)
		}
	}
	in = applyKeys(in, 10, keyOf(tea.KeyHome))
	lines = strings.Split(strings.TrimRight(in.view(colors{}, "x", "submits", 10), "\n"), "\n")
	if lines[1] != "> █abcdef" {
		t.Errorf("home-scrolled input line = %q, want %q", lines[1], "> █abcdef")
	}
}

// Multi-line at a narrow width: logical lines render whole, the hint
// line shows the ctrl+s submit chord (ellipsized to the width), and
// nothing exceeds the width.
func TestTextInputViewMultilinePlain(t *testing.T) {
	in := inputAt(t, "one two three\nfour|", true)
	got := in.view(colors{}, "edit", "saves", 20)
	want := strings.Join([]string{
		" edit" + strings.Repeat(" ", 15),
		"> one two three",
		"> four█",
		"  ctrl+s saves · en…",
		"",
	}, "\n")
	if got != want {
		t.Errorf("multi-line box diverged.\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// Hard wrapping at the box width: rows break exactly where up/down
// walk, the glyph lands on the wrapped row, and a cursor sitting on a
// soft-wrap boundary renders at the start of the following row.
func TestTextInputViewMultilineWraps(t *testing.T) {
	in := inputAt(t, "aa|aabb", true) // width 7 → wrap width 4
	v := in.view(colors{}, "e", "saves", 7)
	lines := strings.Split(strings.TrimRight(v, "\n"), "\n")
	if lines[1] != "> aa█aa" || lines[2] != "> bb" {
		t.Errorf("wrapped rows wrong: %q", lines)
	}
	for _, l := range lines {
		if n := len([]rune(l)); n > 7 {
			t.Errorf("line is %d runes wide (>7): %q", n, l)
		}
	}
	in = inputAt(t, "aaaa|bb", true)
	lines = strings.Split(strings.TrimRight(in.view(colors{}, "e", "saves", 7), "\n"), "\n")
	if lines[1] != "> aaaa" || lines[2] != "> █bb" {
		t.Errorf("boundary cursor rows wrong: %q", lines)
	}
}

// Styled rendering: one style wraps the whole padded header bar, the
// hint is dim, and the input line itself carries no styling (the
// glyph is the cursor).
func TestTextInputViewStyled(t *testing.T) {
	in := inputAt(t, "hi|", false)
	v := in.view(ansiColors, "answer · Q?", "submits", 40)
	for _, want := range []string{
		"\x1b[7m\x1b[1m answer · Q?" + strings.Repeat(" ", 28) + "\x1b[0m\n",
		"\n> hi█\n",
		"  \x1b[2menter submits · esc cancels\x1b[0m\n",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("styled box missing %q; got:\n%q", want, v)
		}
	}
}

func TestTextInputHint(t *testing.T) {
	if got := (textInput{}).hint("submits"); got != "enter submits · esc cancels" {
		t.Errorf("single-line hint = %q", got)
	}
	if got := (textInput{multiline: true}).hint("saves"); got != "ctrl+s saves · enter newline · esc cancels" {
		t.Errorf("multi-line hint = %q", got)
	}
}

// ---- the widget through the model: one input loop for every entry ----

// Every entry edits mid-string through Update: capture, answer, and
// priority all run on the shared widget — no per-screen input handling.
func TestTopAllInputsEditMidString(t *testing.T) {
	// Capture: fix a typo at the head, then rebuild a word by motion.
	fake := newFakeSteering()
	m := newTopModel(fake)
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	m, _ = press(t, m, runes("hllo wrld")...)
	m, _ = press(t, m, keyOf(tea.KeyHome), keyOf(tea.KeyRight))
	m, _ = press(t, m, runes("e")...) // h|llo → he|llo
	m, _ = press(t, m, keyOf(tea.KeyEnd), altB, keyOf(tea.KeyRight))
	m, _ = press(t, m, runes("o")...) // wrld → world
	m, cmd := press(t, m, keyOf(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("capture submit produced no command")
	}
	if am := cmd().(actionMsg); am.err != nil {
		t.Fatalf("capture error: %v", am.err)
	}
	if len(fake.captured) != 1 || fake.captured[0] != "hello world" {
		t.Errorf("captured %v, want [hello world]", fake.captured)
	}

	// Answer: replace a word mid-string with alt+left and delete.
	fake = newFakeSteering()
	m = newTopModel(fake)
	m, _ = press(t, m, keyOf(tea.KeyEnter), keyOf(tea.KeyEnter)) // task view → answer entry
	m, _ = press(t, m, runes("Use GPL.")...)
	m, _ = press(t, m, altLeft,
		keyOf(tea.KeyDelete), keyOf(tea.KeyDelete), keyOf(tea.KeyDelete))
	m, _ = press(t, m, runes("MIT")...)
	m, cmd = press(t, m, keyOf(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("answer submit produced no command")
	}
	if am := cmd().(actionMsg); am.err != nil {
		t.Fatalf("answer error: %v", am.err)
	}
	if got := fake.answers["01E1"]; got != "Use MIT." {
		t.Errorf("answered with %q, want %q", got, "Use MIT.")
	}

	// Priority: cursor left, backspace deletes the mistyped leading digit.
	fake = newFakeSteering()
	m = newTopModel(fake)
	m, _ = press(t, m,
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}, // t-pars
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m, _ = press(t, m, runes("17")...)
	m, _ = press(t, m, keyOf(tea.KeyLeft), keyOf(tea.KeyBackspace))
	m, cmd = press(t, m, keyOf(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("priority submit produced no command")
	}
	if am := cmd().(actionMsg); am.err != nil {
		t.Fatalf("priority error: %v", am.err)
	}
	if got := fake.priorities["t-pars"]; got != 7 {
		t.Errorf("priority set to %d, want 7", got)
	}
}

// The alt chords land through the model exactly as the terminal sends
// them (the KeyMsg forms of ESC b / ESC f / ESC DEL / CSI 1;3D / 1;3C).
func TestTopInputAltChordsThroughUpdate(t *testing.T) {
	m := newTopModel(newFakeSteering())
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	m, _ = press(t, m, runes("alpha beta gamma")...)
	m, _ = press(t, m, altBackspace)
	if got := m.input.String(); got != "alpha beta " {
		t.Fatalf("alt+backspace left %q, want %q", got, "alpha beta ")
	}
	m, _ = press(t, m, altLeft, altB)
	if m.input.cursor != 0 {
		t.Errorf("alt+left, alt+b: cursor %d, want 0", m.input.cursor)
	}
	m, _ = press(t, m, altF)
	if m.input.cursor != 5 {
		t.Errorf("alt+f: cursor %d, want 5 (end of alpha)", m.input.cursor)
	}
	m, _ = press(t, m, altRight)
	if m.input.cursor != 10 {
		t.Errorf("alt+right: cursor %d, want 10 (end of beta)", m.input.cursor)
	}
}

// The hint never moves while typing (dogfood capture, 2026-08-01): in
// every entry it renders on its own line below the box, byte-identical
// and on the same screen line before and after keystrokes.
func TestTopInputHintFixedWhileTyping(t *testing.T) {
	entries := map[string][]tea.KeyMsg{
		"capture":  {{Type: tea.KeyRunes, Runes: []rune{'i'}}},
		"answer":   {keyOf(tea.KeyEnter), keyOf(tea.KeyEnter)},
		"priority": {{Type: tea.KeyRunes, Runes: []rune{'j'}}, {Type: tea.KeyRunes, Runes: []rune{'p'}}},
	}
	for name, open := range entries {
		t.Run(name, func(t *testing.T) {
			m := newTopModel(newFakeSteering())
			m.width, m.height = 80, 40
			m, _ = press(t, m, open...)
			hintLine := func(m topModel) (int, string) {
				for i, l := range strings.Split(m.View(), "\n") {
					if strings.Contains(l, "esc cancels") {
						return i, l
					}
				}
				t.Fatalf("no hint line rendered; view:\n%s", m.View())
				return -1, ""
			}
			i0, l0 := hintLine(m)
			m, _ = press(t, m, runes("someinput")...)
			i1, l1 := hintLine(m)
			if i0 != i1 || l0 != l1 {
				t.Errorf("hint moved while typing: line %d %q → line %d %q", i0, l0, i1, l1)
			}
			if strings.Contains(l1, "someinput") {
				t.Errorf("typed text leaked onto the hint line: %q", l1)
			}
		})
	}
}

// The capture box in the full frame at 80 columns, plain colors:
// byte-exact footer geometry (golden for the fixed-hint rendering).
func TestTopGoldenCaptureBoxPlain80(t *testing.T) {
	m := newTopModel(newFakeSteering())
	m.width, m.height = 80, 40
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	m, _ = press(t, m, runes("idea")...)
	v := m.View()
	// The pinned frame's bottom row is unterminated (no trailing
	// newline): the hint line is the view's last byte.
	want := strings.Join([]string{
		" capture (to inbox)" + strings.Repeat(" ", 61),
		"> idea█",
		"  enter captures · esc cancels",
	}, "\n")
	if !strings.HasSuffix(v, want) {
		t.Errorf("capture box footer diverged.\ngot:\n%s\nwant suffix:\n%s", v, want)
	}
	if strings.Contains(v, "\x1b") {
		t.Errorf("plain render leaked ANSI escapes:\n%q", v)
	}
}

// Multi-line is a mode of the same loop: enter inserts a newline
// instead of submitting, the hint advertises the ctrl+s chord, and
// ctrl+s submits. (The description editor — the description stop in
// the task view — is the mode's real consumer; this pins the loop
// itself, on a capture forced multi-line.)
func TestTopInputMultilineEnterAndCtrlS(t *testing.T) {
	fake := newFakeSteering()
	m := newTopModel(fake)
	m.width, m.height = 80, 40
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	m.input.multiline = true
	m, _ = press(t, m, runes("line one")...)
	m, cmd := press(t, m, keyOf(tea.KeyEnter))
	if cmd != nil || m.mode != modeCapture {
		t.Fatalf("enter submitted a multi-line input: mode %d cmd %v", m.mode, cmd)
	}
	m, _ = press(t, m, runes("line two")...)
	v := m.View()
	if !strings.Contains(v, "ctrl+s captures · enter newline · esc cancels") {
		t.Errorf("multi-line hint missing the submit chord; view:\n%s", v)
	}
	if !strings.Contains(v, "> line one\n> line two█") {
		t.Errorf("multi-line box not rendering both lines; view:\n%s", v)
	}
	m, cmd = press(t, m, keyOf(tea.KeyCtrlS))
	if cmd == nil {
		t.Fatal("ctrl+s produced no command")
	}
	if am := cmd().(actionMsg); am.err != nil {
		t.Fatalf("submit error: %v", am.err)
	}
	if len(fake.captured) != 1 || fake.captured[0] != "line one\nline two" {
		t.Errorf("captured %v, want the two-line title", fake.captured)
	}
}

// editInput seeds the widget for in-place editing: the given value,
// cursor at the end, in the asked-for mode; the empty value stays the
// zero-value entry.
func TestEditInput(t *testing.T) {
	in := editInput("fix the typo", false)
	if got := withCursor(in); got != "fix the typo|" || in.multiline {
		t.Errorf("editInput single-line = %q (multiline %v)", got, in.multiline)
	}
	// Prefilled text edits like typed text: word motion and deletion
	// land where the cursor sits.
	in = applyKeys(in, 80, keyOf(tea.KeyCtrlW))
	if got := withCursor(in); got != "fix the |" {
		t.Errorf("ctrl+w on prefilled = %q", got)
	}
	in = editInput("one\ntwo", true)
	if got := withCursor(in); got != "one\ntwo|" || !in.multiline {
		t.Errorf("editInput multi-line = %q (multiline %v)", got, in.multiline)
	}
	if got := withCursor(editInput("", true)); got != "|" {
		t.Errorf("editInput empty = %q", got)
	}
}

// The title editor's box in the full detail frame at 80 columns, plain
// colors: prefilled buffer, cursor at the end, single-line hint —
// byte-exact footer geometry.
func TestTopGoldenEditTitleBoxPlain80(t *testing.T) {
	m := newTopModel(newFakeSteering())
	m.width, m.height = 80, 40
	m = openDetail(t, m, "t-flak")
	m, _ = press(t, m, keyOf(tea.KeyEnter)) // the plain open leaves the title focused
	v := m.View()
	want := strings.Join([]string{
		" title t-flak" + strings.Repeat(" ", 67),
		"> investigate the flake█",
		"  enter saves · esc cancels",
	}, "\n")
	if !strings.HasSuffix(v, want) {
		t.Errorf("title box footer diverged.\ngot:\n%s\nwant suffix:\n%s", v, want)
	}
	if strings.Contains(v, "\x1b") {
		t.Errorf("plain render leaked ANSI escapes:\n%q", v)
	}
}

// The description editor's box in the full detail frame at 80 columns,
// plain colors: the multi-line prefill renders one gutter line per
// logical line, cursor on the last, ctrl+s hint fixed below.
func TestTopGoldenEditDescBoxPlain80(t *testing.T) {
	m := newTopModel(newFakeSteering())
	m.width, m.height = 80, 40
	m = openDetail(t, m, "t-flak")
	m, _ = press(t, m, // walk the ring past priority to the description
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
		keyOf(tea.KeyEnter))
	v := m.View()
	want := strings.Join([]string{
		" description t-flak (investigate the flake)" + strings.Repeat(" ", 37),
		"> The parser test flakes on CI.",
		"> Find out why.█",
		"  ctrl+s saves · enter newline · esc cancels",
	}, "\n")
	if !strings.HasSuffix(v, want) {
		t.Errorf("description box footer diverged.\ngot:\n%s\nwant suffix:\n%s", v, want)
	}
	if strings.Contains(v, "\x1b") {
		t.Errorf("plain render leaked ANSI escapes:\n%q", v)
	}
}

// The input box rides the detail footer without overflowing the
// terminal: the body window shrinks by the box's extra lines.
func TestTopDetailInputBoxFitsHeight(t *testing.T) {
	m := openDetail(t, newTopModelWithDep(newFakeSteering()), "t-lic")
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
	m = mm.(topModel)
	m, _ = press(t, m, // ring past priority to the escalation, then answer entry
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
		keyOf(tea.KeyEnter))
	m, _ = press(t, m, runes("Use MIT.")...)
	v := m.View()
	if n := strings.Count(strings.TrimRight(v, "\n"), "\n") + 1; n > 10 {
		t.Errorf("frame is %d lines, terminal is 10; view:\n%s", n, v)
	}
	for _, want := range []string{"answer · Which license?", "> Use MIT.█", "enter submits · esc cancels"} {
		if !strings.Contains(v, want) {
			t.Errorf("detail input box missing %q; view:\n%s", want, v)
		}
	}
}
