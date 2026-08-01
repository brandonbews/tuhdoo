# Retire full-replay-per-write and the grow-forever event overlay

`t-01KYRMFV10W1N28TCN5ZZ9Z2C1`

- **Status:** done
- **Priority:** 1
- **Labels:** `go` `performance`
- **Created:** 2026-07-30 04:28 UTC by `brandon/migrator`

## Description

Context: the flagged optimization point from the build-out (sessions 1-4): the daemon replays the full event set on every write and holds a grow-forever in-memory event overlay. Correct and acceptable at dogfood scale; the wrong shape eventually.

The ask: measure first. First deliverable is evidence from a real data branch (replay timings and event counts from daemon logs), not a rewrite. Only if the numbers show real pain: incremental state maintenance or snapshot-bounded replay windows.

Acceptance: before/after numbers on a real data branch; the deterministic core stays pure functions with table-driven tests (T1); all existing convergence and race tests stay green.

Constraints: correctness over speed; no clever concurrency (T1); stored event bytes are never rewritten (T3).

## History

### 2026-07-31 05:30 UTC — run by `4099114+brandonbews/claude-code-2` — done

- Branch: `main`
- Commits: `91161ba`

Measured first, as asked. Real data branch (50 events, 11 leases): full refresh was 503ms, of which pure replay was 1.1ms (0.2%) — the flagged suspect (full replay per write) is innocent. The real cost was one `git cat-file` subprocess per blob (~6.5-8ms each), linear in event count. Fix: content-addressed decode caches in internal/store (blob OID → decoded value; safe because git blobs are immutable) served by a new combined LoadReplayInput — warm refresh is now ~13ms flat regardless of log size (was heading to ~7s/write at 1k events). The grow-forever overlay now trims events once a load sees them on the branch (bounded by the debounce window, not process lifetime; TestOverlayTrimsAfterFlush). Permanent instrumentation: daemon logs a refresh timing line (load vs replay split) on every event-count change and any refresh ≥250ms. Benchmarks landed: BenchmarkReplay (internal/core: 2.5ms @ 1k, 253ms @ 100k, linear) and BenchmarkLoadReplayInput (internal/store: cold vs warm). Recommendation recorded in code comments: do NOT build incremental state or snapshot-bounded replay — replay purity is worth more than 2.5µs/event; revisit only if daemon.log timing lines ever show the replay component (not load) approaching 250ms. Core stayed pure; stored bytes untouched; full suite plus -race on store/daemon/syncer green.
