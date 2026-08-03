# Two-machine convergence: a deliberate claim-collision harness

`t-01KYRMFV10W1N28TCN5WVTCB1J`

- **Status:** open — waiting on an escalation answer
- **Priority:** 2
- **Labels:** `dogfood` `multiplayer`
- **Parents:** [`t-qm7a`](t-01KYRMFV10W1N28TCN5SH4QM7A.md)
- **Depends on:** [`t-c7km`](t-01KYRMFV10W1N28TCN5TDQC7KM.md) (done)
- **Created:** 2026-07-30 04:28 UTC by `brandon/migrator`

## Description

## Context

The v1 definition of done (docs/plan/roadmap.md, clause 2, rewritten 2026-08-03 by the milestone grill) asks for proof that cross-machine convergence holds. Its original form — "two machines run fleets against the same remote for a week" — was retired: a calendar proxy no agent could execute, which would also have measured the wrong number.

The hole this closes is concrete. As of 2026-08-03 the data branch carries 369 commits and **zero merge commits**. The D3 set-union merge path — divergent histories merged by application logic, two-parent commit via `commit-tree`, deterministic view regeneration — has never executed outside unit tests, because a single machine never diverges from itself.

Measurement caveat found during the grill: `syncer.Status.Collisions` counts non-fast-forward **pushes** (push contention), not claims voided by the D6 winner rule. Nothing anywhere counts voided claims. Do not report push collisions as claim collisions; derive claim races from replayed state (`core.ClaimVoided`).

## The ask

Build a deliberate collision harness and run it.

Two independent daemons against one remote. **Two clones on one box is faithful** — `machineID` is minted per repo directory (internal/daemon/daemon.go:526), so the clones get distinct machine ids, and ULID ordering never trusts wall clocks (T3). No second physical machine is required.

Drive many claim races in minutes rather than waiting for incidental ones: seed a scratch repo with tasks that complete in seconds, then have two scripted actors fire `claim_next` in a synchronized loop. The claim lifecycle is session-only (T7) — a scripted actor must hold an MCP session through `tuhdoo mcp`; there is deliberately no one-shot `tuhdoo claim`.

Hammer the sync loop specifically while you are there: both daemons push eagerly on every claim (T8), and the push cycle retries fetch/merge/push on non-fast-forward with a bounded `maxCycleRetries` before erroring "remote kept moving". That bound has never met real contention.

## Acceptance

- Observed claim collisions > 0, each with exactly one winner by the D6 rule (earliest ULID, machine-id tiebreak) and a `superseded` run recorded for every loser, carrying the loser's branch.
- Byte-identical replayed state on both clones afterward, and byte-identical generated views.
- Identical event sets on both sides (set-union merge converged), and at least one real merge commit on the data branch.
- The push-retry loop never exhausts `maxCycleRetries` under the harness's contention — or, if it does, that is reported as a finding with the rate.
- Numbers recorded as notes on this task: claim races observed, voided claims, non-fast-forward pushes, merge commits produced.
- `make test lint` green from repo root if any code lands.

## Pointers

- `internal/core/state.go` — `Ready`, `ClaimBlockers`
- `internal/core/replay.go` — D6 winner rule (~line 271), `ClaimVoided`
- `internal/syncer/syncer.go` — push cycle, `bumpCollisions`, `maxCycleRetries`
- `internal/daemon/daemon.go:526` — `machineID`
- `docs/design/002-technology.md` T2 (merges are application logic), T8 (cadence defaults); `001` D6 (claim semantics)

## Constraints

- Run the harness against a **scratch repo**, never this repo's data branch. A collision experiment must not be able to damage the live ledger.
- No force-push on any data branch, ever (project law).
- If instrumentation is needed to count voided claims, add it as a derived read over replayed state — never add a clock or I/O to `internal/core` (T1).
- Host-agnostic: no host API calls (T2).

## History

### 2026-07-30 05:51 UTC — escalation from `brandon/impl-2` (blocking)

> This task is human-paced — it needs you on a second machine running fleets for a week — so an agent can't execute it. When do you want to start the week, and do you want a prep task filed first for sync-latency instrumentation?

Two things, one blocking and one a finding:

1. The task became ready when the TUI landed (fa8c7d3), but its substance — a week of two-machine operation against one origin, a real claim race, answering an escalation from `tuhdoo top` mid-week — is operational work only you can start and pace. This escalation fences it out of the ready pool so agents stop claim-churning it until you kick the week off. Answer when you're ready to begin (or tell me how you'd rather fence human-paced tasks — this is the same workaround B12 used for the milestone, per open-questions Cycle 3).

2. Acceptance requires "collision/latency numbers recorded onto this task as notes", and T8 says the daemon logs collision counts *and sync latencies* — but internal/syncer only counts collisions (Status.Collisions, syncer.go:37); nothing measures or logs fetch/push latency. Options: (a) I file a well-formed prep task to add sync-latency measurement/logging before the week starts (recommended — the week's evidence is half-blind without it); (b) run the week with collisions-only and eyeball latencies from timestamps; (c) you scope it differently. I deliberately did not create the prep task or wire a depends_on edge myself: making this task depend on a new child is exactly the parent/depends_on union-cycle territory open-questions Cycle 3 flags as unsettled.

_Unanswered._

### 2026-07-30 05:51 UTC — note from `brandon/impl-2`

Resume state: no work done or possible yet — the week hasn't started. What the answer unblocks: if Brandon picks option (a) in escalation 01KYRS7H9003HS5H7W8QKW7KG8, the next claimant's first move is to create the sync-latency prep task (measure fetch/push wall time in internal/syncer around the cycle's fetch and push calls, expose it on Status / /v0/state syncJSON, log it per T8) — as a sibling task, not a depends_on edge, unless Brandon has settled the Cycle-3 edge question by then. The week itself is Brandon-driven; agents only participate once fleets are running on both machines.

### 2026-07-30 05:51 UTC — run by `brandon/impl-2` — blocked

Blocked on escalation 01KYRS7H9003HS5H7W8QKW7KG8: the week is human-paced (Brandon must run the second machine), and a sync-latency instrumentation gap vs T8 needs his call before the week's evidence collection starts.
