# Lease-path parsing diverges between store loader and merge-path replay

`tuh-01KZVZT7F8CVJYX1P005MB1XHP`

- **Status:** inbox — untriaged capture
- **Priority:** 0
- **Labels:** `audit-finding`
- **Created:** 2026-08-12 21:59 UTC by `brandon/claude-code-bg`

## Description

Go-sweep audit finding. store/store.go ~272-276 validates leases/<claimID>.json paths (skips non-.json or embedded '/'); syncer/merge.go replayTreeAt ~347-348 blindly TrimPrefix/TrimSuffixes — so leases/foo or leases/a/b.json is skipped by the daemon's loader but ingested by merge-time replay. Two readers of the same tree computing different lease sets is the divergence posture T3/T8 legislate against, at low stakes. Unifying changes merge-path behavior on malformed paths, so it was out of the sweep's zero-behavior scope. Fix: extract one path parser into store (which owns the lease format, e.g. LeaseClaimID(path) (string, bool)) and use it in both; decide which semantics win.

## History

_No activity yet._
