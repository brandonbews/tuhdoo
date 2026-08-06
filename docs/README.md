# tuhdoo docs

Working documents for the design and development of tuhdoo.

## Layout

- [`design/001-core-design.md`](design/001-core-design.md) — the founding design record: vision, principles, and the eleven decisions from the first design grilling (2026-07-28), with rationale and accepted consequences.
- [`design/002-technology.md`](design/002-technology.md) — the technology decisions from Cycle 2 (2026-07-29): Go daemon, subprocess git plumbing, canonical-JSON event format and the three version contracts, per-repo daemon topology, the twelve-verb MCP surface (originally ten; relay_answer added 2026-07-30, confirm_claim 2026-08-04) and agent protocol, views, CLI-portal/TUI surfaces, cadence defaults.
- [`design/open-questions.md`](design/open-questions.md) — a tombstone (2026-08-05): what got settled went into the design docs, what stayed open migrated to the tuhdoo ledger. Open questions live on `tuhdoo backlog` (inbox/held) now.
- [`agent-protocol.md`](agent-protocol.md) — the instruction text a harness loads for agents working a tuhdoo project (field-tested since 2026-07-30, heavily revised since — every dogfood session runs it).
- [`joining.md`](joining.md) — onboarding a new machine to a repo that already uses tuhdoo: clone shapes, install, `tuhdoo init`, verification, principal override, and the host branch-protection/CI settings. Self-contained by design; a future leaving/uninstall doc belongs beside it.
- [`plan/roadmap.md`](plan/roadmap.md) — v0/v1/v2+ phases, each with a definition of done.
- [`plan/backlog.md`](plan/backlog.md) — a tombstone: the B1–B12 build-out was migrated into tuhdoo itself at B12 (2026-07-30); the live queue is `tuhdoo backlog`.

New agents: start at the repo-root `CLAUDE.md`, which gives the reading order and project laws.

## Conventions

- Design decisions get captured with their **rationale** and **accepted consequences** — the consequences matter more than the decision, because they're what future-us will be tempted to un-accept without noticing.
- New sessions that revise a decision should edit `001-core-design.md` in place and note the revision, rather than letting the doc drift from reality.
- New standalone explorations get numbered files in `design/` (`002-…`, `003-…`).
- Prospective cycle numbering is retired: grills happen when a question is ripe, not on a schedule, and are cited by topic and date ("the milestone grill, 2026-08-03"), not by a planned cycle number. "Cycle 2"/"Cycle 4" survive in the docs as the historical names of the grills that used them.
