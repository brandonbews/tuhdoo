# TUI: ramp badges in every section; gray bars go standard white (bgGray unified, task-view bars switch)

`tuh-01M0YAVG3M1FJQJY15J8NM1DCS`

- **Status:** open — ready
- **Priority:** 1
- **Labels:** `tui` `go`
- **Created:** 2026-08-26 06:06 UTC by `brandon`

## Description

## Context

Brandon's live review of PR #85 (2026-08-25): priority colors only reach READY — badges render only in ready + on-hold, and on-hold deliberately gray (2026-08-21 "shelf rows are dim by design") — and the gray-250 foreground the contrast ramp gave bgGray made the ON HOLD bar read wrong on his theme; the mosh-familiar rendering was default (white) text on bright-black. Steering answers (structured questions, 2026-08-25): badges everywhere, bars uniform.

## The ask

1. Ramp-colored priority badges render in **every section** where a task row renders: ready, in progress, blocked, on hold, inbox, and done/cancelled history rows. Badge only when priority is set; unprioritized rows stay bare. NEEDS INPUT rows keep the red ! (blocking outranks priority in that cell). This reverses the 2026-08-21 held-dim and no-badge-in-inbox/blocked/history consequences — revise the comments.
2. **bgGray becomes plain \x1b[100m** (default foreground on bright-black) on every rung — the bgGray ladder entry is retired; floor and rung identical, pure 16-color.
3. **Task-view section bars** (DEPENDS ON, NEEDED BY, DESCRIPTION, HISTORY) switch from rev+dim to bgGray — unified with ON HOLD/CANCELLED shelf chrome (revises the 2026-08-03 "neutral structural chrome, not shelf" distinction). If col.rev goes unused, remove it.
4. BLOCKED bar keeps muted gray-250-on-red (2026-08-04 drop-the-alarm intent stands; Brandon did not override).

## Acceptance

- Golden tests pin: ramp badge styles per section (incl. held p0 bright red, history/inbox badges); bgGray bytes \x1b[100m on floor and rung; task-view bars bgGray; CANCELLED inherits; the rung noFaint sweeps still pass.
- make test lint green from the repo root.

## Pointers

- cmd/tuhdoo/render.go — colors law comment (amend the third revision), termColors
- cmd/tuhdoo/top.go rowChunk + priorityBadgeStyle doc; barLine call sites ~1116/1131/1139
- cmd/tuhdoo/top_golden_test.go — ansiColors/rungColors literals, bar and ramp goldens

## Constraints

- Rendering only — no stored-byte or event changes. One PR.

## History

_No activity yet._
