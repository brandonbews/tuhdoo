# Batch blob writes and the gate's tree build: hash-object --stdin-paths and a second private index for GatePush

`tuh-01M27FW5A3YG8XZYEH0KDSVCMN`

- **Status:** inbox — untriaged capture
- **Priority:** none
- **Labels:** `go` `gitx` `store` `syncer` `perf`
- **Created:** 2026-09-11 05:43 UTC by `brandon/claude-code-1`

## Description

Measured on 2026-09-10 during review of PR #104 (live replica): 171 per-blob `hash-object -w --stdin` spawns cost 1.44-1.48s (~8.5ms each) versus 83-85ms for temp files under <git-dir>/tuhdoo/blobs/ plus one `hash-object -w --no-filters --stdin-paths`; the ledger already has a 110-page commit (~0.9s under both mutexes) and a 93-page gate commit. GatePush still builds its commit-on-remote-head through MkTreeFromMap: 29 mktree processes, ~180ms in the synchronous confirm_claim path; a second private index (GIT_INDEX_FILE=<git-dir>/tuhdoo/gate-index: read-tree + update-index + write-tree) gives the identical tree OID in 3 processes, ~20ms, but needs an index-name parameter on the gitx index verbs and a mutex (gateVerdict is not serialized). Design record 002 T2 (2026-09-10) explicitly accepts per-blob writes and says batching is a local gitx change if measurement demands it; this is that measurement.

## History

_No activity yet._
