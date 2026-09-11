# Prompt overlay: every TUI prompt renders in a centered box over the screen

`tuh-01M28YZVWS1PJEMG9J7TZKTSV0`

- **Status:** open — blocked on dependencies
- **Priority:** 1
- **Labels:** `go` `tui`
- **Depends on:** [`tuh-8q7x`](tuh-01M26H99QVANVJCXR9FM2Z8Q7X.md) (open)
- **Created:** 2026-09-11 19:26 UTC by `brandon/claude-code-1`

## Description

Context: every TUI prompt — answer, priority, cancel y/n, capture, and the title/description/labels editors — renders as the footer, pinned to the bottom row, replacing the key legend (cmd/tuhdoo/top.go inputFooter, footerView, detailFooter, pinFrame). Decided 2026-09-11 (keyboard/priority grill, Brandon): prompts appear over the top of the screen, lazygit-style, in a centered bordered box; the screen behind stays fully rendered so the row or task being acted on stays in view. This task is the rendering mechanism only — no key changes (those are the keymap rework task, which depends on this). It depends on the versioned-state task because both rewrite top.go's frame code and that one lands first.

The ask:
1. A pure compositor: `overlay(base string, box []string, width, height int) string` in a new cmd/tuhdoo/overlay.go — takes the rendered base frame (exactly height lines, as pinFrame produces) and box lines, centers the box horizontally and vertically, and returns a frame of exactly height lines where the box's cells replace the base's cells. ANSI-aware: reuse the existing cell-width/wrap helpers (wrapTo and friends in top.go/render.go) rather than adding a dependency; a base line under the box is cut at the box's left edge, resumed at its right edge, and its SGR state reset at the cut so colors never bleed. Table-tested on plain and colored input.
2. The box: a dim single-line border in the 16-color palette (no new colors), width = min(80, terminal width − 4), height fitted to content and capped at 60% of the terminal height. Line 1 inside the border is the prompt's label (the same label strings inputFooter uses today, bold). Then the body: the text widget's rows for text prompts (textinput.go view, re-pointed at the box's inner width), or the one-line question for cancel y/n. The last inner line is the hint, dim, right where the widget's hint line sits today ("enter submits · esc cancels", "ctrl+s saves · esc cancels", "y/n"). The multi-line description editor scrolls inside the box when its rows exceed the cap, keeping the cursor row visible (textInput.rows / cursorRow already compute rows).
3. View(): when m.mode is an input mode, render the underlying screen exactly as it renders with no prompt open — list or task view, normal footer legend included — then composite the box over it. inputFooter/detailFooter stop rendering prompts. Below a terminal too small for the box (width < 20 or height < 6), fall back to the box at full width with no border rather than crashing or clipping.
4. Mouse: while an input mode is live, updateMouse ignores every click (rowAt/detailStopAt are never consulted), so a click cannot act on a row under the box.
5. Status-line errors raised by submit ("priority must be an integer", "title cannot be empty") render inside the box on the hint line, red, instead of on the status line behind it, and the prompt stays open.

Acceptance (tests must exist and pass):
- overlay table test: box centered on even and odd sizes; a colored base line cut at the box edges has no SGR bleed (assert the reset sequence at the cut); box taller than the cap is clipped to the cap; a frame of height lines in, height lines out, every line ≤ width cells.
- TUI golden tests for each prompt (answer, priority, cancel, capture, title, description, labels) from both the list and the task view: the base screen behind is byte-identical to the no-prompt golden outside the box region; the box appears at the computed position. Regenerate the existing prompt goldens; the no-prompt goldens are unchanged.
- A description editor golden whose content exceeds the cap shows the cursor row inside the box.
- A test that a click on a row under an open box does not change the cursor or mode.
- Small-terminal test (width 18) renders the borderless fallback without panic.
- `make test lint` green; PR title = task title; body opens with this task ID.

Pointers: cmd/tuhdoo/top.go (View, detailView, pinFrame, inputFooter, footerView, detailFooter, updateMouse, rowAt, detailStopAt, submit's error branches, wrapTo), cmd/tuhdoo/textinput.go (view, hint, rows, cursorRow, inputInnerWidth), cmd/tuhdoo/render.go (colors), cmd/tuhdoo/top_golden_test.go, 002 T7 (16-color law and its two sanctioned exceptions; hand-rolled SGR, no lipgloss).

Constraints: boring Go, pure compositor with table tests (T1); no lipgloss or bubbles; no new colors; no key or mode changes; one-shot CLI output untouched (T7 output contract). Add a revision note to 002 T7 (the chrome-hierarchy/footer paragraph) recording that prompts moved off the footer into a centered overlay and why. Deploy after landing per CLAUDE.md.

## History

_No activity yet._
