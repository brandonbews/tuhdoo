# get_task and /v0/state evaluate lease expiry at read time

`tuh-01KZVZT7F8CVJYX1NZZS3GK62E`

- **Status:** open — in progress, claimed by `brandon/claude-code-2`
- **Priority:** none
- **Labels:** `go` `daemon` `audit-finding`
- **Created:** 2026-08-12 21:59 UTC by `brandon/claude-code-bg`

## Description

Context: Go-sweep audit finding, re-verified 2026-08-27. D6 clause 5: "expiry is evaluated at read time by replay logic". opGetTask (internal/daemon/ops.go:984-991) and handleState (api.go:447-449) serve cached state with no refreshLocked(now); every write op refreshes, and opBacklog refreshes precisely because "a stale expiry must not hide a ready task" (ops.go:997-999) — the same argument, unapplied. No mitigating background refresh exists: d.Refresh's only production caller is the syncer's OnMerged hook (daemon.go:224), so remoteless or write-idle, the TUI's 2s /v0/state poll renders a lapsed lease as a live holder indefinitely (st.Holder, api.go:465). Direction decided by the design letter: make the code match D6; don't revise the wording.

The ask: refreshLocked(now) at the top of opGetTask and handleState, and audit the other read-only serving paths (escalations listing) for the same gap.

Acceptance: daemon tests: with a past-dated lease and no intervening write, GET /v0/tasks/{id} and /v0/state report the claim lapsed / the task ready. Replay is µs-scale (internal/core/replay_bench_test.go), so per-poll refresh is fine — if a benchmark says otherwise, stop and capture rather than caching cleverly. make test lint green.

Constraints: read ops stay read-only (refresh recomputes in-memory state, writes nothing); D6 wording unchanged. Adjacent but NOT this task: the rendered data-branch views have their own expiry-staleness gap with no natural trigger — that half is owned by tuh-01M0XBC1P2NYTPZQ4BAFSWHGY1.

## History

### 2026-08-27 07:06 UTC — edit by `brandon/claude-code-1`

retitled · description edited · status inbox→open · labels +go +daemon
