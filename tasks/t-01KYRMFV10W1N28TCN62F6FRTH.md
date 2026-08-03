# Epoch compaction (D9): snapshot event + in-commit deletion

`t-01KYRMFV10W1N28TCN62F6FRTH`

- **Status:** open — blocked on dependencies
- **Priority:** 1
- **Labels:** `go` `storage`
- **Depends on:** [`t-qm7a`](t-01KYRMFV10W1N28TCN5SH4QM7A.md) (on hold)
- **Created:** 2026-07-30 04:28 UTC by `brandon/migrator`

## Description

Context: 001 D9.4 — designed but deliberately unbuilt until a real data branch is big enough to need it (roadmap: sits between v1 and v2; building against synthetic data would be guessing). Gated on the v1 milestone.

The ask: the snapshot event ("full state as of X"), deletion of superseded event files in an ordinary commit, replay starting from the latest snapshot. The open questions in docs/design/open-questions.md Cycle 4 (triggers, who initiates, snapshot verification) must be settled as design-doc revisions before code.

Inherited constraint (history-view grill, 2026-08-01): the TUI history view (tuh-01KYX7303WN3RSBXXB9CAGZB01) is a pure view over replayed live-tree state showing done/archived tasks with their runs, notes, and escalations. Compaction's grill must decide what that view sees across an epoch boundary: either snapshots carry enough (terminal tasks + their close metadata, runs, notes, escalations) for the shelf to stay complete, or the view goes epoch-scoped with an honest "older history: git log on the data branch" boundary rendered in the TUI. Deleting events without answering this silently amputates the history shelf — that outcome is not acceptable as a default.

Acceptance: replay-from-snapshot equals replay-from-full-history on a real branch (identical state); the live tree shrinks; git history retains everything; a compaction commit merges like any other commit (two-peer convergence test).

Constraints: append-only tree *semantics*, not contents (D9); no force-push, ever; T3 fail-safe rules apply to snapshot events like any other event type.

## History

_No activity yet._
