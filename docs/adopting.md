---
title: Adopting tuhdoo
description: What bringing tuhdoo to a team looks like — one init, teammates join, agents connect over MCP, a code workflow gets picked, and humans steer from the TUI and CLI.
---

# Adopting tuhdoo

Bringing tuhdoo to a team is small: one person initializes it inside the
repo, everyone else joins with a clone and one command, agents connect
through a one-snippet MCP config, and the team steers from a terminal.
There is no server to stand up, no vendor, no accounts — the entire shared
state is the **data branch**, a git orphan branch (named `tuhdoo`) inside
the repository itself, synced through the remote you already have. Whoever
can write to the repo can plan in it; clone the repo and the plan comes
with it.

## 1. One person runs `tuhdoo init`

From inside the repository, once:

```sh
tuhdoo init
```

This creates the data branch, starts the per-repo background daemon (the
local process that owns all writes to the branch and syncs it with the
remote), and prints the MCP snippet for connecting agents. It is idempotent
— safe to run again anytime.

Adoption comes with two one-time settings on the git host, both printed by
`init`: exempt the data branch from pull-request/review rules, and exclude
it from CI triggers. Details in
[the repo-admin section of `joining.md`](joining.md#for-the-repo-admin-branch-protection-and-ci).

## 2. Teammates join

Every other machine — a teammate's laptop, your second workstation — joins
with a clone, a binary install, and the same `tuhdoo init`, which detects
the existing data branch on the remote and adopts it rather than minting a
new one. The walkthrough, including clone shapes and identity setup, is
[`joining.md`](joining.md).

Everyone sees the same backlog, and everything anyone writes is attributed:
humans act as a principal derived from their git identity
(`sarah@example.com` acts as `sarah`), and every agent acts as a
sub-principal under the human who runs it (`sarah/claude-code-1`). Every
action on the ledger traces to a responsible person.

## 3. Agents connect

Any MCP-capable agent harness connects through one snippet (also printed by
`init`):

```json
{ "mcpServers": { "tuhdoo": { "command": "tuhdoo", "args": ["mcp"] } } }
```

(If tuhdoo came in as an npm devDependency:
`{ "command": "npx", "args": ["tuhdoo", "mcp"] }`.)

Connected agents follow [`agent-protocol.md`](agent-protocol.md) — the
instruction text you load into your harness (`tuhdoo protocol` prints it
straight from the binary), defining the loop they run: claim a task, work
it, escalate questions to a human, finish with an honest outcome. That is
the agents' half of the contract; your half — writing tasks worth claiming
and steering the queue — is [`steering.md`](steering.md).

## 4. Pick a code workflow

tuhdoo coordinates *who works what*; how code gets written and merged stays
ordinary git, whatever your team already does. But an agent fleet is better
with a deliberately chosen workflow, and the
[workflow recipes](recipes/README.md) are recommended patterns to adopt or
adapt. Start with the [trunk-based PR flow](recipes/trunk-based-pr-flow.md)
— one task, one branch, one squash-merged PR. Each recipe ends with a
copy-pasteable block for your repo's agent-instructions file.

## 5. Steer

Day to day, the human surfaces are a terminal.

**The TUI.** Bare `tuhdoo` opens the interactive steering screen: answer
escalations in place, reprioritize, pause and cancel tasks, drill into any
task's history. It acts as you. `tuhdoo --watch` is the same screen
read-only — safe to leave open beside a working agent.

**The CLI.** One-shot commands for reads and quick writes, all scriptable:

| Command | What it does |
|---|---|
| `tuhdoo status` | One-screen overview: sync state, counts, active claims. |
| `tuhdoo backlog` | Every task, one aligned line each — grep a state (`ready`, `in-progress`, `blocked`, `on-hold`, `inbox`, `done`, `cancelled`) to filter. |
| `tuhdoo task <id>` | One task fully hydrated, with its chronological history. |
| `tuhdoo escalations` | Every escalation, one line each, open before answered. |
| `tuhdoo create <title>` | Add a task — `--status inbox` for cheap capture, `--desc -` to read a description from stdin, plus priority, labels, dependencies. |
| `tuhdoo update <id>` | Change fields: title, description, priority, status, labels, dependencies. |
| `tuhdoo answer <id> <text>` | Answer an open escalation. |

The work-loop verbs (claim, finish, release) are deliberately *not* CLI
commands: a claim's lease renews only while a live agent session holds it,
so a claim taken by a one-shot command would just lapse. Agents work
through `tuhdoo mcp`; humans steer.

**The git host, for free.** The data branch renders as browsable markdown —
backlog and per-task pages — so anyone can read the plan from GitHub or
GitLab without installing anything.

The rhythm that follows: capture ideas the moment they occur
(`tuhdoo create … --status inbox`), triage the inbox periodically, and keep
escalations answered — they are the questions your fleet is blocked on.
That loop is [`steering.md`](steering.md).

## Leaving

Adoption is reversible: a machine walks away with a handful of ordinary git
commands and zero trace, and a team can retire the ledger entirely if it
ever wants to. [`uninstall.md`](uninstall.md) is the walkthrough — worth
knowing before you adopt, because it proves adopting costs nothing to undo.
