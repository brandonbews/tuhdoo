# Working-set retirement: bounding what surfaces show without deleting history

`tuh-01KZA0VT234XJYVZWT980V7K2Y`

- **Status:** on hold — deliberately paused
- **Priority:** 0
- **Labels:** `design` `ledger`
- **Created:** 2026-08-05 22:31 UTC by `brandon/claude-code-1`

## Description

Gated: unpark for a grill cycle no later than when the epoch-compaction task (t-01KYRMFV10W1N28TCN62F6FRTH) unparks — i.e. at v1 milestone declaration — because the two grills share the snapshot/history-shelf question and should run together or back-to-back (retirement first informs compaction: retired tasks are natural snapshot-elision candidates). Unpark EARLIER if pressure bites: surface noise starts annoying the human, or a short-ID tail collision actually occurs. Held at the 2026-08-06 triage grill (Brandon).

Migrated from open-questions.md (grill 2026-08-02), 2026-08-05 sweep — the migration kept only a fragment; the full grill record below was recovered from git history (commit 858fce8, PR #17) so the future grill never needs archaeology.

SETTLED at the 2026-08-02 grill (don't re-derive): a way for old terminal tasks to leave the live working set — an ordinary appended event, no file moves, no deleted history, consciously recoverable (a flag on read surfaces, or an un-retire event). NOT a revival of the status word "archived" (retired 2026-08-01), and distinct from D9 epoch compaction (repo-size hygiene). Two motivations: (1) TUI/views/resolver stop growing without bound; (2) the short-ID fragment-resolution pool stays bounded at working-set size, keeping 4-char tails effectively unique at any scale (T7 accepted consequence 2026-08-02: ~38% odds some pair shares a tail by ~1,000 tasks, and this question is the named bound). Resolution must stay loud: a fragment matching one live task and retired ones resolves to the live task but SAYS SO — silently resolving an old short-ID reference (in a PR body or branch name) to a newer tail-colliding task is the worst failure mode. Composes with D9 without being coupled to it.

Pressure snapshot 2026-08-06 (one week of dogfood): 104 tasks, 83 closed (65 done + 18 cancelled) — 80% of the ledger. No cap, window, or pagination exists on any surface: tuhdoo backlog prints all 104 rows; backlog.md is roughly half closed sections; the TUI history shelf (h) is one 83-row list; get_backlog scope done returns all 65. Two costs grow with TOTAL task count forever and D9 compaction fixes neither (it deletes events/ files; replayed state — which every surface reads — is unchanged by design): every single-event commit rewrites ~108 view files because each closed task keeps its tasks/<id>.md regenerated; and the TUI hydrates every task individually over the socket each 2-second tick (~104 round-trips). Replay itself is NOT the pressure (bench: ~2.5µs/event, linear — see internal/core/replay_bench_test.go).

Design landmines for the grill, verified 2026-08-06:
- Format: a new event type (task.retired or whatever wins naming) is the T3-sanctioned shape — additive, no version bump, no upcaster; but un-upgraded peers go fail-safe read-only on first contact ("upgrade tuhdoo"). Accepted precedent: claim.confirmed (catalog.go notes, 001 D6 consequences) — upgrade every machine before it ships events in a mixed fleet.
- Naming is double-fenced: not "archived" (2026-08-01 one-vocabulary decision: it collided with done — "both terminal, both kept forever" — which is exactly the property retirement breaks, so the word must not suggest a sixth status either).
- Write surface must respect T5's twelve verbs: riding update_task vs a new verb is a grill decision; a new verb needs a design-doc revision.
- The dependency annotation is load-bearing: taskRef (cmd/tuhdoo/snapshot.go) resolves closed tasks so edges render "depends on t-xxxx (done — title)" instead of dangling; retired tasks must stay resolvable for this.
- If rendering changes, the view-format version bumps (T6, highest-wins, cosmetic-only).
- Compaction's inherited constraint (history-view grill 2026-08-01, carried on t-01KYRMFV10W1N28TCN62F6FRTH) — what the history shelf sees across an epoch boundary — is the shared question; deleting events without answering it "silently amputates the history shelf".

Cross-links: t-01KYRMFV10W1N28TCN62F6FRTH (epoch compaction build task, blocked on v1 milestone t-01KYRMFV10W1N28TCN5SH4QM7A); 002 T7 short-IDs accepted consequence; tuh-01KZ4PY7QJEAZ6T8R1V046G9DT (task-view history rendering — adjacent TUI surface, unrelated mechanism).

## History

### 2026-08-06 23:04 UTC — edit by `brandon/claude-code-1`

description edited · status inbox→held

### 2026-08-12 21:32 UTC — note from `brandon/claude-code-1`

2026-08-12: this capture's unpark condition said "no later than when epoch compaction unparks — i.e. at v1 milestone declaration." The v1 milestone task was closed today (as road-sign retirement, not a phase declaration), and Brandon deliberately parked epoch compaction held anyway. So this capture's unpark condition is consciously overridden: it stays held, paired with compaction — both unpark on real pressure (surface noise annoying the human, a short-ID tail collision, data-branch weight), not on the milestone label. The pressure signals listed in this description remain the trigger list.
