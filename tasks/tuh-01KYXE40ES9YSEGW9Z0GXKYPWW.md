# tuh-01KYXE40ES9YSEGW9Z0GXKYPWW — One real text-input widget: delineated box, fixed hint line, standard cursor editing

- Status: open — ready
- Priority: 1
- Labels: `cli`, `tui`, `ux`
- Created: 2026-08-01 01:13 UTC by `brandon`

## Description

## Context

Two dogfooding captures from Brandon (2026-08-01) about TUI text entry:

1. In the inbox capture field, the "enter captures / esc cancels" hint is drawn inline after the typed text, so it shifts distractingly with every keystroke.
2. Text inputs support only append-and-backspace (cmd/tuhdoo/top.go ~line 415) — no cursor movement at all.

There are already at least two text entries (inbox capture, escalation answer) and more coming (task-view answering in tuh-01KYXDWWM8S1GF6N9NE5FGA86Y, title/description editing in tuh-01KYXE5376YPXHDS98V3K985M6). Each hand-rolls its input handling.

## The ask

One shared text-input component used by every text entry in the TUI:

- A clearly delineated input box (border or header) with the hint line ("enter captures · esc cancels" etc.) rendered on its own fixed line below the box — it must not move as the user types.
- Standard editing: insertion at cursor, left/right arrows, home/end, ctrl+a / ctrl+e, ctrl+k / ctrl+u / ctrl+w, backspace and delete at the cursor.
- All existing entries (inbox capture, escalation answer) migrate onto it.

## Acceptance

- Hint text stays fixed while typing in every input.
- In every input the cursor can be moved mid-string and text inserted/deleted there; the listed key commands all work.
- No per-screen bespoke input handling remains — one widget, table-driven tests over its editing operations, golden tests for the fixed-hint rendering. make test lint green.

## Pointers

- cmd/tuhdoo/top.go (~line 409-420 for the current input loop), cmd/tuhdoo/render.go.

## Constraints

- Boring Go. The TUI is bubbletea; charmbracelet/bubbles/textinput is an acceptable same-family dependency, but a plain struct with a []rune buffer and a cursor index is equally fine — pick whichever stays more auditable.
- Merged from inbox captures tuh-01KYXE40ES9YSEGW9Z0GXKYPWW, tuh-01KYXE4NSNBFFRTT8STNDJHYED.

## History

_No activity yet._
