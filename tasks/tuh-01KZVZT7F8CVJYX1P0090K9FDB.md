# Batcher: log background flush failures at failure time

`tuh-01KZVZT7F8CVJYX1P0090K9FDB`

- **Status:** open — ready
- **Priority:** none
- **Labels:** `go` `storage` `audit-finding`
- **Created:** 2026-08-12 21:59 UTC by `brandon/claude-code-bg`

## Description

Context: Go-sweep audit finding, re-verified 2026-08-27; direction decided by Brandon at the triage grill (log it). store/batcher.go's background() (96-101) discards flushLocked's error; the designed reporting channel LastError (87-94; docs at 19-21 and 103-105) has exactly one reader in the repo — a smoke assertion in store_test.go:212-214 that never exercises the failure path. A failed timer-driven flush is therefore invisible: events stay pending in memory (lost outright if the daemon dies before a later successful flush) and the error surfaces only if a later synchronous Flush happens to run.

The ask: background flush failures log at failure time — wire a logger (or plain error callback) into the Batcher from the daemon. Implementer's choice whether LastError then stays (it must gain a real reader or a failure-path test to earn its keep) or is deleted.

Acceptance: a test forces a background flush failure and asserts it is logged (and, if LastError survives, that it reports the error); no silent path remains from a failed timer flush; make test lint green.

Constraints: boring Go — a logger field or callback, nothing clever; store's public API stays minimal.

## History

### 2026-08-27 14:54 UTC — edit by `brandon/claude-code-1`

retitled · description edited · status inbox→open · labels +go +storage
