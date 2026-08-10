---
title: tuhdoo documentation
description: What tuhdoo is and where the docs start — adopting it on a team, steering a backlog, joining a repo, the agent protocol, workflow recipes, and uninstalling.
---

# tuhdoo documentation

Steering coding agents today is TODO files and vibes: parallel agents trample each other's work, sessions die and take their context with them, and nothing records what actually happened. tuhdoo replaces that with a shared backlog, work queue, and activity ledger that live in a git orphan branch inside the repo they plan — the plan and the code are one organism. One repo, one clone, one history. No server, no vendor, no accounts.

The docs, in reading order:

- [`adopting.md`](adopting.md) — start here: what bringing tuhdoo to a team looks like. One init, teammates join, agents connect, humans steer from a terminal.
- [`steering.md`](steering.md) — the human side of running a backlog: capture, triage, promoting tasks to prompt quality, shaping the dependency graph, answering escalations. With a worked example.
- [`joining.md`](joining.md) — onboarding a new machine to a repo that already uses tuhdoo, plus the branch-protection and CI settings the repo admin sets once.
- [`agent-protocol.md`](agent-protocol.md) — the instruction text a harness loads for agents: connecting, the twelve verbs, the work loop, escalation.
- [`recipes/`](recipes/README.md) — recommended patterns for the code workflow around the backlog; suggestions, never protocol. Start with the [trunk-based PR flow](recipes/trunk-based-pr-flow.md).
- [`uninstall.md`](uninstall.md) — removing tuhdoo from a machine with zero trace, and retiring a team's ledger.
