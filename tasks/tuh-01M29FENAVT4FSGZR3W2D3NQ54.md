# Store: released-lease tombstones keep the exact stand-down instant

`tuh-01M29FENAVT4FSGZR3W2D3NQ54`

- **Status:** open — ready
- **Priority:** none
- **Labels:** `go` `storage` `audit-finding`
- **Created:** 2026-09-12 00:14 UTC by `brandon/claude-code-1`

## Description

Context: found 2026-09-11 by the collision harness while landing tuh-01KZVZT7F8CVJYX1P00BRPGMTX (the bounded harness extension): two of forty storm contests replayed the earlier-ULID loser as expired with an interrupted run instead of the promised superseded one. Cause (internal/store/lease.go encodeLeaseFile): released tombstones were truncated to the second like plain leases. 002 T8 promises a tombstone is live before the instant and lapsed from it onward; truncation moved that boundary up to 999 ms into the past, so a stand-down landing in the same wall-clock second as the winner claim read as lapsed at that claim instant and replay re-adjudicated the settled contest. Hidden until the 2026-09-10 speedups made a contest sub-second.

The ask: store the tombstone instant exactly (RFC3339Nano) and leave plain-lease expiries at second precision; add a dated clarification to the T8 tombstone paragraph in 002.

Acceptance: a store test writes a lease, releases it at an instant with sub-second precision, and reads back exactly that instant, with a claim instant 300 ms earlier in the same second still finding the lease live; the encoding round-trip test pins exact tombstones and still decodes old second-precision tombstones; make test lint green.

Constraints: no reader changes (RFC3339 parsing already accepts fractional seconds, so older binaries are unaffected); plain lease bytes unchanged; stored bytes never rewritten.

## History

_No activity yet._
