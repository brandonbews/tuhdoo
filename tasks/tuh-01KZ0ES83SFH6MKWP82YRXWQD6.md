# One classifier: the daemon serves the derived situation

`tuh-01KZ0ES83SFH6MKWP82YRXWQD6`

- **Status:** inbox — untriaged capture
- **Priority:** 0
- **Labels:** `cleanup` `datashape`
- **Created:** 2026-08-02 05:22 UTC by `brandon/claude-code-1`

## Description

## Context

A datashape exploration (2026-08-01, session with Brandon) found the claimability/bucket rules implemented three times:

1. `core.State.Ready` / `ClaimBlockers` (internal/core/state.go) — authoritative, table-driven-tested.
2. A hand-mirror in cmd/tuhdoo/snapshot.go (`claimable`, `hasUnmetDeps`, `blockingEscalation`) — exists only because `/v0/state` carries no verdict, and can silently drift from core.
3. A redundant re-filter in `views.classify` (internal/views/views.go:105 and :125): it re-checks `blockingEscalation == nil` on tasks `ReadyTasks()` already vetted. Its comment ("a task the core calls ready can still be waiting on a blocking escalation") predates Ready gaining the escalation clause via ClaimBlockers and is now false — the filter is a no-op.

Blessed rule (Brandon, 2026-08-01): exactly one implementation of the derived-situation ("weather") logic, in core. Brandon's blessed-state write-up may land before this is claimed — if it exists, it is the source of truth over this description.

## The ask

- Delete the dead filter and stale comment in `views.classify` (both the ready-loop re-check and the `|| blockingEscalation(...)` disjunct in the blocked arm).
- Extend `stateTask` in the `/v0/state` response (internal/daemon/api.go) with the core-computed verdict — the derived situation and/or the blocker lists from `ClaimBlockers` — so the CLI classifies from served fields only.
- Delete the mirror predicates from cmd/tuhdoo/snapshot.go; `classify`, `waitingOn`, and the TUI's blocked-row logic consume the served fields instead.

## Acceptance

- One implementation of Ready/bucket predicates, in internal/core; cmd contains no dependency/escalation predicate logic (grep for dep-status comparisons in cmd proves it).
- TUI and CLI output unchanged (apart from the vocabulary task's word changes, if that lands first) — including the TUI's settled rule that escalation-only-blocked tasks render no BLOCKED row.
- Core's table-driven tests remain the single home of the rules; daemon tests cover the new stateTask fields.
- `make test lint` green from repo root.

## Constraints

- Do not change what Ready means or bucket membership on any surface — this is consolidation, not redesign.
- The N+1 hydration in the CLI may shrink as a side effect (waitingOn data arriving in /v0/state), but keeping it is fine; scope is one-classifier, not performance.

## History

_No activity yet._
