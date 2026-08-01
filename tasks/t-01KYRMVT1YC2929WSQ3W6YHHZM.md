# Render markdown views on local daemon writes

`t-01KYRMVT1YC2929WSQ3W6YHHZM`

- **Status:** done
- **Priority:** 4
- **Labels:** `go` `bug` `views`
- **Created:** 2026-07-30 04:35 UTC by `brandon/impl-1`

## Description

Context: found at B12 cutover verification — the data branch on the host shows only event JSON. views.Render is invoked only in the syncer's merge path (internal/syncer/merge.go, overlayViews); the daemon's local write path (store.Batcher.flushLocked commits Batch{Events} only) never includes views. Single-machine operation produces no merges, so the branch never gains README.md / backlog.md / escalations.md / tasks/. Violates T6 ("rendered to the data branch on every write/merge"), D3, and B8's intent ("committed alongside events by B5's writer").

The ask: stage rendered views alongside every event batch in the daemon's write path, so views ride the same commit as their events on both the eager and debounced paths. Respect the highest-version-wins guard (views.CanWrite over the branch's current .views-meta.json): if views are stamped by a newer generator, write events only and log why.

Acceptance: integration test — a daemon write lands a commit whose tree contains README.md, backlog.md, escalations.md, .views-meta.json and tasks/<id>.md, with backlog.md naming the task; guard test — a branch pre-stamped with a higher view format gets the events but its views left untouched; make test lint green; existing store/batcher/daemon/syncer tests stay green.

Pointers: internal/store/batcher.go (flushLocked), internal/store/store.go (Batch.Files already exists), internal/daemon/daemon.go (commitLocked / refreshLocked), internal/views (Render / CanWrite / Format / MetaPath), internal/syncer/merge.go (the merge-path precedent).

Constraints: boring Go (T1) — no callbacks crossing lock boundaries; render under d.mu from replayed state and hand bytes to the batcher. Views render is pure (T1). Stored event bytes are never rewritten (T3). Lease-file writes must not churn views (D9: heartbeats are junk mail).

## History

### 2026-07-30 04:39 UTC — run by `brandon/impl-1` — done

- Branch: `main`
- Commits: `f239d49`

Fixed and verified. The daemon now renders the four views from freshly replayed state after staging every write and hands them to the batcher (Batcher.AddFiles), so views ride the same commit as their events on both eager and debounced paths; the B8 highest-version-wins guard applies on this path too (Store.ReadFile checks the .views-meta.json stamp; newer stamp means events only). Tests: TestViewsRideLocalWrites, TestViewsGuardRefusesNewerStamp, TestFilesRideBatchesAndReadFile; make test lint green, store+daemon race-clean. Commit f239d49 on main, pushed. The daemon serving this very call is the fixed binary — this run's own finish event is the first production write to carry views onto the branch.
