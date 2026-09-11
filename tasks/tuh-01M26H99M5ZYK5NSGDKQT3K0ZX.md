# The store as live replica: in-memory head and tree, single ref mover, persistent index, write only what changed

`tuh-01M26H99M5ZYK5NSGDKQT3K0ZX`

- **Status:** done
- **Priority:** 1
- **Labels:** `go` `store` `gitx` `syncer` `daemon`
- **Depends on:** [`tuh-6hd8`](tuh-01M26H99G1YXYDCSBFNQEF6HD8.md) (done)
- **Created:** 2026-09-10 20:48 UTC by `brandon`

## Description

Context: step 2 of 3 of the live-replica plan. Today every read handler (14 `refreshLocked` call sites in internal/daemon) runs `rev-parse` + `ls-tree` + full replay under `d.mu`; every `store.AppendBatch` re-reads `ls-tree`, runs `hash-object` on all ~166 rendered files (160 of them unchanged), and one `mktree` per subtree (25); `syncer.reconcile` moves the ref itself with its own `UpdateRef`; the syncer's merge and gate replays read every event of a tree with no cache. Measured: ~36ms per read op, ~195 subprocesses and ~2s per commit, ~6s per merge or gate replay. After this step, reads touch memory only and a commit costs about six processes plus one per genuinely new blob.

The ask:
1. The store holds the local head OID, the head's tree map (path -> OID), and its decoded caches in memory. `LoadReplayInput` (or its successor) reads memory; git is touched only by `Load` (cold start and the reload path) and by `Commit`.
2. Single mover: `Store.Commit(changes, parents, message)` is the only code that moves refs/heads/tuhdoo. `changes` is path -> OID or delete. It computes the new tree map from memory, builds the tree through the private index, writes a commit, CAS-updates the ref, and on success replaces head/tree in memory. On `ErrRefCASFailed` it reloads head/tree from git (rev-parse + ls-tree, the rare path), rebuilds, and retries (bounded, as today). `AppendBatch` becomes a caller of `Commit`. `syncer.reconcile` computes the union tree map (pure, as today) and calls `Commit` with two parents; its own `UpdateRef` and tree re-reads are deleted. The syncer now depends on the store; `New` signatures change accordingly.
3. Blob writes: for each event and each rendered file, compute `BlobOID`; skip anything whose OID already sits at that path in the tree map; `hash-object -w --stdin` only the new ones (one process each).
4. Tree build: private index at `.git/tuhdoo/index` via `GIT_INDEX_FILE`. On every daemon start: `read-tree <head tree>` (mandatory reseed, not an optimization). Per commit: one `update-index --index-info` fed only the changed paths (mode `100644` adds/updates; mode `0` removes, which is what D9 compaction will use), then one `write-tree`. If the index is missing or `read-tree` fails, recreate it; never build a tree from an index that was not reseeded this process lifetime.
5. Shared cache: the syncer's `replayTreeAt`, `replayTreeFrozen`, and the gate read decoded events and leases from the store by OID (`store.EventsByOID(oids)` fetching only unknown OIDs via `CatFiles`), so a merge or gate replay after a fetch reads only the other machine's new blobs.
6. External-move guard: the syncer's cycle already calls `ReadRef`; if it differs from the store's in-memory head (and the store did not just move it), the store reloads from git and the daemon refreshes. This is the only place the ref is checked against git outside Commit; accepted in the design revision.
7. Daemon: `refreshLocked` no longer calls git; it replays from the store's memory. The 14 call sites stay in this step (the memo and version arrive in step 3). `stageViewsLocked` is unchanged in trigger but its files now flow through the OID skip in (3), so a write that changes two pages writes two blobs.

