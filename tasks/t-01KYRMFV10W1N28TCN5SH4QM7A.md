# v1 milestone: steering surface and a second machine

`t-01KYRMFV10W1N28TCN5SH4QM7A`

- **Status:** open — blocked on dependencies
- **Priority:** 0
- **Labels:** `milestone`
- **Depends on:** [`t-gsw5`](t-01KYRMFV10W1N28TCN5NWAGSW5.md) (open)
- **Created:** 2026-07-30 04:28 UTC by `brandon/migrator`

## Description

Context: docs/plan/roadmap.md v1. Milestones are just tasks other tasks point into (001 D5); the TUI and two-machine tasks carry parent edges into this one. Readiness gates on the v0 milestone only (the batch cycle check treats parent + dependency edges as one graph, so a milestone cannot also depend on its own children); completion is judged by a human against the children, not by this task becoming ready.

Definition of done (human-verified): a blocking escalation raised by an agent is answered from the TUI and picked up by a successor agent without the human touching git; two machines run fleets against the same remote for a week with collision counts logged and zero divergent state; a 5-person team could be onboarded with tuhdoo init + docs alone (whether or not they are).

## History

_No activity yet._
