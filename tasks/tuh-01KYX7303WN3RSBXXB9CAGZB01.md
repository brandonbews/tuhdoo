# tuh-01KYX7303WN3RSBXXB9CAGZB01 — History view: surface the activity ledger (TUI pane and/or command)

- Status: on hold — deliberately paused
- Priority: 0
- Labels: `design`, `tui`, `cli`, `product`
- Created: 2026-07-31 23:10 UTC by `brandon`

## Description

Gated: design-shaped — unpark for a grill cycle in the v1 (steering surface) era, and grill together with or immediately before epoch compaction (t-01KYRMFV10W1N28TCN62F6FRTH), because compaction decides what history even survives to be viewed. Do NOT scope or build before that grill; this is the triage decision (2026-07-31), not a plan.

Original inbox question (Brandon, 2026-07-31): do we need or want some kind of history view — in the main TUI dashboard, or a separate command — just to view work that has been done historically?

Triage argument for "yes, eventually": the project's one-line pitch is "a shared backlog, work queue, and activity ledger" — and the activity-ledger third currently has no surface. Today the only window into completed work is `tuhdoo task <id>` if you already know the ID; the backlog view reduces done work to a count ("Done 30"). All the data exists — the data branch is an event log — and a history view is exactly the pure view-generation-over-events shape the deterministic core is built for (T1).

Design questions for the grill:
- Surface: TUI pane/tab vs a `tuhdoo log`-style command vs both (D8 says CLI is the local human's portal; the TUI is the steering surface — history may be browsing, not steering).
- Granularity: task-level ("what got done, by whom, when") vs event-level ("what happened": claims, finishes, escalations, releases, notes).
- Filters and window: by principal, by label, by time range; how far back by default.
- Overlap with the free surface: D3 markdown views + git log already give per-task history on the git host — is that sufficient for the browsing audience, making the local view steering-adjacent only?

The compaction coupling (why the shared grill): D9.4 compaction writes a snapshot event and deletes superseded event files from the live tree; replay starts from the latest snapshot. A history view reading only the live tree goes amnesiac at each epoch boundary unless it deliberately walks git history (which D9 preserves for forensics). The history view's data source — live tree, git archaeology, or snapshot-carried summaries — must be decided with compaction semantics on the table, not after them.

## History

_No activity yet._