Acceptance (tests must exist and pass):
- store, fake git: a commit with one new event and 166 rendered files of which 5 changed calls `HashObject` exactly 6 times, `UpdateIndex` once, `WriteTree` once, `CommitTree` once, `UpdateRef` once; the fake asserts no `LsTree`/`ReadRef` on the success path.
- store: CAS failure reloads head/tree from git, rebuilds on the new head, retries, and the resulting tree contains both the concurrent change and ours; exhaustion still errors as today.
- store: on `New`/`Load` the index is reseeded (`ReadTree` called once with the head tree); a test that pre-populates the index file with garbage entries proves the next commit's tree matches the in-memory tree map exactly (compare `ls-tree` of the result against the map).
- store: mode-0 removal through the index drops the path from the committed tree.
- syncer tests run against a real store over the fake git (or a temp git repo where they already do): union merge commits through `Store.Commit` with two parents; the syncer never calls `UpdateRef` (the fake asserts zero calls from the syncer).
- syncer: a merge replay after a fetch fetches only OIDs absent from the store's cache (fake counts `CatFiles` OIDs).
- daemon, fake git: every GET handler and every MCP read tool issues zero git calls (the fake records calls; assert none between requests once loaded).
- external-move guard: a test moves the ref behind the store's back, runs one sync cycle, and asserts the daemon's state reflects the moved ref.
- `make test lint` green; PR title = task title; body opens with this task ID and pastes a before/after daemon.log commit line (expect "load" well under 10ms and a commit in tens of ms on the dogfood ledger).

Pointers: internal/store/store.go (AppendBatch, LoadReplayInput, Init, caches), internal/store/batcher.go, internal/gitx/cli.go (mkTreeLevel to be retired from the commit path; add UpdateIndex/WriteTree/ReadTree), internal/gitx/gitx.go (Git interface), internal/syncer/syncer.go (Cycle, reconcile), internal/syncer/merge.go (merge, treeMap, replayTreeAt, replayTreeFrozen, confirmGuard), internal/syncer/gate.go (GateHead, GatePush), internal/daemon/daemon.go (refreshLocked, commitLocked, stageViewsLocked), internal/daemon/ops.go (refreshLocked call sites), the design revision's T2 notes (single mover, private index, Go-side OIDs, CatFiles).

Constraints: boring Go, one mutex per struct, plain loops. Replay stays pure and byte-identical in outcome. `MkTree` may remain in the interface for `Init`'s empty tree and tests but the commit path must not use it. Nothing under .github/workflows. Deploy after landing per CLAUDE.md.

## History

### 2026-09-11 06:02 UTC — run by `brandon/claude-code-1` — done

- Branch: `tuh-01M26H99M5/store-live-replica`
- PR: <https://github.com/brandonbews/tuhdoo/pull/104>
- Merged as: `98c46d51eaaaabfa3b90ec442d9297323db55fe3`

Landed as PR #104 (squash 98c46d5). Store holds head, tree map, and decode caches in memory; Store.Commit(base, changes, extraParents, msg) and Store.FastForward are the only movers of refs/heads/tuhdoo (private index at <git-dir>/tuhdoo/index via ReadTree/UpdateIndex/WriteTree; Go-side BlobOID so only new blobs are hash-object'd; CAS update-ref; a lost CAS reloads and returns ErrRefCASFailed — AppendBatch owns the bounded retry and drops rendered files on retry, merges defer to the next sync pass). The index is reseeded lazily by the committing process at the first commit after any load/reload/failed commit, a missing index and file/directory tree-shape conflicts are refused before git is called (review found update-index silently drops subtrees otherwise). Syncer takes the store, merges through Commit with two parents, replays from the store's caches by OID, and checks the ref against the replica every cycle (remoteless too). Daemon reads never touch git. Measured on a clone of this ledger: refresh load 30-43ms -> ~0.3ms; commit of 1 event + 172 files 1.73s -> 94ms. Deferred with measurements: tuh-01M27FW5A3YG8XZYEH0KDSVCMN (batch hash-object, gate index). Design record 002 T2 precision note added (lazy reseed). Deploy follows this finish.
