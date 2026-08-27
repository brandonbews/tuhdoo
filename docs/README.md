---
title: tuhdoo documentation
description: An overview of tuhdoo and the reading order for its docs, from adopting it on a team through steering, joining, the agent protocol, workflow recipes, and uninstalling.
---

# tuhdoo documentation

These docs are for teams that steer coding agents with tuhdoo, and for the agents themselves. They take you from adopting tuhdoo on a team through day-to-day steering, and they end with how to remove it.

## What tuhdoo is

Steering coding agents with TODO files breaks at fleet scale. Parallel agents trample each other's work. Sessions die and take their context with them. Nothing records what actually happened.

tuhdoo replaces the TODO file with a shared backlog and an activity ledger. Both live on the **data branch**: a git orphan branch inside the repo it plans. The plan travels with the code, so one repo means one clone and one history. tuhdoo syncs through the git remote you already have. It needs no server, no vendor, and no accounts.

## Reading order

Read the docs in this order:

1. [Adopting tuhdoo](adopting.md) — start here. One person initializes tuhdoo, teammates join, agents connect, and humans steer from a terminal.
2. [Steering a backlog](steering.md) — the human side of running a backlog: capture ideas, triage the inbox, promote tasks to prompt quality, shape the dependency graph, and answer escalations. It closes with a worked example.
3. [Joining an existing tuhdoo repo](joining.md) — how a new machine joins a repo that already uses tuhdoo, plus the branch-protection and continuous integration (CI) settings the repo admin sets once.
4. [tuhdoo agent protocol](agent-protocol.md) — the instruction text an agent harness loads: connecting, the twelve tools, the work loop, and escalation.
5. [Workflow recipes](recipes/README.md) — recommended patterns for the code workflow and host settings around the backlog. Recipes are suggestions, never protocol. Start with the [trunk-based pull request (PR) flow](recipes/trunk-based-pr-flow.md).
6. [Uninstalling tuhdoo](uninstall.md) — how to remove tuhdoo from a machine with zero trace, and how a team retires its ledger.
