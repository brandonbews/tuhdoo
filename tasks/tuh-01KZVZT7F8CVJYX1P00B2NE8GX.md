# Selection-bar capability ladder is designed but unrecorded in internal-docs

`tuh-01KZVZT7F8CVJYX1P00B2NE8GX`

- **Status:** inbox — untriaged capture
- **Priority:** none
- **Labels:** `audit-finding`
- **Created:** 2026-08-12 21:59 UTC by `brandon/claude-code-bg`

## Description

Go-sweep audit finding. The TUI selection-bar capability ladder (answered-query-only truecolor unlock, deliberate COLORTERM distrust from the mosh finding, 256-indexed gray rung, bright-black floor, the selBG 16-color-law exception) is dated deliberate design pinned by TestSelectionBGLadder — but recorded only in code comments (cmd/tuhdoo/selection.go header, render.go ~15-27), not in 002 T7, which records the adjacent mouse and bar-recolor decisions. If every dated decision should be findable in internal-docs, add it with a revision note.

Triage note 2026-08-27 (re-verified): still unrecorded, and the inventory DOUBLED since the audit. PR #82 put fragments into 002:156 (selBG named as the first 16-color-law exception, COLORTERM distrust, a passing reference to the ladder's goldens), but the ladder's actual shape — the answered-OSC-11-only truecolor unlock, the 256-indexed rung values, the bright-black floor — lives only in cmd/tuhdoo/selection.go:3-34 and render.go:12-52, the latter now carrying four dated 16-color-law revisions (2026-07-31 selBG, 2026-08-21 orange, 2026-08-25 contrast ramp, 2026-08-26 theme-derived dim). PR #88 added a second parallel ladder: chromeBG (selection.go:47-72, TestChromeBGLadder).

Deliberately kept in inbox rather than promoted or folded into the 002 drift sweep (tuh-01KZVZT7F8CVJYX1P009AJJ4D9): whether every dated decision must be findable in internal-docs — vs code comments + goldens being a legitimate tier of record — is exactly the internal-docs tiering grill (tuh-01M10ZA2VCJ59WWYZG58RXHV8A), which names this capture. This capture is the concrete inventory for whichever tier that grill picks; do not promote or build ahead of it.

## History

### 2026-08-27 07:06 UTC — edit by `brandon/claude-code-1`

description edited
