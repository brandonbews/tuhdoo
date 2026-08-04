# Pinned frame off-by-one: trailing newline clips the header row

`tuh-01KZ53K4DF7Y0TYX3H5XP43595`

- **Status:** open — ready
- **Priority:** 0
- **Labels:** `tui` `go` `bug`
- **Created:** 2026-08-04 00:43 UTC by `brandon/claude-code-2`

## Description

Context: regression from the chrome-hierarchy task tuh-01KZ33YQPXPK59NV1VBWZ9A3V7 (PR #26, f95dacd), reported live by Brandon — the header bar vanished entirely from the TUI. Root cause diagnosed: the bottom-pinning made every frame exactly `height` newlines and the frame still ends with a trailing "\n". bubbletea's standard renderer splits the view on "\n" (a trailing newline yields an extra empty last element) and, when the view exceeds the terminal height, drops lines FROM THE TOP (standard_renderer.go:186-187 in bubbletea v1.3.10) — so the header row is clipped on every render. The off-by-one was latent pre-pinning: a completely full screen would have clipped the old filled header too, but real backlogs never filled the terminal.

The ask: a pinned frame (m.height > 0) must render exactly `height` split-lines with the footer (or live input prompt) as the last, UNTERMINATED line. In cmd/tuhdoo/top.go View: pad so head+body+foot totals m.height newlines (equivalently pad = m.height - strings.Count(head+body+foot, "\n")), then strings.TrimSuffix the final "\n". Same trim in detailView (its detailWindow reservation math is already consistent once the trailing newline goes). Floating case (height <= 0) keeps the trailing newline as always.

Acceptance:
- On all three screens (list, history, task view) at a known height H, View() splits on "\n" into exactly H elements and the LAST element carries the footer legend (or the live input prompt's hint line) — assert this shape in TestTopGoldenFooterPinned, replacing its wrong `newlines == height` assertion.
- Full-height content (body filling the window) also yields exactly H split-lines — no top clipping.
- Plain-80 goldens updated: same pad rows, no trailing newline on pinned frames.
- rowAt/detailStopAt hit-test replay unchanged.
- make test lint green.

Pointers: cmd/tuhdoo/top.go View (~1195), detailView (~1129), detailWindow (~1102), visibleChunks (~1634), joinChunks (~1676); cmd/tuhdoo/top_golden_test.go TestTopGoldenFooterPinned and the Plain80 goldens; renderer behavior at ~/go/pkg/mod/github.com/charmbracelet/bubbletea@v1.3.10/standard_renderer.go:186.

Constraints: boring Go (T1); do not change visibleChunks' window accounting (avail = height - head - foot is correct once the trailing newline is trimmed); one PR.

## History

_No activity yet._
