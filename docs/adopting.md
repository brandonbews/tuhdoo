---
title: Adopting tuhdoo
description: What bringing tuhdoo to a team looks like. One person runs init, teammates join, agents connect over MCP, the team picks a code workflow, and humans steer from the TUI and CLI.
---

# Adopting tuhdoo

This page is for the person bringing tuhdoo to a team. By the end, tuhdoo runs in your repo, teammates can join, agents can connect, and you can steer the backlog from a terminal.

Adoption is small: one person initializes tuhdoo inside the repo, everyone else joins with a clone and one command, agents connect through a one-snippet config, and the team steers from a terminal. There is no server to stand up, no vendor, and no accounts. The entire shared state is the **data branch**: a git orphan branch (named `tuhdoo`) inside the repository itself, synced through the remote you already have. Whoever can write to the repo can plan in it. Clone the repo and the plan comes with it.

## 1. One person runs `tuhdoo init`

From inside the repository, run:

```sh
tuhdoo init
```

The command does three things:

- Creates the data branch.
- Starts the per-repo background daemon: the local process that owns all writes to the data branch and syncs it with the remote.
- Prints the snippet that connects agents (step 3) and the git-host settings (below).

The command is idempotent: running it again at any time is safe. To verify, run `tuhdoo status`: it shows the data branch, `syncing with "origin"`, and a running daemon.

Adoption comes with two one-time settings on the git host, both printed by `tuhdoo init`:

- Exempt the data branch from pull-request and review rules.
- Exclude the data branch from continuous integration (CI) triggers.

Details are in [the repo-admin section of `joining.md`](joining.md#for-the-repo-admin-branch-protection-and-ci).

## 2. Teammates join

Every other machine — a teammate's laptop or your second workstation — joins in three steps: clone the repo, install the binary, and run the same `tuhdoo init`. On a machine that joins, `tuhdoo init` detects the existing data branch on the remote and adopts it rather than minting a new one. [`joining.md`](joining.md) is the walkthrough, including clone shapes and identity setup.

Everyone sees the same backlog, and everything anyone writes is attributed to a **principal**:

- A human acts as a principal derived from their git identity: `sarah@example.com` acts as `sarah`.
- An agent acts as a sub-principal under the human who runs it: `sarah/claude-code-1`.

Every action on the **ledger** — the append-only record of tasks, claims, notes, outcomes, and questions on the data branch — traces to a responsible person.

## 3. Agents connect

Any agent harness that supports the Model Context Protocol (MCP) connects through one snippet, also printed by `tuhdoo init`:

```json
{ "mcpServers": { "tuhdoo": { "command": "tuhdoo", "args": ["mcp"] } } }
```

If tuhdoo came in as an npm devDependency, use `{ "command": "npx", "args": ["tuhdoo", "mcp"] }` instead.

Connected agents follow [`agent-protocol.md`](agent-protocol.md): the instruction text you load into your harness. `tuhdoo protocol` prints it straight from the binary. The protocol defines the loop agents run: claim a task, work it, escalate questions to a human, and finish with an honest outcome. That is the agents' half of the contract. Your half — writing tasks worth claiming and steering the backlog — is [`steering.md`](steering.md).

## 4. Pick a code workflow

tuhdoo coordinates *who works what*. How code gets written and merged stays ordinary git, whatever your team already does. An agent fleet still benefits from a deliberately chosen workflow, and the [workflow recipes](recipes/README.md) are recommended patterns to adopt or adapt. Start with the [trunk-based pull request (PR) flow](recipes/trunk-based-pr-flow.md): one task, one branch, one squash-merged PR. Each recipe ends with a copy-pasteable block for your repo's agent-instructions file.

## 5. Steer

Day to day, the human surfaces are a terminal.

**The TUI.** Bare `tuhdoo` opens the interactive terminal user interface (TUI) for steering: answer escalations in place, reprioritize, pause and cancel tasks, and drill into any task's history. It acts as you. `tuhdoo --watch` is the same screen read-only, safe to leave open beside a working agent.

**The CLI.** The command-line interface (CLI) offers one-shot commands for reads and quick writes, all scriptable:

| Command | What it does |
|---|---|
| `tuhdoo status` | One-screen overview: sync state, counts, active claims. |
| `tuhdoo backlog` | Every task, one aligned line each; grep a state (`ready`, `in-progress`, `blocked`, `on-hold`, `inbox`, `done`, `cancelled`) to filter. |
| `tuhdoo task <id>` | One task in full: its description plus a chronological history of claims, notes, runs, and escalations. |
| `tuhdoo escalations` | Every escalation, one line each, open before answered. |
| `tuhdoo create <title>` | Add a task. `--status inbox` captures cheaply, `--desc -` reads a description from stdin, and flags exist for priority, labels, and dependencies. |
| `tuhdoo update <id>` | Change fields: title, description, priority, status, labels, dependencies. |
| `tuhdoo answer <id> <text>` | Answer an open escalation. |

The work-loop tools (claim, finish, release) are deliberately *not* CLI commands. A claim's lease renews only while a live agent session holds it, so a claim taken by a one-shot command would just lapse. Agents work through `tuhdoo mcp`; humans steer.

**The git host, for free.** The data branch carries generated **views**: browsable markdown (a backlog page and per-task pages) that lets anyone read the plan from GitHub or GitLab without installing anything.

The rhythm that follows is small: capture ideas the moment they occur (`tuhdoo create … --status inbox`), triage the inbox periodically, and keep escalations answered — they are the questions your fleet is blocked on. [`steering.md`](steering.md) covers that loop.

## Leaving

Adoption is reversible. A machine walks away with a handful of ordinary git commands and zero trace, and a team can retire the ledger entirely if it ever wants to. [`uninstall.md`](uninstall.md) is the walkthrough. It's worth reading before you adopt, because it proves adopting costs nothing to undo.
