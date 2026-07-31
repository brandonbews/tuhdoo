# t-01KYVD31CNTR1EVCDHPHJEV9VK — TUI mouse support: click selects, click again acts as enter

- Status: open — ready
- Priority: 1
- Labels: `cli`, `tui`, `ux`
- Created: 2026-07-31 06:16 UTC by `brandon/claude-code-2`

## Description

Context: dogfood steering ask (Brandon, 2026-07-30): clicking a row should mean what enter means. Bubble Tea has native mouse support; the work is enabling it and hit-testing variable-height rows.

The ask: enable mouse tracking (tea.WithMouseCellMotion); build a rendered-line→row map in the same code path that renders rows (rows are variable height — escalation rows, two-line blocked rows, row spacing — so the map must come from layout, not be re-derived); single click moves the cursor to the row under the pointer; click on the already-selected row (or double-click) triggers the same path as enter. Scroll wheel scrolling the list is optional and cheap if it falls out. Watch mode: clicks may select and open read-only detail, never input.

Accepted consequence to record in the 002 T7 note: enabling mouse tracking captures the pointer, so terminal-native text selection needs shift-click while the TUI runs — a real cost over SSH/tmux. Cheap mitigations (e.g. mouse armed only outside watch mode) are allowed if documented.

Acceptance: model-driven tests with tea.MouseMsg: click selects the correct row across variable-height layouts and scroll offsets; click-on-selected triggers the enter path; watch mode never opens input from a click; make test lint green.

Pointers: cmd/tuhdoo/top.go (View/renderTopRows is the layout source of truth), Bubble Tea MouseMsg/WithMouseCellMotion.

Constraints: boring Go; no new dependencies (bubbletea is already in go.mod).

## History

_No activity yet._
