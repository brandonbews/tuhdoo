# Collision harness: drive the real D6 machinery, add a confirmation-race storm

`tuh-01KZ5WMT4GWZTYVRGWN56TYSVN`

- **Status:** open — blocked on dependencies
- **Priority:** 1
- **Labels:** `harness` `d6`
- **Depends on:** [`tuh-n777`](tuh-01KZ5WMT4GWZTYVRGWN4PFN777.md) (open), [`tuh-76wt`](tuh-01KZ4TH4HT56TE4CQPKF3R76WT.md) (inbox)
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

_No activity yet._
