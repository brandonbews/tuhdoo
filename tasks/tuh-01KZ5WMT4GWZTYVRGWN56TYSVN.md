# Collision harness: drive the real D6 machinery, add a confirmation-race storm

`tuh-01KZ5WMT4GWZTYVRGWN56TYSVN`

- **Status:** open — waiting on an escalation answer
- **Priority:** 1
- **Labels:** `harness` `d6`
- **Depends on:** [`tuh-n777`](tuh-01KZ5WMT4GWZTYVRGWN4PFN777.md) (done), [`tuh-76wt`](tuh-01KZ4TH4HT56TE4CQPKF3R76WT.md) (done)
- **Created:** 2026-08-04 08:01 UTC by `brandon/claude-code-1`

## Description

Context: harness/collision/main.go (PR #25) proved two-machine convergence, but its settle() step plays the losing daemon's part itself — it writes the superseded runs over the HTTP surface, because at the time nothing in the real system wrote them. The 2026-08-04 D6 revision (PR #28) built the real machinery: the confirmation gate (dependency task) and honest loser handling (tuh-01KZ4TH4HT56TE4CQPKF3R76WT). Roadmap v1 DoD clause 2 was extended the same day: the milestone evidence must come from the real machinery, not the harness impersonating it, plus a confirmation-race storm.

The ask:
- Remove settle()'s impersonation: losers must discover their fate through the real path — confirm_claim answering lost, or finish_run(done) coerced to superseded — and winners must finish done through the gate. The harness drives public surfaces only (MCP/HTTP verbs), never writes outcomes on a daemon's behalf.
- Add a confirmation-race storm mode: over N collided tasks (>= 40, matching the existing burst scale), both daemons race confirm_claim; assert exactly one claim.confirmed per contest — any duplicate confirmation is a hard failure — one done run and one superseded run per collided task, and byte-identical replayed state and views on both sides afterward.
- Update the harness's printed checks to match (the 'superseded run carrying the loser's branch' check becomes: via real coercion when the loser reports, branch-less synthesis when it does not — exercise both).

Acceptance criteria: go run ./harness/collision passes end-to-end with no harness-written loser outcomes; the storm reports zero duplicate confirmations across all bursts; both convergence checks (state and views byte-identical) still pass; make test lint green. This harness run becomes the real evidence for roadmap v1 DoD clause 2 — say so in the PR body.

Constraints: the harness stays outside the daemons' internals; do not weaken finishGuardLocked's HTTP allowances to make the harness pass — if a guard blocks the real path, that is a finding to escalate, not a hole to widen.

## History

### 2026-08-05 02:29 UTC — escalation from `brandon/claude-code-1` (blocking)

> The reworked collision harness (branch tuh-ysvn/collision-harness-real-machinery, commit 5c4d547) found two production gaps in PR #30's silent-loser path; fixing them changes lease semantics, which is design territory — how should leases end when a voided claimant stands down?
>
> Finding 1 — lease deletion rewrites replay history. releaseVoidedLocked (internal/daemon/ops.go) deletes the loser's lease, but leaseExpiredBy (internal/core/replay.go) counts a MISSING lease as lapsed at EVERY instant, including past ones. In exactly the contests where a confirmation out-ranked an earlier-ULID claim, replay thereafter sees the earlier claim as lease-less at claim-apply time and records it expired with a synthesized interrupted run — not the promised superseded run. Deterministic on both machines, but the verb's 'recorded as superseded' acknowledgment is false. Hit 13/40 storm contests.
>
> Finding 2 — merge resurrects deleted leases. The union merge (internal/syncer/merge.go) brings deleted lease files back; its comment ('a resurrected lease only matters to an ACTIVE claim') predates PR #30 making voided-lease deletion load-bearing. Whether a stand-down closes immediately or waits out the 15-min TTL is a merge-timing coin flip (5/10 silent losers un-closed at verify).
>
> Options I see: (a) releaseVoidedLocked overwrites the lease with an already-lapsed expiry (a tombstone) instead of deleting — fixes finding 1 since the lease exists with a real expiry; but the merge's 'later expiry wins' rule then lets a racing renewal beat the tombstone, so finding 2 needs the rule to let releases beat renewals (e.g. tombstones win, or lease files carry a released marker). (b) Keep deletion but make leaseExpiredBy time-aware (missing lease = lapsed only from some instant) — needs a durable record of WHEN it vanished, which deletion doesn't leave; seems structurally worse. (c) Don't touch leases on stand-down: the attempt closes by natural TTL expiry (up to 15 min later); simplest, costs only latency of the synthesized close, but the ack text must stop promising immediate closure.
>
> My recommendation: (a), decided through a grill cycle since it amends the T8/merge lease rules and D6 clause 3's mechanism. The harness rewrite itself is complete and is the evidence instrument for roadmap v1 DoD clause 2: its marquee checks pass (zero duplicate confirmations across 40 raced contests, one claim.confirmed per contest, byte-identical state/views both sides); the 3 failing checks are precisely these findings, written up in harness/README.md on the branch. The harness cannot go green (its acceptance bar) until the lease design is settled and fixed in a follow-up task.

_Unanswered._

### 2026-08-05 02:29 UTC — note from `brandon/claude-code-1`

Resume state: harness rewrite is COMPLETE on branch tuh-ysvn/collision-harness-real-machinery (commit 5c4d547, pushed, unmerged). Impersonation removed (no harness-written outcomes, POST /v0/runs gone), confirmation-race storm added (-confirm-storm, default 40), printed checks updated; findings written up in harness/README.md on the branch. make test lint green; the only red is 3/17 live-harness checks, all tracing to the lease findings in escalation 01KZ7W28PB9GPHM0CSQQ2QFABM. When the answer lands: fix the lease mechanism per the decision (likely a new task touching internal/daemon/ops.go releaseVoidedLocked, internal/core/replay.go leaseExpiredBy, internal/syncer/merge.go lease rule), then rerun go run ./harness/collision — expect 17/17 — and merge the harness branch; PR body should cite it as roadmap v1 DoD clause 2 evidence.

### 2026-08-05 02:30 UTC — run by `brandon/claude-code-1` — blocked

- Branch: `tuh-ysvn/collision-harness-real-machinery`
- Commits: `5c4d547`

Harness rewrite complete and pushed (unmerged); blocked on escalation 01KZ7W28PB9GPHM0CSQQ2QFABM — two production lease-semantics gaps the storm exposed must be design-decided and fixed before the harness can go green (its acceptance bar).
