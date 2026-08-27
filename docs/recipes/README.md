---
title: Workflow recipes
description: Recommended patterns for the code workflow around a tuhdoo backlog, covering branching shape, PR conventions, and landing discipline. They are suggestions to adapt, never protocol.
---

# Workflow recipes

This page is for teams choosing how code gets written and merged around a tuhdoo backlog. It explains what recipes are, where the protocol ends and recipes begin, and which recipe to read first.

tuhdoo coordinates *who works what*: agents claim tasks, report outcomes, and escalate questions, and the ledger of all of it lives on the data branch, a git orphan branch inside your repo. What tuhdoo deliberately does **not** touch is how the code itself gets written and merged; the [agent protocol](../agent-protocol.md)'s step for that is "ordinary git on ordinary branches", whatever your team does.

That leaves a real question: what *should* the git workflow around an agent fleet look like? Recipes answer it: recommended patterns for branching shape, pull request (PR) conventions, how work lands, and the repo and host settings that keep a fleet honest. The boundary is deliberate:

- **Protocol** ships in the binary and in [`agent-protocol.md`](../agent-protocol.md): the claim → work → escalate → finish loop, lease semantics, and honest outcomes. Agents must follow it; the daemon enforces the parts that can be enforced.
- **Recipes** are suggestions. Adopt one wholesale, adapt it, or ignore it; tuhdoo behaves identically either way.

Recipes aimed at agents end with a copy-pasteable instructions block for your repo's agent-instructions file (`CLAUDE.md`, `AGENTS.md`, or whatever your harness reads). Recipes aimed at the repo operator are one-time setup.

## The recipes

- [`trunk-based-pr-flow.md`](trunk-based-pr-flow.md): one task, one branch, one squash-merged PR into a protected default branch. It is the flow tuhdoo's own repository is built with; start here.
- [`vercel.md`](vercel.md): stop Vercel from attempting a deployment on every sync of the data branch. One `vercel.json`, committed onto the data branch itself — and why that is the placement that works.
