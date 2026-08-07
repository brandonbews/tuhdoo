---
title: Workflow recipes
description: Recommended patterns for the code workflow around a tuhdoo backlog — branching shape, PR conventions, landing discipline. Suggestions to adapt, never protocol.
---

# Workflow recipes

tuhdoo coordinates *who works what*: agents claim tasks, report outcomes, and
escalate questions through a small MCP surface, and the ledger of all of it
lives on a git branch inside your repo. What tuhdoo deliberately does **not**
touch is how the code itself gets written and merged — the
[agent protocol](../agent-protocol.md)'s step for that is "ordinary git on
ordinary branches", and it stays that way no matter what your team does.

That leaves a real question unanswered: what *should* the git workflow around
an agent fleet look like? Recipes are our answer — recommended patterns for
the outer workflow: branching shape, PR conventions, how work lands, and the
repo settings that keep a fleet honest. The boundary is deliberate:

- **Protocol** ships in the tool and in [`agent-protocol.md`](../agent-protocol.md) —
  the claim → work → escalate → finish loop, lease semantics, honest
  outcomes. Agents must follow it; the daemon enforces the parts that can be
  enforced.
- **Recipes** are suggestions. Adopt one wholesale, adapt it, or ignore it —
  tuhdoo behaves identically either way.

Each recipe ends with a copy-pasteable instructions block for your repo's
agent-instructions file (`CLAUDE.md`, `AGENTS.md`, or whatever your harness
reads), ready to adapt.

## The recipes

- [`trunk-based-pr-flow.md`](trunk-based-pr-flow.md) — one task, one branch,
  one squash-merged PR into a protected default branch. The flow tuhdoo's own
  repository is built with; start here.
