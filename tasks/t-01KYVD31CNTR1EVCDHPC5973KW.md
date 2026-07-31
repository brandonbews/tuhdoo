# t-01KYVD31CNTR1EVCDHPC5973KW — Needs Input: enter answers in place; blocked rows stop repeating the question

- Status: open — ready
- Priority: 1
- Labels: `cli`, `tui`, `ux`
- Depends on: [t-01KYVD31CNTR1EVCDHPGZFQ5EV](t-01KYVD31CNTR1EVCDHPGZFQ5EV.md) (done)
- Created: 2026-07-31 06:16 UTC by `brandon/claude-code-2`

## Description

Context: dogfood steering feedback (Brandon, 2026-07-30): an open escalation's question renders twice on the main screen — once as its Needs Input row and again verbatim in the owning task's blocked-row `waiting:` reason. And enter on a Needs Input row opens the task detail, while the natural expectation is that enter starts answering; `a` as the answer key is unintuitive.

The ask:
1. Needs Input rows keep showing the question (current one-line summary behavior is fine).
2. Enter with the cursor on a Needs Input row goes straight into answer input for that escalation (the input mode `a` uses today), with the matching footer. Decide whether `a` survives as an alias or dies — one documented behavior, no vestigial keys (the Cycle-4 rule: vestigial names outlive their excuse).
3. Blocked rows stop repeating the question: the `waiting:` reason names the escalation without duplicating its text — e.g. `waiting: needs input (above)` — dependency reasons unchanged.
4. Watch mode (--watch) stays fully disarmed: enter on a Needs Input row must not open input; opening read-only detail is acceptable.

Acceptance: model-driven tests per top_test.go patterns: enter on an escalation row enters answer mode with the right footer and submits to the right escalation; blocked-row rendering no longer contains the question text; a watch-mode test proves no input opens. If the blocked-reason copy change is applied to one-shot output too, update the exact-format CLI tests deliberately and say so in the run summary — never leak TUI sugar into one-shot accidentally. make test lint green.

Pointers: cmd/tuhdoo/top.go (input modes, submit(), footer()), cmd/tuhdoo/snapshot.go (blockedReason/blockedReasonDisp), top_test.go, top_cli_test.go.

Constraints: boring Go; display/input only — no event or API changes.

## History

_No activity yet._
