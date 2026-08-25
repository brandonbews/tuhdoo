# TUI/CLI contrast: p0 bright red; laddered gray replaces SGR-2 dim (a mosh no-op)

`tuh-01M0XE2FZCW0HCPRV6KD413RYE`

- **Status:** done
- **Priority:** 1
- **Created:** 2026-08-25 21:43 UTC by `brandon`

## Description

## Context

The #82 priority-badge ramp styles p0 with ANSI-31 normal red — low contrast on dark themes — and p3+ with SGR-2 dim. Grilled 2026-08-25 and empirically confirmed in Brandon's daily setup: **mosh does not implement SGR-2 faint**, so every `col.dim` surface renders normal-weight there — the badge complaint was the visible tip of a TUI-wide gap. 256-indexed colors are the reliable path (same finding as the 2026-07-31 selection ladder: COLORTERM lies under mosh, TERM's 256color rung is trustworthy).

`col.dim` is load-bearing across ~30 call sites in both the TUI (`cmd/tuhdoo/top.go`: meta lines, id column, status words, placeholders, footer hints, held-row badges) and the CLI read commands (`cmd/tuhdoo/commands.go`: holder lines, status columns, timestamps, "none" placeholders).

## The ask

1. **p0 and negative priorities → bright red, ANSI 91** (`\x1b[91m`) in `priorityBadgeStyle`. Works on every rung including the 16-color floor; no new exception — brights are already in the palette (bgGray=100, bgWhite=107).
2. **Make `col.dim` capability-laddered inside `newColors`**: when TERM contains "256color", `col.dim` resolves to foreground gray index 245 (`\x1b[38;5;245m`); on the floor it stays SGR-2 byte-identical to today. This moves rung detection into `newColors` so all call sites — TUI and CLI — inherit the fix with zero call-site churn. Revise the `colors` comment in `render.go` (its third revision): the "never by newColors" boundary is explicitly retired; the 16-color law stays the baseline with the 256 rung as the sanctioned exception ladder.
3. **Composites are in scope, behavioral criterion**: every surface that is muted *by design* — `dimRed` (blocked "waiting:" lead), `bgRed` (BLOCKED bar), `bgGray` (shelf bar), `rev+dim` (detail-view section bars) — must visibly read as muted on the 256 rung. Pick the nearest 256-indexed equivalents (implementer's judgment) and pin them in golden tests. The blocked row's drop-the-alarm intent (2026-08-04 bar recolor) must survive under mosh: its replacement must read distinct from both `col.red` and plain text.

## Acceptance

- p0/negative badges render `\x1b[91m` in TUI ready rows on all rungs.
- On a 256color TERM, p3+ badges and every standalone `col.dim` surface render `38;5;245`, in both TUI and CLI read-command output; on the 16-color floor they render SGR-2 exactly as today.
- On the 256 rung, muted composites read visibly muted; dimRed's replacement is distinct from both `col.red` and plain text.
- `NO_COLOR` / non-TTY still yields the all-empty struct — zero escapes.
- COLORTERM is never consulted (the mosh finding stands).
- Golden tests updated to pin the above; `make test lint` green from the repo root.

## Pointers

- `cmd/tuhdoo/render.go` — `colors` struct + law comment, `newColors`, `orangeFG` (the existing rung-check precedent)
- `cmd/tuhdoo/top.go:1798` `priorityBadgeStyle`; `top.go:1850` held-row badge; `rev+dim` bars around `top.go:1116–1139`
- `cmd/tuhdoo/commands.go` — CLI dim call sites
- `cmd/tuhdoo/selection.go` — the capability-ladder precedent and mosh rationale
- `cmd/tuhdoo/top_golden_test.go` — where the escapes are pinned

## Constraints

- 16-color law remains the baseline; 256-indexed codes exist only on the rung, never faked on the floor.
- Rendering only — no stored-byte or event changes.
- One PR (repo conventions in CLAUDE.md).

## History

### 2026-08-25 22:05 UTC — run by `brandon/claude-code-1` — done

- Branch: `tuh-3rye/contrast-gray-ladder`
- PR: <https://github.com/brandonbews/tuhdoo/pull/85>
- Merged as: `0332cd691bee47c7eece5487e90770a1149667ab`

All acceptance criteria met; squash-merged to main as 0332cd6 (PR #85), make test lint green. p0/negative badges now ANSI 91 bright red on every rung via a new colors.brightRed field. Rung detection moved into newColors through a new pure termColors(term): on a TERM containing "256color", dim → 38;5;245, dimRed (waiting: lead) → 38;5;131, bgRed/bgGray keep themed backgrounds 41/100 and swap the faint foreground for gray 250; 16-color floor stays SGR-2 byte-identical (TestTermColorsLadder pins both). All ~30 col.dim call sites (TUI + CLI) inherited with zero churn; rev+dim detail bars resolve through the laddered dim. COLORTERM never consulted. Consequence of retiring the "never by newColors" boundary: orange moved into termColors, orangeFG deleted (behavior-neutral; selBG stays in runTUI — it needs the OSC 11 query); colors law comment carries its third revision. New golden tests: rungColors set, dashboard/task-view/CLI rung goldens, each with a noFaint sweep asserting zero SGR-2 bytes on the rung. Daemon redeploy (rebuild + restart) is happening immediately after this finish_run.
