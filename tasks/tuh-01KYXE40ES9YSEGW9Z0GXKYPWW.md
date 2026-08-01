# One real text-input widget: delineated box, fixed hint line, standard cursor editing

`tuh-01KYXE40ES9YSEGW9Z0GXKYPWW`

- **Status:** done
- **Priority:** 1
- **Labels:** `cli` `tui` `ux`
- **Created:** 2026-08-01 01:13 UTC by `brandon`

## Description

## Context

Two dogfooding captures from Brandon (2026-08-01) about TUI text entry:

1. In the inbox capture field, the "enter captures / esc cancels" hint is drawn inline after the typed text, so it shifts distractingly with every keystroke.
2. Text inputs support only append-and-backspace (cmd/tuhdoo/top.go ~line 415) — no cursor movement at all.

There are already at least two text entries (inbox capture, escalation answer) and more coming (task-view answering in tuh-01KYXDWWM8S1GF6N9NE5FGA86Y, title/description editing in tuh-01KYXE5376YPXHDS98V3K985M6). Each hand-rolls its input handling.

## The ask

One shared text-input component used by every text entry in the TUI:

- A clearly delineated input box (border or header) with the hint line ("enter captures · esc cancels" etc.) rendered on its own fixed line below the box — it must not move as the user types.
- Standard editing (Brandon confirmed the full set, 2026-07-31): insertion at cursor; left/right arrows; home/end; ctrl+a / ctrl+e; ctrl+k / ctrl+u / ctrl+w; backspace and delete at the cursor; **word motion**: alt/option+left/right (and alt+b / alt+f) jump by word, alt/option+backspace deletes the previous word.
- **Multi-line is a mode of this same widget** (Brandon's call, 2026-07-31 — no separate bespoke editor): the description editor in tuh-01KYXE5376YPXHDS98V3K985M6 uses it. Multi-line mode adds line wrapping, up/down cursor movement across lines, and enter-inserts-newline (submit moves to a different chord, e.g. ctrl+d or ctrl+s — pick one and put it in the hint line). Single-line mode keeps enter-submits.
- All existing entries (inbox capture, escalation answer) migrate onto it.

## Acceptance

- Hint text stays fixed while typing in every input.
- In every input the cursor can be moved mid-string and text inserted/deleted there; every listed key command works, including the alt/option word operations (test the escape-sequence forms a terminal actually sends: ESC b / ESC f / ESC backspace and the modified-arrow CSI forms).
- Multi-line mode: up/down moves across lines, wrapping renders correctly at narrow widths, hint line shows the submit chord.
- No per-screen bespoke input handling remains — one widget, table-driven tests over its editing operations (both modes), golden tests for the fixed-hint rendering. make test lint green.

## Pointers

- cmd/tuhdoo/top.go (~line 409-420 for the current input loop), cmd/tuhdoo/render.go.

## Constraints

- Boring Go. The TUI is bubbletea; charmbracelet/bubbles textinput/textarea are acceptable same-family dependencies, but a plain struct with a []rune buffer and a cursor index is equally fine — pick whichever stays more auditable given multi-line is required.
- Merged from inbox captures tuh-01KYXE40ES9YSEGW9Z0GXKYPWW, tuh-01KYXE4NSNBFFRTT8STNDJHYED.

## History

### 2026-08-01 02:40 UTC — run by `brandon/claude-code-1` — interrupted

lease expired without a finish or release

_Synthesized by replay, not recorded by the agent._

### 2026-08-01 02:57 UTC — run by `brandon/claude-code-1` — done

- Branch: `tuh-ypww/text-input-widget`
- PR: <https://github.com/brandonbews/tuhdoo/pull/10>
- Commits: `0a9727b`

Landed on main as 0a9727b (PR #10, squash). New cmd/tuhdoo/textinput.go: value-semantics textInput widget ([]rune buffer + cursor), rendered as a delineated box with header bar, > gutter, cursor glyph, and the hint on its own fixed line — it no longer shifts while typing. Full confirmed editing set incl. alt/option word ops, tested via the exact KeyMsg forms bubbletea produces (ESC b/f, ESC backspace, CSI 1;3D/C). Multi-line is a mode of the same widget: hard wrap, up/down across wrapped rows, enter-newline, ctrl+s submits (ctrl+d rejected: readline forward-delete collision). All three entries (answer, priority, capture) migrated; updateInput owns only mode keys; no bespoke editing remains. Hand-rolled over bubbles: no lipgloss second styling system (T1 auditability); go.mod untouched. 41-case table-driven suite + goldens; make test lint and CI green. Note: the prior interrupted run on this task was this same session — the daemon restart after PR #9 killed the harness MCP session, so the lease could not renew during the build; work continued uninterrupted and this run is its completion. Deliberately not done: ctrl+h-as-backspace (not in confirmed set), kill-ring/yank, multi-line answers (one-line flip at the modeAnswer entry points if dogfooding wants it). The future description editor task should set multiline: true on this widget.
