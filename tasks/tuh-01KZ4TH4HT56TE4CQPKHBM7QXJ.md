# D6's machine-id tiebreak is vacuous — ULIDs never tie, so the tiebreak branch does not exist

`tuh-01KZ4TH4HT56TE4CQPKHBM7QXJ`

- **Status:** cancelled
- **Priority:** 0
- **Labels:** `docs` `d6`
- **Created:** 2026-08-03 22:04 UTC by `brandon/claude-code-1`

## Description

Found by the collision harness (task t-01KYRMFV10W1N28TCN5WVTCB1J).

D6 states the winner rule as earliest ULID with a machine-id tiebreak. As implemented, replay sorts events by ULID and the first claim to land holds the task; ULIDs are minted from `ulid.Monotonic` over crypto/rand, so two claims can never tie and the machine id is never consulted. The tiebreak clause describes a branch of the rule that does not exist in code.

Harmless in behaviour, but the doc should either say so or the intent behind the tiebreak should be recovered — it may have been guarding a case (same-millisecond claims across machines) that ULID uniqueness already covers. Likely a design-doc revision to `001` D6 rather than a code change.

## History

### 2026-08-04 08:01 UTC — note from `brandon/claude-code-1`

Absorbed by the 2026-08-04 confirmation-gate grill (Brandon's direction, this session). The D6 revision (PR #28) deleted the machine-id tiebreak clause outright as vacuous — ULIDs never tie — and went further: the mint-time ULID rule itself is demoted to a provisional verdict, with the final verdict now a claim.confirmed event won through the remote's CAS push. The doc change this capture asked for is a subset of that rewrite. Cancelling as absorbed, not as wrong: the finding was correct and is credited in the D6 revision note.
