# Backlog

## Ready

| ID | Task | Priority | Labels |
|---|---|---:|---|
| [t-views](tasks/t-views.md) | Render markdown views | 5 | core |
| [t-epic](tasks/t-epic.md) | v0 build-out | 1 |  |

## In progress

| ID | Task | Priority | Claimed by |
|---|---|---:|---|
| [t-daemon](tasks/t-daemon.md) | Daemon skeleton | 4 | `sarah/impl-9` |

## Blocked / waiting

| ID | Task | Priority | Waiting on |
|---|---|---:|---|
| [t-sync](tasks/t-sync.md) | Sync loop \| app-level merge | 2 | depends on [t-views](tasks/t-views.md) |
| [t-flaky](tasks/t-flaky.md) | Fix flaky TestFoo | 8 | escalation: TestFoo depends on wall-clock timing — rewrite or delete? |

## Done

- [t-core](tasks/t-core.md) — Build the replay engine

## Cancelled

- [t-old](tasks/t-old.md) — Spike: evaluate go-git
