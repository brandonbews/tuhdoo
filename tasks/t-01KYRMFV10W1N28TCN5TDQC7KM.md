# t-01KYRMFV10W1N28TCN5TDQC7KM — Grow watch into the interactive steering TUI (tuhdoo top)

- Status: open — ready
- Priority: 3
- Labels: `go`, `tui`
- Parents: [t-01KYRMFV10W1N28TCN5SH4QM7A](t-01KYRMFV10W1N28TCN5SH4QM7A.md)
- Created: 2026-07-30 04:28 UTC by `brandon/migrator`

## Description

Context: 002 T7 / 001 D8 — v1's steering surface is a TUI grown from the read-only watch skeleton, shipped in the same binary as `tuhdoo top`. watch was built deliberately as the TUI's skeleton with interactivity amputated.

The ask: add input handling to the watch view loop: answer an escalation, reprioritize the queue, cancel/archive a task. Human/admin verbs already exist on the daemon HTTP API; the TUI speaks that API — no new write paths.

Acceptance: a blocking escalation raised by an agent can be answered from the TUI and the task returns to the ready pool; a reprioritize is visible to a subsequent get_backlog; cancel/archive lands with the acting human principal. Tests follow the project pattern: pure state->render functions with table-driven tests; interaction logic covered without a live terminal where possible. make test lint green.

Pointers: cmd/tuhdoo/watch.go (the skeleton), cmd/tuhdoo/render.go, internal/daemon/api.go (admin verbs), internal/daemon/ops.go.

Constraints: boring Go (T1); TUI writes go through the daemon HTTP API only — never git, never the ops layer directly; the agent MCP surface stays at ten verbs (T5).

## History

_No activity yet._
