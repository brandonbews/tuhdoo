# TUI mouse support: click selects, click again acts as enter

`t-01KYVD31CNTR1EVCDHPHJEV9VK`

- **Status:** done
- **Priority:** 1
- **Labels:** `cli` `tui` `ux`
- **Depends on:** [`t-rqjm`](t-01KYT63MB28Z535SMJCA63RQJM.md) (done)
- **Created:** 2026-07-31 06:16 UTC by `brandon/claude-code-2`

## Description

Context: dogfood steering ask (Brandon, 2026-07-30): clicking a row should mean what enter means. Bubble Tea has native mouse support; the work is enabling it and hit-testing variable-height rows.

The ask: enable mouse tracking (tea.WithMouseCellMotion); build a rendered-line→row map in the same code path that renders rows (rows are variable height — escalation rows, two-line blocked rows, row spacing — so the map must come from layout, not be re-derived); single click moves the cursor to the row under the pointer; click on the already-selected row (or double-click) triggers the same path as enter. Scroll wheel scrolling the list is optional and cheap if it falls out. Watch mode: clicks may select and open read-only detail, never input.

Accepted consequence to record in the 002 T7 note: enabling mouse tracking captures the pointer, so terminal-native text selection needs shift-click while the TUI runs — a real cost over SSH/tmux. Cheap mitigations (e.g. mouse armed only outside watch mode) are allowed if documented.

Acceptance: model-driven tests with tea.MouseMsg: click selects the correct row across variable-height layouts and scroll offsets; click-on-selected triggers the enter path; watch mode never opens input from a click; make test lint green.

Pointers: cmd/tuhdoo/top.go (View/renderTopRows is the layout source of truth), Bubble Tea MouseMsg/WithMouseCellMotion.

Constraints: boring Go; no new dependencies (bubbletea is already in go.mod).

## History

### 2026-07-31 07:53 UTC — edit by `brandon/claude-code-9`

depends_on +t-rqjm

### 2026-07-31 15:14 UTC — run by `brandon/claude-code-1` — done

- Branch: `main`
- Commits: `57bf3d3`

Landed in commit 57bf3d3. Mouse tracking via tea.WithMouseCellMotion, enabled for armed panes only — watch mode never captures the pointer, so plain click-drag text selection keeps working in the pane left open to copy from (the documented mitigation; the model-level guard holds regardless and is tested by injecting MouseMsg into a disarmed model). Hit-testing comes from layout, not re-derivation: chunk gained a row field stamped in the same listChunks pass that renders each row; windowChunks split into visibleChunks+joinChunks and the header factored into listHead so View and rowAt consume identical bytes — variable-height rows and scroll offsets cannot drift (tested across two-line escalation/blocked rows, chrome, and scrolled windows on an 80x8 terminal). Click selects; click on the already-selected row runs openRow, the enter handler factored out of updateNav so both inputs share one path — double-click needs no special case (first press selects, second finds it selected). Scroll wheel fell out: list wheel moves the cursor, detail wheel drives detailScroll. Input modes ignore the mouse entirely (stray clicks never disturb a pending prompt). Dated T7 note in 002 records semantics, the accepted shift-click consequence over SSH/tmux, and the armed-only mitigation. Golden tests pass unchanged (refactor byte-identical). make test lint green. Flagged for Brandon: the armed-only mitigation means steer-mode text selection needs shift-click — if that grates, mouse-everywhere or a config knob is a one-line change plus a T7 edit. Clicks inside the detail screen do nothing but wheel-scroll; click-to-answer in detail would need the same layout-replay treatment for detailView — natural follow-up if wanted.
