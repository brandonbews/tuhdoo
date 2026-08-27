# closedByRun: match closes by claim, not actor — a later attempt must not erase a lost attempt's superseded trace

`tuh-01KZVZT7F8CVJYX1NZZTB1SMWW`

- **Status:** open — ready
- **Priority:** none
- **Labels:** `go` `core` `audit-finding`
- **Created:** 2026-08-12 21:59 UTC by `brandon/claude-code-bg`

## Description

Context: Go-sweep audit finding, re-verified 2026-08-27 — and worse than first written: the daemon's write side shares the conflation. Replay's closedByRun (internal/core/replay.go:518-529, decision at 524) matches a voided claim's closing run by task+actor+ULID order, never r.Claim == c.ID, because apply stores non-holder runs with Claim unset (381-398; Claim assigned only under the holder guard at 384-385) while synthesized superseded runs carry Claim=c.ID (512-516). Confirmed scenario: claim c1 voided, lease lapses, winner releases without finishing, same actor re-claims (c2) and finishes — the c2 run suppresses superseded synthesis for c1 (loser-expiry loop, 140-148) and double-books as both attempts' close, contradicting D6 clause 3 ("a loser that never reports leaves a trace anyway"; one close per attempt). Write side: attemptCloseLocked uses the identical predicate (internal/daemon/ops.go:1179-1192) and finishGuardLocked judges only latestClaim (1165-1173, 1232-1270), so after the re-claim the loser cannot record attempt 1 separately by ANY path. No test covers the re-claim shape (supersede_test.go is single-claim-per-actor throughout).

The ask: make run-to-claim linkage real so closes match by claim: a loser's real coerced-superseded run carries its claim's ID in replayed state, and closedByRun (plus the daemon-side predicate) matches on it. If that requires run.finished (or the write path) to name the claim, that is T3-additive — events without the field keep replaying via the current heuristic as fallback so existing history is unchanged.

Acceptance: table-driven core tests: (1) the re-claim scenario yields TWO closes — a superseded trace for c1 and the done run for c2; (2) a loser's real salvage run carries its claim ID in state; (3) legacy-shaped history (runs without linkage) replays to identical state as today. make test lint green.

Constraints: stored event bytes never rewritten (T3); additive schema only, with a design-doc revision note if the event gains a field; deterministic core stays pure; D6 clause 3 is the spec.

## History

### 2026-08-27 07:06 UTC — edit by `brandon/claude-code-1`

retitled · description edited · status inbox→open · labels +go +core
