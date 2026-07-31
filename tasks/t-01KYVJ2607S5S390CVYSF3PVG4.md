# t-01KYVJ2607S5S390CVYSF3PVG4 — TUI dashboard visual redesign: section bars + fixed column grid (mock-a)

- Status: open — ready
- Priority: 1
- Labels: `cli`, `tui`, `design`
- Created: 2026-07-31 07:43 UTC by `brandon/claude-code-6`

## Description

**Context:** The dashboard list (`tuhdoo` / `tuhdoo -w`) has almost no visual
hierarchy: bold section headers, wrapped rows, blank-line spacing. A design
pass (2026-07-31) produced an approved mockup — variant "mock-a" in
`docs/design/mockups/tui-dashboard/` (`cat mock-a.ansi` in a terminal to see
it; `mock-a.txt` is the colorless geometry; `mockups.py` regenerates both;
`current.*` shows today's render for contrast). This ticket reproduces that
mockup in the real TUI. Display-only: no daemon, API, or protocol changes.

**The ask — two structural moves and their consequences:**

1. **Full-width bars divide the screen.** Header bar (reverse video, bold):
   left ` tuhdoo · <syncLine>`, right `acting as <actor> ` (or the watch-mode
   badge). One bar per section, black text on 16-color ANSI background:
   NEEDS INPUT on magenta (45), READY on green (42), IN PROGRESS on
   yellow (43), BLOCKED on red (41). Bar left edge: ` SECTION (count) `.
   Bar right edge: that section's steering keys (`a answer` on Needs Input;
   `p priority · c cancel` on Ready) — omitted in watch mode. Footer bar
   (reverse + dim): full key legend left, `<n> done` tally right. Bars span
   the full terminal width. The old summary-counts line is removed — the
   bars carry the counts now.
2. **Every row shares one column grid.** mark(2) + short id(6) + gap(2) +
   badge(2) + gap(2): titles start at column 14. Badge column: `p<n>` for
   ready tasks (dim; p0 rendered yellow), red bold `!` for blocking
   escalations, blank otherwise. Escalation rows show their task's short id
   in the id column. Second lines (escalation meta, blocked `waiting:`)
   indent to the title column. **List rows never wrap** — one line per row
   (two for escalation/blocked), overflow ellipsized with `…`; full text
   stays on the detail screen. Labels render as a dim suffix only when they
   fit; the title wins. Row spacing: single-spaced (the blank-line breathing
   room existed because rows wrapped).

**Acceptance:**
- Golden render tests (table-driven, fixed injected width/height per T1)
  covering: bar composition at 80 and 120 columns, the shared column grid
  across all four sections, ellipsis truncation, label drop-when-tight,
  cursor row, watch mode (no steering hints on bars), and the detail screen
  unchanged.
- `NO_COLOR` / non-TTY: bars degrade to their plain text (no fill), layout
  intact — existing `colors{}` zero-value discipline extends to backgrounds.
- Cursor-following windowing (`windowList`) still holds with the new line
  economy; escalation/blocked two-line rows never split across the window
  edge.
- `make test lint` green from the repo root.

**Pointers:** `cmd/tuhdoo/render.go` (colors struct, shared sections),
`cmd/tuhdoo/top.go` (View, renderTopRows, renderTopRow, windowList, footer),
`docs/design/mockups/tui-dashboard/` (the spec), `docs/design/002-technology.md`
T7. Consider promoting `charmbracelet/lipgloss` (already an indirect dep) to
direct for style definitions — one named-styles block, not scattered escapes.

**Constraints:**
- 16-color ANSI only; no truecolor, no 256-color (user themes must survive).
- One-shot CLI commands (`backlog`, `status`, `escalations`) keep their
  current plain rendering in this ticket; only the interactive TUI changes.
- The mockup says `c archive` — that rename belongs to its own open task
  (t-…q5ev). Render `c cancel` until that task lands; do not fold the
  rename into this ticket.
- Section list will grow (inbox / on-hold statuses are being designed in
  parallel): build the bar+section rendering over a slice of section
  descriptors so a fifth section is data, not new code. Do not implement
  the new statuses here.
- Boring Go (T1): string building and plain loops; no layout framework
  beyond what exists.

## History

_No activity yet._
