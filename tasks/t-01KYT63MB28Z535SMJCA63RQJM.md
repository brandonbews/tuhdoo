# t-01KYT63MB28Z535SMJCA63RQJM — Arm the TUI detail screen (a/p/c on the viewed task)

- Status: open — ready
- Priority: 0
- Labels: `cli`, `tui`
- Depends on: [t-01KYT63MB28Z535SMJC9B0D83W](t-01KYT63MB28Z535SMJC9B0D83W.md) (done)
- Created: 2026-07-30 18:55 UTC by `4099114+brandonbews`

## Description

Follow-up from Cycle 4, gated on dogfooding friction: the in-place detail screen ships read-only; if esc-act-enter round-trips prove annoying, add steering keys inside detail. Target semantics: a answers the viewed task's open blocking escalation, p reprioritizes, c cancels (same y/n confirm). Only claim this if the friction has actually been felt and noted — otherwise leave it.

Acceptance: input modes work from modeDetail with correct footers; watch mode still fully disarmed; model-driven tests per top_test.go patterns; make test lint green.

## History

_No activity yet._
