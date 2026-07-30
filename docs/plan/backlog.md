# v0 Backlog — migrated into tuhdoo

**This file is a tombstone.** On 2026-07-30 (B12, the final backlog task) this
backlog was migrated into tuhdoo itself: the plan now lives on the `tuhdoo`
data branch of this repository, managed by the system it built.

## Where the plan lives now

- **Humans:** `tuhdoo status` · `tuhdoo backlog` · `tuhdoo escalations` ·
  `tuhdoo task <id>` · `tuhdoo watch` — or browse the `tuhdoo` branch on the
  git host (its `README.md` is the live overview).
- **Agents:** connect through the stdio shim (`tuhdoo mcp --as <principal>`),
  follow `docs/agent-protocol.md`, and drive work through `claim_next`.
  Development sessions on this repo are driven that way from now on.

## The v0 clock

The v0 definition-of-done clock (one week of tuhdoo managing its own
development, zero manual repair of the data branch — `docs/plan/roadmap.md`)
started **2026-07-30**. The gate is the blocking escalation on the
*"v0 definition of done: the dogfood week holds"* milestone task: answer it
on or after **2026-08-06** from `tuhdoo escalations`.

## History

The markdown era — B1–B12 task definitions, per-session Status blocks, and
the decisions made while building — is preserved in this file's git history
(`git log -p -- docs/plan/backlog.md`).
