package main

// The prompt overlay (keyboard/priority grill, 2026-09-11): every TUI
// prompt — answer, priority, cancel y/n, capture, and the title,
// description, and labels editors — renders in a centered bordered
// box over the screen, lazygit-style, instead of riding the footer in
// the key legend's slot. The screen behind stays fully rendered, so
// the row or task being acted on stays in view under the box. Two
// pure functions (T1): overlay composites rendered lines, promptBox
// renders the box; neither touches the model.

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// sgrReset is the reset the compositor writes where it cuts a styled
// base line: the box's own bytes must never inherit a style the base
// left open at the cut.
const sgrReset = "\x1b[0m"

// overlay composites box over base. base is the rendered frame (height
// lines, as pinFrame produces; fewer are padded, more are clipped), box
// the lines to draw, centered horizontally and vertically. The result
// is exactly height lines in which the box's cells replace the base's.
// ANSI-aware: a base line under the box is cut at the box's left edge
// and resumed at its right edge, the cut end carries a reset so a style
// open there never bleeds into the box, and the resumed tail keeps every
// escape the cut skipped (ansi.TruncateLeft passes them through), so its
// own styling is intact. A box taller than the frame is clipped to the
// frame, a box wider than it to its width.
func overlay(base string, box []string, width, height int) string {
	if height < 1 {
		height = 1
	}
	lines := strings.Split(base, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	if len(box) > height {
		box = box[:height]
	}
	boxW := 0
	for _, l := range box {
		if w := ansi.StringWidth(l); w > boxW {
			boxW = w
		}
	}
	if boxW > width {
		boxW = width
	}
	if len(box) == 0 || boxW == 0 {
		return strings.Join(lines, "\n")
	}
	left := (width - boxW) / 2
	top := (height - len(box)) / 2
	for i, bl := range box {
		lines[top+i] = ansi.Truncate(splice(lines[top+i], bl, left, boxW), width, "")
	}
	return strings.Join(lines, "\n")
}

// splice replaces cells [left, left+w) of line with cell, padded to w.
func splice(line, cell string, left, w int) string {
	head := ansi.Truncate(line, left, "")
	if strings.Contains(head, "\x1b") && !strings.HasSuffix(head, sgrReset) {
		head += sgrReset
	}
	head = padCells(head, left)
	tail := ""
	if ansi.StringWidth(line) > left+w {
		tail = ansi.TruncateLeft(line, left+w, "")
	}
	return head + padCells(ansi.Truncate(cell, w, ""), w) + tail
}

// padCells right-pads s with spaces to n cells, ANSI-aware (padTo
// counts runes and would be wrecked by a styled span).
func padCells(s string, n int) string {
	if d := n - ansi.StringWidth(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

// ---- the box ----

// Box geometry: width min(boxMaxWidth, terminal − 4), height fitted to
// the content and capped at 60% of the terminal. Below a terminal too
// small for a bordered box the box renders borderless at full width —
// never clipped, never a panic.
const (
	boxMaxWidth  = 80
	boxMinWidth  = 20 // narrower terminals get the borderless fallback
	boxMinHeight = 6  // shorter ones too
)

// promptFrame resolves the box geometry for a terminal: its outer
// width, the inner width the content renders at, and whether it is
// bordered. An unknown height (0, before the first WindowSizeMsg)
// never forces the fallback: the border decision is the width's alone
// then.
func promptFrame(width, height int) (boxW, inner int, bordered bool) {
	if width < 1 {
		width = 1
	}
	bordered = width >= boxMinWidth && (height <= 0 || height >= boxMinHeight)
	if !bordered {
		return width, width, false
	}
	boxW = min(boxMaxWidth, width-4)
	return boxW, boxW - 4, true
}

// promptCap is the most lines the box may take: 60% of the terminal,
// floored at the smallest box that still shows label, one body line,
// and hint. Unknown height: no cap.
func promptCap(height int, bordered bool) int {
	if height <= 0 {
		return 1 << 30
	}
	floor := 3
	if bordered {
		floor = 5
	}
	return max(height*3/5, floor)
}

// promptContent is what an open input mode shows in the box: the label
// (line 1, bold), the body — the text widget's rows, or the one-line
// question of the cancel confirm — the row to keep visible when the
// body outgrows the box (−1: none), the hint, and a validation error
// that replaces the hint, red, while the prompt stays open.
type promptContent struct {
	label  string
	body   []string
	cursor int
	hint   string
	err    string
}

// promptBox renders the box lines for a terminal: dim single-line
// border in the 16-color palette, bold label, the body scrolled to keep
// the cursor row in view when it exceeds the cap, and the hint last —
// dim, or the validation error in red. Every line is exactly the box
// width; overlay centers them.
func promptBox(col colors, p promptContent, width, height int) []string {
	boxW, inner, bordered := promptFrame(width, height)
	chrome := 2 // label + hint
	if bordered {
		chrome += 2
	}
	avail := max(promptCap(height, bordered)-chrome, 1)
	body := p.body
	if len(body) > avail {
		off := 0
		if p.cursor >= avail {
			off = p.cursor - avail + 1
		}
		if off+avail > len(body) {
			off = len(body) - avail
		}
		body = body[off : off+avail]
	}
	hint := sgr(col, col.dim, ellipsize(p.hint, inner))
	if p.err != "" {
		hint = sgr(col, col.red, ellipsize(p.err, inner))
	}
	content := make([]string, 0, len(body)+2)
	content = append(content, sgr(col, col.bold, ellipsize(p.label, inner)))
	for _, l := range body {
		content = append(content, ansi.Truncate(l, inner, ""))
	}
	content = append(content, hint)

	lines := make([]string, 0, len(content)+2)
	if !bordered {
		for _, c := range content {
			lines = append(lines, padCells(c, boxW))
		}
		return lines
	}
	rule := strings.Repeat("─", boxW-2)
	side := sgr(col, col.dim, "│")
	lines = append(lines, sgr(col, col.dim, "┌"+rule+"┐"))
	for _, c := range content {
		lines = append(lines, side+" "+padCells(c, inner)+" "+side)
	}
	return append(lines, sgr(col, col.dim, "└"+rule+"┘"))
}
