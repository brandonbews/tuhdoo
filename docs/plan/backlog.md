# v0 Backlog — migrated into tuhdoo

**This file is a tombstone.** On 2026-07-30 (B12, the final backlog task) this
backlog was migrated into tuhdoo itself: the plan now lives on the `tuhdoo`
data branch of this repository, managed by the system it built.

## Where the plan lives now

- **Humans:** `tuhdoo status` · `tuhdoo backlog` · `tuhdoo escalations` ·
  `tuhdoo task <id>` · bare `tuhdoo` (the TUI; `--watch` for read-only) — or
  browse the `tuhdoo` branch on the git host (its `README.md` is the live
  overview).
- **Agents:** connect through the stdio shim (`tuhdoo mcp --as <principal>`),
  follow `docs/agent-protocol.md`, and drive work through `claim_next`.
  Development sessions on this repo are driven that way from now on.

## The v0 clock *(retired 2026-08-05)*

A week-clock definition of done once ticked here. It was superseded
2026-08-03 by five checkable facts (`docs/plan/roadmap.md`), and the gate
escalation was answered the same day. The escalation-as-fence pattern the
gate used is now the documented wrong fence — "no attempt, no escalation"
(`docs/agent-protocol.md`); parked work is `held`.

## History

The markdown era — B1–B12 task definitions, per-session Status blocks, and
the decisions made while building — is preserved in this file's git history
(`git log -p -- docs/plan/backlog.md`).
