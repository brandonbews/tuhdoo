# D9 epoch compaction is on a collision course with the events union merge

`tuh-01KZVZT7F8CVJYX1P003BNA3A9`

- **Status:** inbox — untriaged capture
- **Priority:** 0
- **Labels:** `audit-finding`
- **Created:** 2026-08-12 21:59 UTC by `brandon/claude-code-bg`

## Description

Go-sweep audit finding. syncer/merge.go ~32-35: 'a file present on only one side always survives; for events that is correctness (append-only)'. D9 clause 4 designs compaction that deletes superseded event files in an ordinary commit and promises 'the commit merges like any other' — under the current union rule, merging a compacted head with any pre-compaction head resurrects every deleted event file. Nothing is wrong today (compaction unbuilt; Batch.Delete had no callers and was removed by the sweep), but when compaction lands, merge must learn snapshot awareness or D9's promise fails. Reconcile at design level before building compaction.

## History

_No activity yet._
