# Two-machine convergence: a deliberate claim-collision harness

`t-01KYRMFV10W1N28TCN5WVTCB1J`

- **Status:** open — in progress, claimed by `brandon/claude-code-1`
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

**Answer** (`brandon`): Neither — the premise is retired. The milestone grill of 2026-08-03 rewrote this task's acceptance from 'a week of two-machine operation' to a deliberate claim-collision harness (see the rewritten description). It is no longer human-paced: two clones on one box give two genuinely independent daemons, because machineID is minted per repo directory and ULID ordering never trusts wall clocks. An agent can execute it start to finish, so it returns to the ready pool with this answer rather than waiting on me.

On the sync-latency prep task: not filed, and the latency requirement is dropped from acceptance. The grill found the deeper measurement problem — syncer.Status.Collisions counts non-fast-forward pushes, not claims voided by the D6 winner rule, so the original 'collision/latency numbers' bar would have recorded the wrong quantity no matter how much latency instrumentation landed first. The rewritten acceptance names the facts that actually prove convergence (one winner per race, a superseded run per loser, byte-identical state and views, at least one real merge commit) and folds the push-contention count in as one reported figure among several. If latency proves to matter, it gets filed on evidence from this run rather than ahead of it.

Filing note for the record: fencing this task with a blocking escalation was the wrong tool. Nothing had stalled — the task simply wasn't to be started yet, which is 'held'. The rule is now written down: docs/agent-protocol.md, 'no attempt, no escalation'.

### 2026-07-30 05:51 UTC — note from `brandon/impl-2`

Resume state: no work done or possible yet — the week hasn't started. What the answer unblocks: if Brandon picks option (a) in escalation 01KYRS7H9003HS5H7W8QKW7KG8, the next claimant's first move is to create the sync-latency prep task (measure fetch/push wall time in internal/syncer around the cycle's fetch and push calls, expose it on Status / /v0/state syncJSON, log it per T8) — as a sibling task, not a depends_on edge, unless Brandon has settled the Cycle-3 edge question by then. The week itself is Brandon-driven; agents only participate once fleets are running on both machines.

### 2026-07-30 05:51 UTC — run by `brandon/impl-2` — blocked

Blocked on escalation 01KYRS7H9003HS5H7W8QKW7KG8: the week is human-paced (Brandon must run the second machine), and a sync-latency instrumentation gap vs T8 needs his call before the week's evidence collection starts.

### 2026-08-03 22:04 UTC — note from `brandon/claude-code-1`

Measurement record — collision harness run, 2026-08-03, defaults (`go run ./harness/collision`, harness landed in PR #25). Two independent runs the same day produced these figures on every line; the numbers below are the second, which I ran and verified myself.

    claim rounds fired            10
    claim races observed          10   (every round contested)
    claims made                   20
    claims voided (D6 losers)     10
    merge commits on data branch  2
    non-fast-forward pushes       11   (alpha 0, bravo 11)
    app-level merges built        13   (alpha 0, bravo 13)
    pushes losing the ref lock    1    (alpha 0, bravo 1)
    maxCycleRetries exhausted     1    (9.1% of non-fast-forward pushes)

134 events, byte-identical replayed state, 18 byte-identical generated view files, identical data-branch trees on both clones. All ten acceptance checks green.

Read the counters carefully, they are not interchangeable:
- "non-fast-forward pushes" is `syncer.Status.Collisions` — push contention, NOT claim collisions. Voided claims are derived from replayed state (`core.ClaimVoided`), which is the only place they are counted at all.
- "app-level merges built" is `syncer.Status.Merges`, which counts merge commits *constructed*; a merge whose ref update or push then loses a race is discarded, hence 13 built vs 2 landed on the branch. And with exactly two peers, whichever daemon is marginally faster owns the ref every time and never merges — all 13 merges were on one side. Don't read one machine's `merges` as a fleet number.
- "pushes losing the ref lock" is push contention the daemon does not count as contention at all (finding 4 below).

### 2026-08-03 22:04 UTC — note from `brandon/claude-code-1`

Findings from the run. The convergence half of the acceptance passed cleanly; these are what the experiment turned up on the way, all written up in `harness/README.md`.

1. **The daemon never writes the `superseded` run D6 promises** — the one finding that is a real gap, not a caveat. Replay voids the loser's claim and stops there; the only run replay ever synthesizes is `interrupted` (lease expiry). The MCP surface rejects the outcome from agents as "daemon-synthesized" (`internal/daemon/mcp.go`), no daemon code synthesizes it, and there is no CLI verb (work loop is session-only, T7). The only writer that exists is `POST /v0/runs` on the unix socket — and `finishGuardLocked` (`internal/daemon/ops.go`) explicitly anticipates this exact case. So the shape is designed for and has no writer: **a real fleet today leaves voided claims with no run at all.** The harness plays the part D6 assigns to the losing daemon, over that HTTP surface, which is why the acceptance line passes. Captured as its own task (see below) rather than fixed here — it is daemon behaviour, not harness scope.

2. **D6's "machine-id tiebreak" is vacuous.** The implemented rule is "earliest event ID wins": replay sorts by ULID, and ULIDs (crypto/rand + monotonic) never tie, so the machine id is never consulted. The harness asserts the rule it can actually observe — surviving claim is lexicographically smallest on its task — and separately checks winner and loser sit on different machines. The doc clause describes a branch that does not exist. Harmless, but it should say so.

3. **`maxCycleRetries` (4) does exhaust under sustained symmetric eager writes**, at ~3.4 writes/s/machine with both machines writing at once: one daemon reported "remote origin kept moving for 4 attempts" and its work stopped reaching the remote for the length of the burst. Nothing was lost — the ledger converged once the burst stopped — but while it lasted one machine was invisible to the other, which is precisely the window D6 races open in. The load is ~100x realistic by design; this is a ceiling measurement, not a bug report.

4. **Some push contention is not counted as contention.** Two peers pushing to one bare repo at the same instant can get `cannot lock ref 'refs/heads/tuhdoo': is at X but expected Y` — a lost ref-update race, not a stale history. `gitx.Push` classifies a rejection as `ErrNonFastForward` only on "non-fast-forward"/"fetch first" in git's output (`internal/gitx/cli.go`), so this shape falls through as a generic error: `Syncer.Cycle` returns instead of going round its retry loop, the daemon records `mode=error`, and `Status.Collisions` does not count it. The next cycle recovers, so nothing is lost — but the push-contention counter T8 says the daemon keeps is an undercount.
