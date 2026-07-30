# t-01KYRMFV10W1N28TCN5ZZ9Z2C1 — Retire full-replay-per-write and the grow-forever event overlay

- Status: open — ready
- Priority: 1
- Labels: `go`, `performance`
- Created: 2026-07-30 04:28 UTC by `brandon/migrator`

## Description

Context: the flagged optimization point from the build-out (sessions 1-4): the daemon replays the full event set on every write and holds a grow-forever in-memory event overlay. Correct and acceptable at dogfood scale; the wrong shape eventually.

The ask: measure first. First deliverable is evidence from a real data branch (replay timings and event counts from daemon logs), not a rewrite. Only if the numbers show real pain: incremental state maintenance or snapshot-bounded replay windows.

Acceptance: before/after numbers on a real data branch; the deterministic core stays pure functions with table-driven tests (T1); all existing convergence and race tests stay green.

Constraints: correctness over speed; no clever concurrency (T1); stored event bytes are never rewritten (T3).

## History

_No activity yet._
