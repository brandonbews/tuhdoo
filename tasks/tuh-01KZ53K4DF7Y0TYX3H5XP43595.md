# Pinned frame off-by-one: trailing newline clips the header row

`tuh-01KZ53K4DF7Y0TYX3H5XP43595`

- **Status:** done
- **Priority:** none
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

### 2026-08-04 00:47 UTC — run by `brandon/claude-code-2` — done

- Branch: `tuh-3595/pinned-frame-off-by-one`
- PR: <https://github.com/brandonbews/tuhdoo/pull/27>
- Commits: `ebf6e80ef01235ac868bf90850c2ad59b012e3e6`

Root cause confirmed and fixed: the chrome-hierarchy pinning (PR #26) made every frame height newlines, still newline-terminated; bubbletea splits the view on \n (trailing newline = extra empty line) and drops overflow from the TOP (standard_renderer.go:186-187, v1.3.10), clipping the header row on every render. Fix: pinned frames (height known) are exactly height split-lines with the footer/input-prompt hint as the last unterminated line — View and detailView trim the final newline; floating pre-WindowSizeMsg frames keep it; visibleChunks/detailWindow accounting and hit-test replay untouched. TestTopGoldenFooterPinned now asserts the renderer's real invariant (split count == height, header present on the top row at a full 12-row window) — its previous newline-count assertion had baked the bug in. Squash-merged to main as ebf6e80 (PR #27); make test lint green. Deploy restart follows this finish.
