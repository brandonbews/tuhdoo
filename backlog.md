# Backlog

## Ready

| ID | Task | Priority | Labels |
|---|---|---:|---|
| [t-01KYRMFV10W1N28TCN62RR3A4D](tasks/t-01KYRMFV10W1N28TCN62RR3A4D.md) | Daemon portability: unix-only lock and the socket-path length limit | 0 | go, platform, parked |
| [t-01KYT63MB28Z535SMJCA63RQJM](tasks/t-01KYT63MB28Z535SMJCA63RQJM.md) | Arm the TUI detail screen (a/p/c on the viewed task) | 0 | cli, tui |
| [t-01KYT63MB28Z535SMJCBC7SY1P](tasks/t-01KYT63MB28Z535SMJCBC7SY1P.md) | Tree/parent-grouped rendering in the TUI list | 0 | cli, tui, design |

## In progress

| ID | Task | Priority | Claimed by |
|---|---|---:|---|
| [t-01KYRVCBE83KT62BAE11W3TAM8](tasks/t-01KYRVCBE83KT62BAE11W3TAM8.md) | Release pipeline: tagged, cross-compiled binaries | 1 | `4099114+brandonbews/claude-code-2` |
| [t-01KYTQTEQYGXVQ0QY7F95CHXVV](tasks/t-01KYTQTEQYGXVQ0QY7F95CHXVV.md) | Principal identity override: stop deriving ugly actors from noreply emails | 1 | `4099114+brandonbews/claude-code-2` |
| [t-01KYTQTEQYGXVQ0QY7FCTAZ3G6](tasks/t-01KYTQTEQYGXVQ0QY7FCTAZ3G6.md) | Agent protocol: dangling-pointer anti-pattern; notes are garnish, transitions are the record | 1 | `4099114+brandonbews/claude-code-2` |
| [t-01KYTSQDQJWM8YQQ8FWMHBZ5DW](tasks/t-01KYTSQDQJWM8YQQ8FWMHBZ5DW.md) | Short IDs are the human contract: display everywhere, accept as input, annotate edges | 1 | `4099114+brandonbews/claude-code-2` |

## Blocked / waiting

| ID | Task | Priority | Waiting on |
|---|---|---:|---|
| [t-01KYRMFV10W1N28TCN5NWAGSW5](tasks/t-01KYRMFV10W1N28TCN5NWAGSW5.md) | v0 definition of done: the dogfood week holds | 0 | escalation: v0 definition-of-done check: did the dogfood week hold? Answer on or after 2026-08-06. The bar (docs/plan/roadmap.md, v0): one week of tuhdoo managing its own development — sessions driven through claim_next, watch running beside agents — with zero manual repair of the data branch. Answer "yes" to release this milestone (then mark it done), or describe what broke. |
| [t-01KYRMFV10W1N28TCN5SH4QM7A](tasks/t-01KYRMFV10W1N28TCN5SH4QM7A.md) | v1 milestone: steering surface and a second machine | 0 | depends on [t-01KYRMFV10W1N28TCN5NWAGSW5](tasks/t-01KYRMFV10W1N28TCN5NWAGSW5.md) |
| [t-01KYRMFV10W1N28TCN5WVTCB1J](tasks/t-01KYRMFV10W1N28TCN5WVTCB1J.md) | Two-machine dogfood: real claim races over one remote | 2 | escalation: This task is human-paced — it needs you on a second machine running fleets for a week — so an agent can't execute it. When do you want to start the week, and do you want a prep task filed first for sync-latency instrumentation? |
| [t-01KYRMFV10W1N28TCN62F6FRTH](tasks/t-01KYRMFV10W1N28TCN62F6FRTH.md) | Epoch compaction (D9): snapshot event + in-commit deletion | 1 | depends on [t-01KYRMFV10W1N28TCN5SH4QM7A](tasks/t-01KYRMFV10W1N28TCN5SH4QM7A.md) |
| [t-01KYRVCBE83KT62BAE1502VV29](tasks/t-01KYRVCBE83KT62BAE1502VV29.md) | npm devDependency distribution (esbuild-pattern wrapper packages) | 1 | depends on [t-01KYRVCBE83KT62BAE11W3TAM8](tasks/t-01KYRVCBE83KT62BAE11W3TAM8.md) |

## Done

- [t-01KYRMFV10W1N28TCN5TDQC7KM](tasks/t-01KYRMFV10W1N28TCN5TDQC7KM.md) — Grow watch into the interactive steering TUI (tuhdoo top)
- [t-01KYRMFV10W1N28TCN5ZZ9Z2C1](tasks/t-01KYRMFV10W1N28TCN5ZZ9Z2C1.md) — Retire full-replay-per-write and the grow-forever event overlay
- [t-01KYRMVT1YC2929WSQ3W6YHHZM](tasks/t-01KYRMVT1YC2929WSQ3W6YHHZM.md) — Render markdown views on local daemon writes
- [t-01KYRR78YKX9YHZE6W6B798X4G](tasks/t-01KYRR78YKX9YHZE6W6B798X4G.md) — Auto-derive session principals: git identity + daemon-minted agent names
- [t-01KYT63MB28Z535SMJC9B0D83W](tasks/t-01KYT63MB28Z535SMJC9B0D83W.md) — One TUI: bare tuhdoo is the interactive surface (Cycle 4)
- [t-01KYT80CP4JKAM3V2C4DNGF1Y3](tasks/t-01KYT80CP4JKAM3V2C4DNGF1Y3.md) — TUI readability: short display IDs, width-aware wrapping, list scrolling
- [t-01KYTRJQT44HGYXGR9C7C3R2GS](tasks/t-01KYTRJQT44HGYXGR9C7C3R2GS.md) — Escalation ergonomics: resolve answered-out-of-band, and the name itself

## Cancelled

_None._
