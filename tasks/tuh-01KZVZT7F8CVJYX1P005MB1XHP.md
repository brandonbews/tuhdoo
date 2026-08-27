# One lease-path parser: store loader and merge-time replay must compute the same lease set

`tuh-01KZVZT7F8CVJYX1P005MB1XHP`

- **Status:** open — ready
- **Priority:** none
- **Labels:** `go` `storage` `audit-finding`
- **Created:** 2026-08-12 21:59 UTC by `brandon/claude-code-bg`

## Description

Context: Go-sweep audit finding, re-verified 2026-08-27 — three parsers now, not two. Store loader, strict: store/store.go:260-265 (CutPrefix/CutSuffix; skips non-.json or embedded '/'). Merge-time replay, blind: syncer/merge.go:342 (TrimPrefix/TrimSuffix; ingests leases/foo as claim "foo" and leases/a/b.json as "a/b"). Writer-side leasePath (store/lease.go:31-37) validates claim IDs, not paths, and is unexported — reusable by neither reader. Extra edge found at verification: leases/.json yields claimID "" in BOTH readers; the store guard misses it too. Two readers of one tree computing different lease sets is the divergence posture T3/T8 legislate against, at low stakes (no writer produces malformed paths).

The ask: extract one parser into store, which owns the lease format — e.g. LeaseClaimID(path string) (string, bool) — and use it in the store loader and replayTreeAt. Strict semantics win: skip malformed paths in both readers (a lease no writer could have produced is noise, and skipping errs toward treating a claim as expired — replay's existing posture for a missing lease), and reject the empty claim ID.

Acceptance: table-driven tests on the parser (valid; non-.json; nested path; leases/.json → rejected); a merge-path test proving replayTreeAt now skips exactly what the loader skips; make test lint green.

Constraints: valid paths parse identically to today; the only behavior change is on malformed paths, where the two readers previously diverged.

## History

### 2026-08-27 07:06 UTC — edit by `brandon/claude-code-1`

retitled · description edited · status inbox→open · labels +go +storage
