# Versioned state: memoized replay, long-poll snapshot, views follow the version, TUI without the tick

`tuh-01M26H99QVANVJCXR9FM2Z8Q7X`

- **Status:** open — in progress, claimed by `brandon/claude-code-1`
- **Priority:** 1
- **Labels:** `go` `core` `daemon` `views` `tui` `cli`
- **Depends on:** [`tuh-k0zx`](tuh-01M26H99M5ZYK5NSGDKQT3K0ZX.md) (done)
- **Created:** 2026-09-10 20:48 UTC by `brandon`

## Description

Context: step 3 of 3 of the live-replica plan, the one that changes what you see. After step 2 every read is a memory replay (~14ms) but the daemon still has no notion of "has anything changed": the TUI ticks every 2s, hydrates every task with one GET each, and has no in-flight guard (cmd/tuhdoo/top.go tickMsg, cmd/tuhdoo/snapshot.go fetchSnapshot); the rendered markdown views are a side effect of `commitLocked` only, so claims/releases never re-render (tuh-01M0XBC1P2NYTPZQ4BAFSWHGY1) and a lease expiring re-renders nothing at all (on 2026-09-10 the branch's backlog.md listed 3 in progress against 1 live); claim commits never poke the syncer (tuh-01KZWNMJH264W3B3TGNP7FP51R).

The ask:
1. core: `NextTransition(leases map[string]time.Time, now time.Time) (time.Time, bool)` returns the earliest lease expiry strictly after now (pure, table-tested). Replay is unchanged.
2. daemon: a monotonic `version uint64`, a memoized `state` with `validUntil`, and a broadcast channel replaced-and-closed on every bump (the plain "closed channel as broadcast" idiom, no sync.Cond gymnastics). Version bumps on: events staged, a commit landing, a merge (the syncer's OnMerged), and each lease transition (a timer armed to `validUntil`, re-armed on every bump). Reads at an unchanged version serve the memo; a read past `validUntil` replays once, bumps, and serves. Shutdown bumps once so parked requests return before the HTTP server drains.
3. API: `GET /v0/snapshot?since=N&wait=30s` returns `{version, degraded, sync, tasks: [hydrated...], open_escalations, runs}` immediately when version > N, otherwise parks until a bump or the wait elapses (then returns the current version so the client re-polls). Delete `/v0/state` and `GET /v0/tasks/{id}`. The write endpoints are untouched. The twelve MCP tools read the memo (no new tools, no changed schemas).
4. Views follow the version: on every bump, render (pure, `views.Render(state)`), compute each file's `BlobOID`, and stage only files whose OID differs from the tree map; they ride the batcher's quiet period as today. `stageViewsLocked` and its call-site discipline are deleted; claim and release paths lose their special-casing. A lease transition therefore produces a view-only commit ("0 events, N files") which is the designed D9 trigger. Claim commits call `sync.Poke()` (T8 eager wire time). Renewals stage nothing because their rendered bytes do not change (assert, do not assume).
5. Clients: `fetchSnapshot` becomes one request. `tuhdoo status`, `backlog`, `task <id>`, `escalations` read the snapshot and print byte-identical output to today (golden tests prove it). The TUI: `Init` issues one long-poll with since=0; each `snapMsg` re-renders and issues the next long-poll with the received version; `tickCmd`/`tickMsg` are deleted; exactly one request is ever in flight; an error renders the existing "daemon unreachable (retrying)" line and retries with a short backoff (1s, doubling to 5s). The detail screen keeps reading from the polled snapshot. No key, screen, section, or output changes.

Acceptance (tests must exist and pass):
- core table test: NextTransition over empty, all-past, mixed, and tombstoned (released) leases; ties.
- daemon: version increments exactly once per staged write, per merge callback, per lease transition; a parked snapshot request wakes within one scheduler tick of a bump; a lease that expires at T wakes a parked request at T (fake clock); `since` equal to the current version parks, `since` behind it returns at once; the wait timeout returns the unchanged version; `Shutdown` returns a parked request before the listener closes.
- daemon: the memo is served (fake replayer counts calls) for reads at an unchanged version; a read after `validUntil` replays once.
- views: claim.made then its lease write yields staged bytes whose backlog.md contains the task under In progress; a release yields it back under Ready; a lease expiry (fake clock) yields a view-only batch; three renewals stage zero files; the claim path pokes the syncer (fake syncer counts pokes).
- API: `/v0/state` and `/v0/tasks/{id}` return 404; the snapshot's hydrated task shape equals the old per-task shape field-for-field (a test decodes one into the other).
- cmd/tuhdoo: golden tests for `status`, `backlog`, `task`, `escalations` unchanged; TUI golden tests unchanged; a TUI interaction test proves a second snapshot is not requested while one is in flight and that an error path retries with backoff.
- Manual, in the PR body: warm TUI first paint measured under 100ms (script(1) or a stopwatch on the first rendered row); daemon at rest with one open pane shows no subprocess spawns in a 60s window (`ps` CPU time flat); after a claim via the MCP shim, GitHub's backlog.md shows the task in progress within one sync interval.
- `make test lint` green; PR title = task title; body opens with this task ID.

Pointers: internal/core/replay.go (leaseExpiredBy, Input.Now), internal/daemon/daemon.go (refreshLocked, commitLocked, stageViewsLocked, Shutdown), internal/daemon/api.go (handleState, handleGetTask, routes), internal/daemon/ops.go (claimTargetLocked, releaseLocked, opGetTask, the refreshLocked call sites), internal/daemon/mcp.go (renewal refresh), internal/views/views.go (Render), cmd/tuhdoo/top.go (Init, tickCmd, tickMsg, snapMsg, the loading/unreachable views), cmd/tuhdoo/snapshot.go (fetchState, fetchSnapshot), cmd/tuhdoo/commands.go, the design revision's D6/T4/T6 notes.

Constraints: T5 twelve tools unchanged; T7 output contract byte-identical; boring Go (one mutex, one timer, one channel); replay purity untouched; nothing under .github/workflows. Deploy after landing per CLAUDE.md; note that the restart kills live MCP sessions.


Added 2026-09-10: one more manual acceptance item, in the PR body. Leave an armed pane open against the new daemon for at least one hour with the ledger active (a drain session running is ideal) and record its RSS at start and end from ps; it must be flat within a few MB. This is the verification for tuh-01M11XDA6ST74WD1JWCY0GS5FW (3.4GB pane overnight), which is triaged as the pile-up this step removes; cite that ID in the PR body with the numbers.

## History

### 2026-09-10 20:59 UTC — edit by `brandon`

description edited

### 2026-09-11 19:36 UTC — note from `brandon/claude-code-1`

Branch: tuh-01M26H99QVANVJCXR9FM2Z8Q7X/versioned-state. Plan: server half first (core NextTransition, daemon version/memo/broadcast, /v0/snapshot long-poll, views on bump, delete /v0/state + GET /v0/tasks/{id}), then cmd/tuhdoo clients (one-shot commands + TUI long-poll without tick), then manual measurements for the PR body (first paint, 60s no-spawn, sync latency, 1h RSS for tuh-01M11XDA6ST74WD1JWCY0GS5FW).
