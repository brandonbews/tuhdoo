# D9 epoch compaction is on a collision course with the events union merge

`tuh-01KZVZT7F8CVJYX1P003BNA3A9`

- **Status:** cancelled
- **Priority:** none
- **Labels:** `audit-finding`
- **Created:** 2026-08-12 21:59 UTC by `brandon/claude-code-bg`

## Description

Cancelled 2026-08-27 at the audit-findings triage (Brandon-delegated): folded into the epoch-compaction task t-01KYRMFV10W1N28TCN62F6FRTH as its open question (4), joining the three design questions already gating that build — same fold-in pattern as the 2026-08-06 open-questions migration. Re-verified before folding: merge.go:26-31 still unions (a file on only one side always survives) and nothing snapshot-aware exists anywhere in internal/ or cmd/.

Original capture: Go-sweep audit finding. syncer/merge.go ~32-35: 'a file present on only one side always survives; for events that is correctness (append-only)'. D9 clause 4 designs compaction that deletes superseded event files in an ordinary commit and promises 'the commit merges like any other' — under the current union rule, merging a compacted head with any pre-compaction head resurrects every deleted event file. Nothing is wrong today (compaction unbuilt; Batch.Delete had no callers and was removed by the sweep), but when compaction lands, merge must learn snapshot awareness or D9's promise fails. Reconcile at design level before building compaction.

## History

### 2026-08-27 07:06 UTC — edit by `brandon/claude-code-1`

description edited · status inbox→cancelled
