# History view: h opens the done/cancelled shelf in the TUI

`tuh-01KYX7303WN3RSBXXB9CAGZB01`

- **Status:** open — blocked on dependencies
- **Priority:** 0
- **Labels:** `design` `tui` `cli` `product`
- **Depends on:** [`tuh-s8vt`](tuh-01KYXT2KAG7QXZGF1W47E6S8VT.md) (open)
- **Created:** 2026-07-31 23:10 UTC by `brandon`

## Description

Context: The activity-ledger third of the pitch had no surface — done work was a count ("Done 30") and `tuhdoo task <id>` needed a known ID. Grilled 2026-08-01 (unparked early from the v1-era gate by Brandon); this description is the settled design. History is a "what did I build" device for the steering developer, NOT an audit device — agents dig raw events/git for forensics. (Vocabulary reconciled 2026-08-01 with tuh-01KZ0ES83SFH6MKWP82Y2HNTPK: cancelled replaces archived on every display surface; the TUI cancel key is c.)

The ask: A history mode in the TUI. `h` from the top list (both armed dashboard and watch mode) opens it; esc returns to the dashboard; q quits. The mode reuses the dashboard's list machinery wholesale — same scroll, same selection, same click/enter behavior. Two section bars: DONE, then CANCELLED, each reverse-chronological by close time, newest first (recency is the browse axis here, not priority). Rows render with the ready-row anatomy (title, dim labels suffix, edge markers) plus a dim close stamp and closing actor appended, e.g. `· 2026-07-30 · brandon/claude-code-1`. Enter opens the existing task view (modeDetail) on the row's task; esc from there returns to the history list, not the dashboard.

Task-view additions (both entry paths, no history-only variant):
- The status field line of a terminal task shows close metadata: `done — finished 2026-07-30 by <actor>` / `cancelled — <stamp> by <actor>`.
- Unanswered escalations on terminal tasks render in the History section instead of vanishing (the NEEDS INPUT section only serves open ones); on a finished task an unanswered escalation is part of the record.
- No other augmentation: description + runs + notes + escalations already carry the spirit of the task as built. Explicitly rejected at the grill: a per-task raw-event timeline (edits, status twists, claim fates) — not this device.

Replay/state: core.Task grows ClosedAt/ClosedBy (in-memory only — no stored-byte changes, T3), set by the status-change event that enters done/cancelled, cleared if status leaves terminal; deterministic in ULID order. Fallback: a task created directly in a terminal status (B12 migration shape) closes at its creation event's time/actor. The TUI snapshot payload (stateTask + hydration) carries the new fields; no new endpoints, no write verbs — this feature is read-only.

Data source (settled): pure view over replayed live-tree state, the T1 shape. The epoch-compaction task (t-01KYRMFV10W1N28TCN62F6FRTH) inherits the continuity constraint — its grill decides what survives snapshots for this view; do not solve that here.

Out of scope (deliberate): filters (principal/label/time), an un-cancel TUI key (agents/CLI resurrect via update --status; capture a task if dogfooding wants it), event-level activity feed, pagination.

Acceptance: Interaction tests: h opens history from nav and watch; esc stack (history→dashboard, detail→history); enter/click open detail identically to dashboard; scroll matches dashboard behavior; c/p dead on terminal tasks in detail. Golden tests: two-bar layout, reverse-chron order within bars, row stamp+actor suffix, terminal status line with close metadata, unanswered escalation of a terminal task shown in History. Replay table-tests: ClosedAt/ClosedBy from status-change, created-terminal fallback, cleared when status leaves terminal, deterministic across event orderings. Footer legends updated in both modes. make test lint green.

Pointers: cmd/tuhdoo/top.go (buildRows, modeDetail, detailLines, footer legends), render.go (labelSuffix), internal/core/replay.go + state.go (ClosedAt/ClosedBy), internal/daemon/api.go (stateTask, taskJSON).

Constraints: boring Go; no new dependencies; read-only feature — no new daemon write paths; depends_on the focus-ring task (tuh-01KYXT2KAG7QXZGF1W47E6S8VT) purely to serialize the two top.go reworks.

## History

_No activity yet._
