# tuhdoo

A coordination fabric for agent fleets, steered by humans: a shared backlog, work queue, and activity ledger living in a git orphan branch inside the repo it plans. Synced through an ordinary git remote — no server, no vendor, no accounts.

## State of the project

Design phase complete through two cycles; **no application code exists yet**. The build-out is defined in `docs/plan/backlog.md`. Everything decided so far lives in `docs/` — do not re-litigate settled decisions from scratch; revise them explicitly (see conventions below).

## Read in this order

1. `docs/design/001-core-design.md` — vision, principles, the eleven founding decisions (D1–D11)
2. `docs/design/002-technology.md` — stack and technical contracts (T1–T8)
3. `docs/plan/roadmap.md` — phases and definitions of done
4. `docs/plan/backlog.md` — the actual work queue; pick up from there
5. `docs/design/open-questions.md` — what's unresolved, and which cycle it belongs to

## Project laws (non-negotiable unless a design doc is revised first)

- **No force-push on the data branch, ever** (founding principle; breaks every peer at once).
- **Boring Go only**: no clever concurrency, no channels-of-channels; plain mutexes, plain loops — code a Go-learner can audit (T1).
- **The deterministic core is pure functions**: replay, winner rules, lease expiry, compaction, view generation — data in, data out, no I/O, table-driven tests (T1).
- **Host-agnostic by construction**: tuhdoo speaks the git protocol only; never call a host API from core paths; remoteless operation is a normal state, not an error (T2).
- **Stored event bytes are never rewritten**; schema evolution is additive-first with in-memory upcasters; incomprehensible events trigger fail-safe read-only mode, never best-effort skipping (T3).
- **Fewer, better MCP verbs**: the agent surface is ten tools (T5); additions need a design-doc revision.

## Building backlog tasks

- A task's **Accept** block in `docs/plan/backlog.md` is its definition of done: the tests it describes must exist and pass. Nothing is "complete" until `make test lint` is green from the repo root.
- One commit per task (or per coherent piece), named for it ("B9: …"), pushed after green.
- When fanning work out to sub-agents, point each at: this file, the relevant design-doc sections, and the task's backlog entry — with explicit file/directory boundaries and "do not commit; report back" (the orchestrator reviews and commits). This is the established pattern; it kept eight parallel-built packages conflict-free.
- Review is behavioral and machine-driven: humans judge behavior, protocol semantics, and design drift; line-level scrutiny comes from test suites now and a full `/code-review` pass at the POC milestone — not from human diff-reading.

## Working conventions

- The design was produced through `/grill-me` sessions ("cycles"); decisions carry rationale and **accepted consequences** — the consequences are the part future sessions are tempted to quietly un-accept. Don't.
- Revising a decision = edit the design doc in place with a revision note, the way Cycle 2 amended `001` (see D5/D6/D8 for the pattern).
- The human (Brandon, a TS dev deliberately learning Go) steers by reviewing; write code and explanations accordingly.
- Task descriptions in the backlog are written as prompts with acceptance criteria — the same convention tuhdoo itself will enforce. Keep it when adding tasks.
- Once v0 is dogfoodable, this markdown backlog migrates into tuhdoo itself (final backlog task).
- **Workflow files are the exception to machine-driven review.** Anything under `.github/workflows/` executes unattended with the repo's CI credentials, and no test suite covers it. Any change there must be called out explicitly and separately in the session summary for Brandon's eyes-on diff review — never folded silently into a larger commit.
