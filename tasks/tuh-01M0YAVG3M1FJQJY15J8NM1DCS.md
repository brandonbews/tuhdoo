# TUI: ramp badges everywhere; dashboard bars go black-on-color (BLOCKED bright red, ON HOLD gray); quiet chrome unified

`tuh-01M0YAVG3M1FJQJY15J8NM1DCS`

- **Status:** open — in progress, claimed by `brandon/claude-code-1`
- **Priority:** 1
- **Labels:** `tui` `go`
- **Created:** 2026-08-26 06:06 UTC by `brandon`

## Description

## Context

Brandon's live review of PR #85 (2026-08-25): priority colors only reach READY — badges render only in ready + on-hold, and on-hold deliberately gray (2026-08-21 "shelf rows are dim by design") — and the gray-250 foreground the contrast ramp gave bgGray made the ON HOLD bar read wrong on his theme. Steering answers (structured questions + follow-up, 2026-08-25): badges everywhere; the dashboard's bars become uniformly black-text-on-color; the task-view bars and CANCELLED take the quiet white-on-bright-black chrome.

## The ask

1. **Ramp-colored priority badges in every section** where a task row renders: ready, in progress, blocked, on hold, inbox, and done/cancelled history rows. Badge only when priority is set; unprioritized rows stay bare. NEEDS INPUT rows keep the red ! (blocking outranks priority in that cell). Reverses the 2026-08-21 held-dim and no-badge-in-inbox/blocked/history consequences — revise the comments.
2. **BLOCKED bar: black on bright red** — `\x1b[30;101m`, the background twin of the p0 badge's 91, same dark text as READY/IN PROGRESS. Reverses the 2026-08-04 drop-the-alarm muted bar (Brandon's explicit steer); the bgRed ladder entry is retired. The blocked row's muted-red waiting: lead stays (a reason line, not an alarm) — revise its stale "quieter than the bar" comment.
3. **ON HOLD bar: black on gray** — `\x1b[30;47m` (palette slot 7, bright enough for black text; distinct from INBOX's bright-white 107). Every dashboard bar is now black-on-color. The bgGray ladder entry is retired.
4. **Quiet chrome unified on white-on-bright-black** — a new field (e.g. bgDarkGray) carrying plain `\x1b[100m` on every rung: the CANCELLED history bar and the task-view section bars (DEPENDS ON, NEEDED BY, DESCRIPTION, HISTORY — switched from rev+dim; revises the 2026-08-03 "neutral structural chrome, not shelf" distinction). ON HOLD does NOT take this style. If col.rev goes unused, remove it.
5. termColors rung overrides shrink to dim (245), dimRed (131), orange (208).

## Acceptance

- Golden tests pin: ramp badge styles per section (incl. held p0 bright red, in-progress/blocked/inbox/history badges); BLOCKED `30;101` and ON HOLD `30;47` on floor and rung; CANCELLED + task-view bars `\x1b[100m` on floor and rung; the rung noFaint sweeps still pass.
- make test lint green from the repo root.

## Pointers

- cmd/tuhdoo/render.go — colors law comment (amend with a dated revision note), termColors
- cmd/tuhdoo/top.go rowChunk + priorityBadgeStyle doc; barLine call sites ~1116/1131/1139; history-view CANCELLED bar; blocked waiting: comment
- cmd/tuhdoo/top_golden_test.go — ansiColors/rungColors literals, bar and ramp goldens

## Constraints

- Rendering only — no stored-byte or event changes. 16-color law: all new bar codes are in-palette (101, 47, 100); indexed codes remain rung-only. One PR.

## History

### 2026-08-26 06:09 UTC — edit by `brandon`

retitled · description edited

### 2026-08-26 06:09 UTC — run by `brandon/claude-code-1` — interrupted

lease expired without a finish or release

_Synthesized by replay, not recorded by the agent._

### 2026-08-26 06:25 UTC — run by `brandon/claude-code-1` — interrupted

lease expired without a finish or release

_Synthesized by replay, not recorded by the agent._
