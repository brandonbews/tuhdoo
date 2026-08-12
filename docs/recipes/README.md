---
title: Workflow recipes
description: Recommended patterns for the code workflow around a tuhdoo backlog, covering branching shape, PR conventions, and landing discipline. They are suggestions to adapt, never protocol.
---

# Workflow recipes

tuhdoo coordinates *who works what*: agents claim tasks, report outcomes,
and escalate questions, and the ledger of all of it lives on a git branch
inside your repo. What tuhdoo deliberately does **not** touch is how the
code itself gets written and merged; the
[agent protocol](../agent-protocol.md)'s step for that is "ordinary git on
ordinary branches", whatever your team does.

That leaves a real question: what *should* the git workflow around an agent
fleet look like? Recipes are our answer: recommended patterns for branching
shape, PR conventions, how work lands, and the repo settings that keep a
fleet honest. The boundary is deliberate:

- **Protocol** ships in the binary and in
  [`agent-protocol.md`](../agent-protocol.md): the claim → work →
  escalate → finish loop, lease semantics, and honest outcomes. Agents must
  follow it; the daemon enforces the parts that can be enforced.
- **Recipes** are suggestions. Adopt one wholesale, adapt it, or ignore it;
  tuhdoo behaves identically either way.

Each recipe ends with a copy-pasteable instructions block for your repo's
agent-instructions file (`CLAUDE.md`, `AGENTS.md`, or whatever your harness
reads).

## The recipes

- [`trunk-based-pr-flow.md`](trunk-based-pr-flow.md): one task, one branch,
  one squash-merged PR into a protected default branch. It is the flow
  tuhdoo's own repository is built with; start here.
