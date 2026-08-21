# Backlog

1 in progress · 2 ready · 5 blocked · 2 on hold · 1 inbox · 1 done · 1 cancelled

**[2 open questions](escalations.md) are waiting on a human.**

## In progress

| ID | Task | Priority | Claimed by |
|---|---|---:|---|
| [`tuh-dmn4`](tasks/tuh-01KYRMFV10W1N28TCN5NDADMN4.md) | Daemon skeleton | 4 | `sarah/impl-9` |

## Ready

| ID | Task | Priority | Labels |
|---|---|---:|---|
| [`t-epic`](tasks/t-epic.md) | v0 build-out | 1 |  |
| [`t-view`](tasks/t-view.md) | Render markdown views | 5 | `core` |

## Blocked / waiting

| ID | Task | Priority | Waiting on |
|---|---|---:|---|
| [`t-flak`](tasks/t-flak.md) | Fix flaky TestFoo | 8 | an [open question](escalations.md) |
| [`t-rout`](tasks/t-rout.md) | Route claims by label | — | depends on [`t-idea`](tasks/t-idea.md) |
| [`t-lpa`](tasks/t-lpa.md) | Extract the store interface | — | **cyclic** — a human must cut an edge; depends on [`t-lpb`](tasks/t-lpb.md) |
| [`t-lpb`](tasks/t-lpb.md) | Rework store tests on the interface | — | **cyclic** — a human must cut an edge; depends on [`t-lpa`](tasks/t-lpa.md) |
| [`t-onit`](tasks/t-onit.md) | Build on the go-git spike | — | waiting on cancelled [`t-old`](tasks/t-old.md) |

## On hold

Triaged, deliberately paused — never served to agents until reopened.

| ID | Task | Priority | Labels |
|---|---|---:|---|
| [`t-sync`](tasks/t-sync.md) | Sync loop \| app-level merge | 2 |  |
| [`t-web`](tasks/t-web.md) | Browser UI spike (kanban) | 1 | `v2` `web` |

## Inbox

Untriaged captures — promoting one to open means writing it a real (prompt-quality) description first.

- [`t-idea`](tasks/t-idea.md) Idea: label-based claim routing

## Done

- [`t-core`](tasks/t-core.md) Build the replay engine

## Cancelled

- [`t-old`](tasks/t-old.md) Spike: evaluate go-git
