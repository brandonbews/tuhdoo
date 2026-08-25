---
title: tuhdoo documentation
description: An overview of tuhdoo and the reading order for its docs, from adopting it on a team through steering, joining, the agent protocol, workflow recipes, and uninstalling.
---

# tuhdoo documentation

Steering coding agents with TODO files breaks at fleet scale: parallel agents trample each other's work, sessions die and take their context with them, and nothing records what actually happened. tuhdoo replaces the TODO file with a shared backlog, work queue, and activity ledger, stored on a git orphan branch inside the repo it plans. The plan travels with the code: one repo, one clone, one history. tuhdoo syncs over the remote you already have and needs no server, no vendor, and no accounts.

The docs, in reading order:

- [`adopting.md`](adopting.md) is the place to start: what bringing tuhdoo to a team looks like. One person initializes it, teammates join, agents connect, and humans steer from a terminal.
- [`steering.md`](steering.md) covers the human side of running a backlog: capturing ideas, triaging, promoting tasks to prompt quality, shaping the dependency graph, and answering escalations. It closes with a worked example.
- [`joining.md`](joining.md) walks a new machine through joining a repo that already uses tuhdoo, and gives the repo admin the branch-protection and CI settings to set once.
- [`agent-protocol.md`](agent-protocol.md) is the instruction text a harness loads for agents: connecting, the twelve tools, the work loop, and escalation.
- [`recipes/`](recipes/README.md) collects recommended patterns for the code workflow and host settings around the backlog; they are suggestions, never protocol. Start with the [trunk-based PR flow](recipes/trunk-based-pr-flow.md).
- [`uninstall.md`](uninstall.md) removes tuhdoo from a machine with zero trace, and retires a team's ledger.
