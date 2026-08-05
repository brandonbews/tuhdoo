# Confirmation gate: claim.confirmed won through the remote CAS (D6 revision 2026-08-04)

`tuh-01KZ5WMT4GWZTYVRGWN4PFN777`

- **Status:** done
- **Priority:** 2
- **Labels:** `daemon` `core` `syncer` `d6`
- **Created:** 2026-08-04 08:01 UTC by `brandon/claude-code-1`

## Description

Context: the 2026-08-04 confirmation-gate grill revised D6 (docs/design/001-core-design.md D6, 002 T5/T8, docs/agent-protocol.md step 5 — landed in PR #28; read the revised D6 first, it is the contract). Final claim verdicts move from mint-time ULID order (revocable — an earlier-minted claim can always still be in flight) to a claim.confirmed event won through the remote's atomic compare-and-swap push (irrevocable — the remote serializes). This task builds the gate. Loser handling (coercion, synthesis, warnings) is the separate successor task tuh-01KZ4TH4HT56TE4CQPKF3R76WT; this task deliberately does not touch finish_run.

The ask:
- New event type claim.confirmed (internal/event/catalog.go): payload names the claim ID it confirms; envelope carries task/actor/machine as usual. Additive schema (T3) — a new type, no version bumps. Note the deploy consequence: older binaries meeting it enter read-only fail-safe, so all fleet machines must update binaries before the first confirmation lands (call this out in the PR body).
- Replay rule (internal/core/replay.go, pure): a confirmed claim wins its contest unconditionally — it beats earlier-ULID unconfirmed claims. CORRECTNESS TRAP, handle explicitly: a confirmation binds to one claim, and settles the race, not liveness — a confirmed claim can still end (finish/release/lease-expiry interrupted), after which the task returns to the pool and a NEW contest begins in which a new claim may be confirmed. So the invariant is one confirmation per contest, not one per task forever. If a corrupt ledger ever carries two confirmations for one contest, replay resolves deterministically (earliest confirmation ULID wins) rather than failing — fail-safe determinism, not fail-stop.
- The writers' invariant at the chokepoint: the app-level merge (internal/syncer/merge.go) and the daemon's commit/push path never carry a confirmation for a task whose head state already shows a different active confirmed claim. This is what makes 'at most one per contest' true by construction — the remote CAS serializes pushes, and no push introduces a competitor.
- The gate operation (daemon): synchronous — sync against the remote, replay the head being pushed onto, proceed only if this claim is the provisional winner with no competing confirmation, commit claim.confirmed, push onto exactly that head; on non-fast-forward, refetch and re-judge (bounded retries riding the existing Syncer.Cycle shape). No remote configured: the daemon is sole writer, confirm locally and instantly (T2 — remoteless is a normal state). Remote configured but unreachable: fail with a retryable error and write nothing — the referee never guesses.
- MCP verb confirm_claim (twelfth verb, T5 as revised): requires the session to hold the claim; answers confirmed (irrevocable — merge freely) or lost (stand down), with tool-result text an agent can act on without reading docs.

Acceptance criteria:
- Table-driven core tests: confirmed claim beats an earlier-ULID unconfirmed claim; after a confirmed claim ends, a new contest's confirmation is honored; a two-confirmation corrupt ledger resolves deterministically and identically on every replay.
- Syncer test: the merge/push path refuses to introduce a second confirmation into a head with an active confirmed claim (literal-tree table test).
- Integration test in the collision-harness style: two daemons, one bare remote, both race confirm_claim on the same task repeatedly — exactly one confirmation per contest, every time, and the loser is told it lost. Zero duplicates across the run.
- Remoteless repo: confirm_claim answers instantly. Unreachable remote: retryable error, no event written.
- make test lint green from the repo root.

Constraints: boring Go (T1) — the gate is a plain loop over existing sync primitives, no new concurrency; replay stays pure (no I/O); host-agnostic (T2) — git protocol only, no host API; stored event bytes never rewritten (T3); the daemon remains the sole writer (D2). The revised docs are authoritative; if code wants to deviate, escalate rather than drift.

## History

### 2026-08-05 01:21 UTC — run by `brandon/claude-code-1` — done

- Branch: `tuh-n777/confirmation-gate`
- PR: <https://github.com/brandonbews/tuhdoo/pull/29>
- Commits: `a7da5f82e4e4cbcc129903c60f07e5857a79700d`

Confirmation gate built and merged (PR #29, squash a7da5f8). claim.confirmed event type (additive, v1, payload = confirmed claim's event ID); pure replay rule — confirmed claim wins its contest unconditionally, one confirmation per contest (not per task forever), corrupt double-confirmation resolves to earliest ULID deterministically; writers' invariant at the merge chokepoint (confirmGuard in internal/syncer/merge.go, judged at frozen tree timestamps); gate ops GateHead/GatePush in internal/syncer/gate.go — the event touches no ref until the remote CAS accepts the push; daemon opConfirmClaim with bounded 4-retry loop, instant remoteless arm (T2), 503-nothing-written on unreachable remote; MCP verb confirm_claim (twelfth verb) requiring session-held claim. All acceptance tests exist and pass incl. two-daemon race integration (5 rounds, one confirmation per contest, zero duplicates); make test lint green. DEPLOY NOTE: all fleet binaries must be rebuilt before the first confirmation lands — older binaries go read-only fail-safe on meeting claim.confirmed. Successor task tuh-01KZ4TH4HT56TE4CQPKF3R76WT (loser handling in finish_run) is now unblocked and deliberately untouched here.
