# t-01KYVD31CNTR1EVCDHPG0G4GMZ — TUI navigation: up/down arrows move the cursor; footer says so

- Status: done
- Priority: 1
- Labels: `cli`, `tui`, `ux`
- Created: 2026-07-31 06:16 UTC by `brandon/claude-code-2`

## Description

Context: dogfood steering feedback (Brandon, 2026-07-30): the footer advertises `j/k move` only, but arrows are the keys a human actually reaches for. If arrows already work, this is a copy fix; if not, bind them.

The ask: up/down arrows behave exactly like j/k in every cursored context (main list, and any other list/scroll the TUI grows); footer copy becomes `↑/↓ move` or `↑/↓ (j/k) move`. esc/q semantics stay absolute (one meaning per key, every screen).

Acceptance: model-driven tests sending tea.KeyUp/tea.KeyDown alongside the existing j/k tests; footer assertions updated; make test lint green.

Pointers: cmd/tuhdoo/top.go Update() and footer(); top_test.go.

Constraints: boring Go; display/input only.

## History

### 2026-07-31 08:16 UTC — run by `brandon/claude-code-11` — done

Arrows were already bound (Update matches "j"/"down" and "k"/"up" in both the list and detail contexts), so this was the copy fix plus missing coverage. Commit ffb7bd6 on main: footer legends now read "↑/↓ (j/k) move …" / "↑/↓ (j/k) scroll …" in all three places (armed, watch-mode, detail); new model-driven tests TestTopArrowKeysMirrorJK (tea.KeyDown/KeyUp move the list cursor) and TestTopDetailArrowsScroll (arrows scroll the detail body and clamp at 0); existing footer assertions updated. make test lint green. esc/q untouched. Note: the historical mockup files under docs/design/mockups/ still show the old footer copy — they are design records, left as-is.
