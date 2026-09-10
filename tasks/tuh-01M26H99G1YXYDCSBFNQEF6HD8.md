# Read fast, start fast: batch blob reads, Go-side object IDs, bind-before-load

`tuh-01M26H99G1YXYDCSBFNQEF6HD8`

- **Status:** open — ready
- **Priority:** 1
- **Labels:** `go` `gitx` `store` `daemon` `cli`
- **Depends on:** [`tuh-1bap`](tuh-01M26H99BZBCHHZX7G98X41BAP.md) (done)
- **Created:** 2026-09-10 20:48 UTC by `brandon`

## Description

Context: step 1 of 3 of the live-replica plan (design revision landed first: see the task this depends on). Today `store.LoadReplayInput` runs one `git cat-file` subprocess per never-decoded blob, so a cold daemon loads the dogfood ledger (876 blobs) in ~7.2s; `daemon.New` binds the socket only after that load; `cmd/tuhdoo/client.go` gives up after a fixed `spawnWait` of 5s. Result: after any daemon death the first launch of the TUI, any CLI command, or the MCP shim fails with "daemon did not come up within 5s" (the shim surfaces as "Connection closed" in the harness). Historical cold loads in daemon.log crept from 4.4s (Aug 12) past 5s (Aug 25) to 7.2s (Sep 10).

The ask, three pieces, no change to replayed state:
1. `gitx.Git` gains `CatFiles(oids []string) (map[string][]byte, error)` implemented over a single `git cat-file --batch` process (stdin: one OID per line; parse "<oid> <type> <size>\n<bytes>\n" records; a "<oid> missing" line is an error naming the OID). `CatFile` becomes a wrapper over `CatFiles` so the batch format is parsed in exactly one place. `store.LoadReplayInput` collects every OID not in its caches and fetches them in one `CatFiles` call. `syncer.replayTreeAt` does the same for its own reads (cache sharing with the store comes in step 2).
2. `gitx` gains `BlobOID(data []byte) string` computing the git blob object ID for the repo's object format. The `CLI` constructor runs `git rev-parse --show-object-format` once and stores the hash (sha1 or sha256). Unused by the write path until step 2; ships here with its tests so step 2 is smaller.
3. Startup order in `daemon.New`/`Run`: acquire the flock, bind the socket, write daemon.json, start serving, THEN `AdoptRemoteBranch`/`Init`/first `Refresh`. Until the first replay lands, every read answers sync mode `starting` (the shape `fetchState` already loops on) and every write answers a retryable error (503, `"starting"`). In `cmd/tuhdoo/client.go` delete `spawnWait`; `ensureDaemon` waits for the socket to accept up to a single generous ceiling (30s) and keeps pointing at daemon.log on failure.

Acceptance (tests must exist and pass):
- gitx table tests: `CatFiles` returns exact bytes for N blobs including a binary blob and an empty blob; a missing OID errors naming it; `CatFile(oid)` equals `CatFiles([oid])[oid]`.
- `BlobOID` matches `git hash-object --stdin` for the same bytes in a sha1 repo and in a repo created with `git init --object-format=sha256` (skip with a clear message if the local git cannot create one).
- store test with the fake git: loading N never-seen events issues exactly one `CatFiles` call, and a second load issues none.
- daemon test: with an artificially slow store load, the socket accepts a connection and `/v0/state` reports `starting` before the first replay completes; a write during that window returns 503 with `starting`; after load the same calls succeed.
- client test: `ensureDaemon` returns as soon as the socket accepts, not after a fixed wait; the 30s ceiling error names daemon.log.
- Manual, recorded in the PR body: kill the dogfood daemon, run `bin/tuhdoo status` cold; paste the daemon.log refresh line (expect load well under 1s versus 7.19s before) and confirm the command succeeds on the first try.
- `make test lint` green; PR title = task title; body opens with this task ID.

Pointers: internal/gitx/cli.go (CatFile, LsTree, runCtx), internal/gitx/gitx.go (Git interface), internal/store/store.go (LoadReplayInput, caches), internal/syncer/merge.go (replayTreeAt), internal/daemon/daemon.go (New, Run, Refresh), cmd/tuhdoo/client.go (ensureDaemon, spawnWait, liveSocket), cmd/tuhdoo/snapshot.go (fetchState's starting loop), .git/tuhdoo/daemon.log lines matching "serving" for the cold-load history.

Constraints: boring Go (a bufio scanner over one subprocess, plain loops); replayed state byte-identical before and after; the design revision's T2/T4 notes are the authority for the interface change and the startup order; deploy after landing per CLAUDE.md (rebuild, TERM the daemon, wait for exit, respawn).

## History

_No activity yet._
