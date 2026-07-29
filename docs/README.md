# tuhdoo docs

Working documents for the design and development of tuhdoo.

## Layout

- [`design/001-core-design.md`](design/001-core-design.md) — the founding design record: vision, principles, and the eleven decisions from the first design grilling (2026-07-28), with rationale and accepted consequences.
- [`design/002-technology.md`](design/002-technology.md) — the technology decisions from Cycle 2 (2026-07-29): Go daemon, subprocess git plumbing, canonical-JSON event format and the three version contracts, per-repo daemon topology, the ten-verb MCP surface and agent protocol, views, CLI-portal/TUI surfaces, cadence defaults.
- [`design/open-questions.md`](design/open-questions.md) — unresolved questions, organized into candidate grilling cycles (the agent loop in practice is the next one up).
- [`plan/roadmap.md`](plan/roadmap.md) — v0/v1/v2+ phases, each with a definition of done.
- [`plan/backlog.md`](plan/backlog.md) — the v0 build-out as an ordered, dependency-aware task list (B1–B12); the final task migrates it into tuhdoo itself.

New agents: start at the repo-root `CLAUDE.md`, which gives the reading order and project laws.

## Conventions

- Design decisions get captured with their **rationale** and **accepted consequences** — the consequences matter more than the decision, because they're what future-us will be tempted to un-accept without noticing.
- New sessions that revise a decision should edit `001-core-design.md` in place and note the revision, rather than letting the doc drift from reality.
- New standalone explorations get numbered files in `design/` (`002-…`, `003-…`).
