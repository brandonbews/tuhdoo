# tuh-01KYXDWWM8S1GF6N9NE5FGA86Y — Task view is the answering home: routed from Needs Input, dash-aligned design, selectable escalations, a to archive

- Status: open — ready
- Priority: 2
- Labels: `cli`, `tui`, `ux`, `design`
- Created: 2026-08-01 01:09 UTC by `brandon`

## Description

## Context

Three dogfooding captures from Brandon (2026-08-01) converge on the task view:

1. Enter on a Needs Input row opens an answer prompt at the bottom of the dashboard, so you answer without seeing the task's context. That in-place prompt was deliberately built by t-01KYVD31CNTR1EVCDHPC5973KW — **this task supersedes that decision**; dogfooding showed answering needs context.
2. The task view's visual design lags the dashboard redesign (t-01KYVJ2607S5S390CVYSF3PVG4, tuh-01KYWJRW0DD4CYGH29EZ151DCT): plain field names, no section bars, no visual hierarchy.
3. Answering and archiving from the task view are clunky: escalations aren't first-class selectable things, and archive is bound to `c`, which reads as "cancel"/"close", not archive.

## The ask

Rework the task view into the place where steering happens:

- **Routing**: enter on a Needs Input row in the dashboard opens the task view for that task (blocking escalation section visible and preselected). Remove the dashboard's inline answer prompt for Needs Input rows.
- **Visual alignment with the dashboard**: bold field names in the header block, the dashboard's section-bar headings, and a clearly delineated escalations section.
- **Selectable escalations**: every open escalation on the task renders as a selectable row using the same selection UI as the dashboard (gutter bar / gray tint, arrow keys, click). Enter on the selected question opens answer entry; submitting records the answer through the existing plumbing. Multiple open escalations per task must work.
- **Archive is `a`**: rebind archive from `c` to `a` on both the dashboard and the task view; update footer hints. Free `c`.

## Acceptance

- Enter on a Needs Input row lands in the task view with the escalation preselected; the old bottom-of-dashboard answer flow is gone (tests updated, not deleted-and-forgotten).
- Task view shows bold field labels, dashboard-style section bars, and an escalations section listing all open escalations as selectable rows; arrows/click move selection; enter answers; the view re-renders after an answer.
- A task with two open escalations can have each answered from the task view.
- `a` archives from both screens; footers say so; `c` no longer archives.
- Golden tests (cmd/tuhdoo/top_golden_test.go) updated; behavioral tests cover routing and multi-escalation selection. make test lint green.

## Pointers

- cmd/tuhdoo/top.go (key handling ~lines 260-480: dashboard enter at ~280, archive `c` at ~288/476, task-view keys ~447+), cmd/tuhdoo/render.go.
- Prior art: dashboard selection UI (tuh-01KYWJRW0DD4CYGH29EZ151DCT), armed detail screen (t-01KYT63MB28Z535SMJCA63RQJM).

## Constraints

- Boring Go; TUI-only — no daemon, event-schema, or MCP changes.
- Merged from inbox captures tuh-01KYXDWWM8S1GF6N9NE5FGA86Y, tuh-01KYXDZ2Y0MX2YJBX94TVF1NCE, tuh-01KYXE2S7A1WD7RKZSA0TNP09A.

## History

_No activity yet._
