# t-01KYRMFV10W1N28TCN5WVTCB1J — Two-machine dogfood: real claim races over one remote

- Status: open — waiting on an escalation answer
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

### 2026-07-30 05:51 UTC — escalation from `brandon/impl-2` (blocking)

**Q:** This task is human-paced — it needs you on a second machine running fleets for a week — so an agent can't execute it. When do you want to start the week, and do you want a prep task filed first for sync-latency instrumentation?

Two things, one blocking and one a finding:

1. The task became ready when the TUI landed (fa8c7d3), but its substance — a week of two-machine operation against one origin, a real claim race, answering an escalation from `tuhdoo top` mid-week — is operational work only you can start and pace. This escalation fences it out of the ready pool so agents stop claim-churning it until you kick the week off. Answer when you're ready to begin (or tell me how you'd rather fence human-paced tasks — this is the same workaround B12 used for the milestone, per open-questions Cycle 3).

2. Acceptance requires "collision/latency numbers recorded onto this task as notes", and T8 says the daemon logs collision counts *and sync latencies* — but internal/syncer only counts collisions (Status.Collisions, syncer.go:37); nothing measures or logs fetch/push latency. Options: (a) I file a well-formed prep task to add sync-latency measurement/logging before the week starts (recommended — the week's evidence is half-blind without it); (b) run the week with collisions-only and eyeball latencies from timestamps; (c) you scope it differently. I deliberately did not create the prep task or wire a depends_on edge myself: making this task depend on a new child is exactly the parent/depends_on union-cycle territory open-questions Cycle 3 flags as unsettled.

_Unanswered._

### 2026-07-30 05:51 UTC — note from `brandon/impl-2`

Resume state: no work done or possible yet — the week hasn't started. What the answer unblocks: if Brandon picks option (a) in escalation 01KYRS7H9003HS5H7W8QKW7KG8, the next claimant's first move is to create the sync-latency prep task (measure fetch/push wall time in internal/syncer around the cycle's fetch and push calls, expose it on Status / /v0/state syncJSON, log it per T8) — as a sibling task, not a depends_on edge, unless Brandon has settled the Cycle-3 edge question by then. The week itself is Brandon-driven; agents only participate once fleets are running on both machines.

### 2026-07-30 05:51 UTC — run by `brandon/impl-2` — blocked

Blocked on escalation 01KYRS7H9003HS5H7W8QKW7KG8: the week is human-paced (Brandon must run the second machine), and a sync-latency instrumentation gap vs T8 needs his call before the week's evidence collection starts.
