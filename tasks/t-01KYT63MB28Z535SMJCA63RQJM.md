# t-01KYT63MB28Z535SMJCA63RQJM — Arm the TUI detail screen: selectable escalation, enter to answer; p/c on the viewed task

- Status: open — blocked on dependencies
- Priority: 1
- Labels: `cli`, `tui`
- Depends on: [t-01KYT63MB28Z535SMJC9B0D83W](t-01KYT63MB28Z535SMJC9B0D83W.md) (done), [t-01KYVD31CNTR1EVCDHPC5973KW](t-01KYVD31CNTR1EVCDHPC5973KW.md) (open)
- Created: 2026-07-30 18:55 UTC by `4099114+brandonbews`

## Description

Context: originally parked pending dogfooding friction; the friction is now felt (steering feedback, Brandon, 2026-07-30): opening a blocked task's detail gives no way to answer its escalation, forcing esc → navigate → answer round-trips. Unparked to p1 and respecced: the original a/p/c key idea is replaced for answering — the escalation should be a selectable element, because enter-on-the-thing is how the rest of the TUI now works (see the same-day Needs Input task: enter on a Needs Input row answers in place).

The ask: in the armed detail screen (modeDetail): the task's open escalation(s) render as focusable items; focus moves with arrows/j-k and enter opens answer input with the standard footer; submit behaves exactly like answering from the list (same API call, same refresh). p (reprioritize) and c (cancel — its display verb may become 'archive'; see the same-day vocabulary task 'Rename the cancel interaction' and coordinate if both are in flight) act on the viewed task with the same y/n confirm as the list. Watch mode: detail stays fully read-only — no focusable affordances, no input.

Acceptance: input modes work from modeDetail with correct footers; answering targets the escalation belonging to the viewed task; watch mode fully disarmed (tested); model-driven tests per top_test.go patterns; make test lint green.

Constraints: boring Go; display/input only — no event or API changes.

## History

_No activity yet._
