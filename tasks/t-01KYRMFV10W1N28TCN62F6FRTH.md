# Epoch compaction (D9): snapshot event + in-commit deletion

`t-01KYRMFV10W1N28TCN62F6FRTH`

- **Status:** on hold — deliberately paused
- **Priority:** 2
- **Labels:** `go` `storage`
- **Depends on:** [`t-qm7a`](t-01KYRMFV10W1N28TCN5SH4QM7A.md) (done)
- **Created:** 2026-07-30 04:28 UTC by `brandon/migrator`

## Description

Context: 001 D9.4 — designed but deliberately unbuilt until a real data branch is big enough to need it (roadmap: sits between v1 and v2; building against synthetic data would be guessing). Gated on the v1 milestone.

The ask: the snapshot event ("full state as of X"), deletion of superseded event files in an ordinary commit, replay starting from the latest snapshot. Three open questions must be settled as design-doc revisions before code (inlined 2026-08-06 from the migrated open-questions capture, tuh-01KZA0VT234XJYVZWT95JM25KW, cancelled as folded-in — the doc it pointed at is tombstoned): (1) triggers — when does compaction run: milestone close, event-count/age threshold, or human command only; (2) initiator — who runs it: a daemon automatic, a CLI verb, or human-only, and what happens when two machines both try (the compaction commit must merge or lose cleanly, never fork state); (3) snapshot verification — how the snapshot is proven equivalent before deletions land: replay-from-snapshot must equal replay-from-full-history byte-identically, checked at write time, not trusted.

Inherited constraint (history-view grill, 2026-08-01): the TUI history view (tuh-01KYX7303WN3RSBXXB9CAGZB01) is a pure view over replayed live-tree state showing done/archived tasks with their runs, notes, and escalations. Compaction's grill must decide what that view sees across an epoch boundary: either snapshots carry enough (terminal tasks + their close metadata, runs, notes, escalations) for the shelf to stay complete, or the view goes epoch-scoped with an honest "older history: git log on the data branch" boundary rendered in the TUI. Deleting events without answering this silently amputates the history shelf — that outcome is not acceptable as a default.

Acceptance: replay-from-snapshot equals replay-from-full-history on a real branch (identical state); the live tree shrinks; git history retains everything; a compaction commit merges like any other commit (two-peer convergence test).

Constraints: append-only tree *semantics*, not contents (D9); no force-push, ever; T3 fail-safe rules apply to snapshot events like any other event type.

## History

### 2026-08-01 06:11 UTC — edit by `brandon/claude-code-1`

description edited

### 2026-08-06 22:00 UTC — edit by `brandon/claude-code-1`

description edited

### 2026-08-12 21:32 UTC — edit by `brandon/claude-code-1`

status open→held

### 2026-08-12 21:32 UTC — note from `brandon/claude-code-1`

2026-08-12, Brandon: the v1 milestone (t-01KYRMFV10W1N28TCN5SH4QM7A) was closed today, which satisfies this task's dependency edge — but Brandon parked this HELD deliberately instead of letting it go ready. Unpark on real repo-size pressure (data-branch weight actually hurting), not because the milestone label cleared. The retirement grill (tuh-01KZA0VT234XJYVZWT980V7K2Y) should still run together with or just before this one when it unparks — that pairing decision stands.

### 2026-08-21 23:59 UTC — edit by `brandon`

priority 1→2
