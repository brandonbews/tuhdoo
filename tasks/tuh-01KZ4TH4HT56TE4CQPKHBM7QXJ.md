# D6's machine-id tiebreak is vacuous — ULIDs never tie, so the tiebreak branch does not exist

`tuh-01KZ4TH4HT56TE4CQPKHBM7QXJ`

- **Status:** inbox — untriaged capture
- **Priority:** 0
- **Labels:** `docs` `d6`
- **Created:** 2026-08-03 22:04 UTC by `brandon/claude-code-1`

## Description

Found by the collision harness (task t-01KYRMFV10W1N28TCN5WVTCB1J).

D6 states the winner rule as earliest ULID with a machine-id tiebreak. As implemented, replay sorts events by ULID and the first claim to land holds the task; ULIDs are minted from `ulid.Monotonic` over crypto/rand, so two claims can never tie and the machine id is never consulted. The tiebreak clause describes a branch of the rule that does not exist in code.

Harmless in behaviour, but the doc should either say so or the intent behind the tiebreak should be recovered — it may have been guarding a case (same-millisecond claims across machines) that ULID uniqueness already covers. Likely a design-doc revision to `001` D6 rather than a code change.

## History

_No activity yet._
