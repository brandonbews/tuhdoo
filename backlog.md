# Backlog

## Ready

| ID | Task | Priority | Labels |
|---|---|---:|---|
| [t-01KYRMFV10W1N28TCN5ZZ9Z2C1](tasks/t-01KYRMFV10W1N28TCN5ZZ9Z2C1.md) | Retire full-replay-per-write and the grow-forever event overlay | 1 | go, performance |
| [t-01KYRMFV10W1N28TCN62RR3A4D](tasks/t-01KYRMFV10W1N28TCN62RR3A4D.md) | Daemon portability: unix-only lock and the socket-path length limit | 0 | go, platform, parked |

## In progress

| ID | Task | Priority | Claimed by |
|---|---|---:|---|
| [t-01KYRR78YKX9YHZE6W6B798X4G](tasks/t-01KYRR78YKX9YHZE6W6B798X4G.md) | Auto-derive session principals: git identity + daemon-minted agent names | 2 | `brandon/impl-2` |

## Blocked / waiting

| ID | Task | Priority | Waiting on |
|---|---|---:|---|
| [t-01KYRMFV10W1N28TCN5NWAGSW5](tasks/t-01KYRMFV10W1N28TCN5NWAGSW5.md) | v0 definition of done: the dogfood week holds | 0 | escalation: v0 definition-of-done check: did the dogfood week hold? Answer on or after 2026-08-06. The bar (docs/plan/roadmap.md, v0): one week of tuhdoo managing its own development — sessions driven through claim_next, watch running beside agents — with zero manual repair of the data branch. Answer "yes" to release this milestone (then mark it done), or describe what broke. |
| [t-01KYRMFV10W1N28TCN5SH4QM7A](tasks/t-01KYRMFV10W1N28TCN5SH4QM7A.md) | v1 milestone: steering surface and a second machine | 0 | depends on [t-01KYRMFV10W1N28TCN5NWAGSW5](tasks/t-01KYRMFV10W1N28TCN5NWAGSW5.md) |
| [t-01KYRMFV10W1N28TCN5WVTCB1J](tasks/t-01KYRMFV10W1N28TCN5WVTCB1J.md) | Two-machine dogfood: real claim races over one remote | 2 | escalation: This task is human-paced — it needs you on a second machine running fleets for a week — so an agent can't execute it. When do you want to start the week, and do you want a prep task filed first for sync-latency instrumentation? |
| [t-01KYRMFV10W1N28TCN62F6FRTH](tasks/t-01KYRMFV10W1N28TCN62F6FRTH.md) | Epoch compaction (D9): snapshot event + in-commit deletion | 1 | depends on [t-01KYRMFV10W1N28TCN5SH4QM7A](tasks/t-01KYRMFV10W1N28TCN5SH4QM7A.md) |

## Done

- [t-01KYRMFV10W1N28TCN5TDQC7KM](tasks/t-01KYRMFV10W1N28TCN5TDQC7KM.md) — Grow watch into the interactive steering TUI (tuhdoo top)
- [t-01KYRMVT1YC2929WSQ3W6YHHZM](tasks/t-01KYRMVT1YC2929WSQ3W6YHHZM.md) — Render markdown views on local daemon writes

## Cancelled

_None._
