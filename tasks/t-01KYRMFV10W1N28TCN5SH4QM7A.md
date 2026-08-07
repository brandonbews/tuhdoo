# v1 milestone: steering surface and a second machine

`t-01KYRMFV10W1N28TCN5SH4QM7A`

- **Status:** on hold — deliberately paused
- **Priority:** 0
- **Labels:** `milestone`
- **Depends on:** [`t-gsw5`](t-01KYRMFV10W1N28TCN5NWAGSW5.md) (done), [`tuh-pk06`](tuh-01KZA0VT234XJYVZWT8S09PK06.md) (done)
- **Created:** 2026-07-30 04:28 UTC by `brandon/migrator`

## Description

Context: internal-docs/plan/roadmap.md v1. A milestone is a label, not a mechanism (001 D5, revised by the milestone grill 2026-08-03): its done-ness is declared by a human, never computed. This task is a container and a gate, not a unit of work — the v1 children carry parent edges into it (the steering TUI, done; the two-machine convergence harness, ready), and Epoch compaction (D9) depends on it, so declaring this done is what unparks compaction. It stays `held` until the evidence below exists; parked work is held, never fenced with a blocking escalation.

Definition of done (human-verified — mirrors internal-docs/plan/roadmap.md v1, which is authoritative if the two ever disagree):

1. a blocking escalation is raised by an agent, answered from the TUI, and picked up by a successor agent without the human touching git;
2. two daemons against one remote produce observed claim collisions with one winner each, a `superseded` run recorded for every loser, and byte-identical replayed state and views on both sides afterward;
3. Brandon's 5-person work team could be onboarded with `tuhdoo init` + docs alone (whether or not they are).

Clause 3 is a judgment call on purpose; mechanizing it would swap in a proxy.

Constraints: agents must not work this task. If you are an agent holding this claim, release it with reason "human-declared milestone".

## History

_No activity yet._
