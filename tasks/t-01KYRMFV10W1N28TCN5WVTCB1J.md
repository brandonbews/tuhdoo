# t-01KYRMFV10W1N28TCN5WVTCB1J — Two-machine dogfood: real claim races over one remote

- Status: open — ready
- Priority: 2
- Labels: `dogfood`, `multiplayer`
- Parents: [t-01KYRMFV10W1N28TCN5SH4QM7A](t-01KYRMFV10W1N28TCN5SH4QM7A.md)
- Depends on: [t-01KYRMFV10W1N28TCN5TDQC7KM](t-01KYRMFV10W1N28TCN5TDQC7KM.md) (done)
- Created: 2026-07-30 04:28 UTC by `brandon/migrator`

## Description

Context: roadmap v1 — the first real multi-machine usage. The D6 machinery (winner rule, voided claims, superseded runs) has only been proven in integration tests (B7), never across real machines. Depends on the TUI because the v1 proof includes answering an escalation from it mid-week.

The ask: run fleets on two machines against the same origin for a week. Log collision counts and sync latencies (T8's evidence-based tuning). Exercise at least one real claim race and confirm the loser's run lands superseded with a salvageable branch pointer.

Acceptance: a week of two-machine operation with zero divergent state (identical replayed state and branch tips after both sync); collision/latency numbers recorded onto this task as notes; any D6/T8 tuning changes proposed as design-doc revisions, never applied silently.

Constraints: no force-push on the data branch, ever; no manual repair — needing one is a bug to file and fix, and it resets the week.

## History

_No activity yet._
