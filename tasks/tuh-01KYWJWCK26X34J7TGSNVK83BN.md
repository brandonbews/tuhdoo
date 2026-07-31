# tuh-01KYWJWCK26X34J7TGSNVK83BN — Needs Input is the single home for escalation-blocked tasks (task-shaped 3-line rows)

- Status: open — ready
- Priority: 0
- Labels: `cli`, `tui`
- Created: 2026-07-31 17:17 UTC by `brandon`

## Description

Context: In `tuhdoo top`, a task with an open blocking escalation renders twice: its question under NEEDS INPUT, and a task row under BLOCKED reading "waiting: needs input (above)". Steering feedback (2026-07-31): visually confusing. Grill cycle (2026-07-31) decided: the NEEDS INPUT row becomes the single representation, and it becomes task-shaped for consistency with the other sections.

The ask:
1. NEEDS INPUT rows become three lines on the shared grid:
   - line 1 — like a task row: short task ID, red `!` badge when blocking (empty badge otherwise), task title, dim labels/edges suffix.
   - line 2 — `question: ` lead in magenta (add a foreground-magenta code to `colors`; `bgMagenta` is the bar style), then the question, one line, ellipsized.
   - line 3 — dim meta: actor · raised stamp. The word "blocking" disappears; the `!` badge carries it.
2. BLOCKED stops listing tasks whose only blocker is an open blocking escalation (their NEEDS INPUT row is the representation). A task with unmet deps AND a blocking escalation keeps its BLOCKED row, but its `waiting:` line names only the unmet deps — the "needs input (above)" phrase dies.
3. Non-blocking escalations unchanged in semantics: the task still rows under READY/IN PROGRESS; the title appearing in both places is accepted (the rows state different truths).
4. TUI only. One-shot `backlog`/`escalations` output untouched; their Blocked count may now differ from the TUI's — accepted.

Acceptance:
- TUI/golden tests cover: (a) escalation-only-blocked task — 3-line NEEDS INPUT row (title line, question: line, meta line), absent from BLOCKED; (b) deps+escalation task — rows in both sections, BLOCKED `waiting:` line naming only the dep; (c) non-blocking escalation — no `!` badge, its task still rowed under its status section; (d) BLOCKED bar count equals its rendered rows.
- Enter-to-answer on a NEEDS INPUT row and cursor behavior unchanged.
- `make test lint` green from the repo root.

Pointers: cmd/tuhdoo/top.go (buildRows, rowChunk, topSections), cmd/tuhdoo/snapshot.go (blockedReasonTUI), cmd/tuhdoo/render.go (colors), cmd/tuhdoo/top_golden_test.go.

Constraints: one-shot commands and internal/views untouched; 16-color ANSI only; NO_COLOR/non-TTY degradation intact.

## History

_No activity yet._
