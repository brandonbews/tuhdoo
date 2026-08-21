---
title: Steering a backlog
description: The human side of running tuhdoo. Capture ideas, triage the inbox, promote tasks to prompt quality, decompose work into a dependency graph, and steer an agent fleet day to day.
---

# Steering a backlog

tuhdoo splits running a project in two: agents execute, humans decide.
Agents take tasks from a shared queue, build, and report; you set intent,
order the queue, answer questions, and judge outcomes. This page is the
human half of that contract: the lifecycle an idea moves through from the
moment it occurs to you until an agent lands it, and the levers you hold at
each step. (The agent half is [`agent-protocol.md`](agent-protocol.md);
team setup is [`adopting.md`](adopting.md).)

A few terms, used throughout:

- A **task** is a unit of intent: title, description, priority, status,
  labels, and dependency edges to other tasks.
- A **claim** is an exclusive, time-boxed **lease** on a task. An agent
  claims before working; the lease renews automatically while the agent's
  session is alive and lapses when it dies, returning the task to the pool.
  No task is ever silently stuck with a crashed agent.
- An **escalation** is a question an agent routes to a human. A *blocking*
  escalation also fences its task out of the queue until you answer.
- The **ledger** is the append-only record of all of it: every task edit,
  claim, note, outcome, and question is a typed event, attributed to whoever
  wrote it. It lives on the **data branch**, a git orphan branch (named
  `tuhdoo`) inside your repo, synced through your ordinary git remote.
  There is no server and there are no accounts; clone the repo and the
  whole plan comes with it.

## The lifecycle at a glance

A task is always in exactly one of five statuses:

| Status | Meaning |
|---|---|
| `inbox` | Captured, never triaged. Title-only is legitimate. |
| `held` | Passed triage, deliberately paused: "yes, but not now". |
| `open` | Commissioned. The only status agents are ever served from. |
| `done` | Finished, with the run record saying how. |
| `cancelled` | Closed without doing it. A human call; never a deletion. |

An `open` task is further displayed as **ready** (claimable now),
**in-progress** (someone holds the claim), or **blocked** (waiting on an
unfinished dependency or an unanswered blocking escalation). Those three
are computed from the task's situation, not stored, and they are what
`tuhdoo backlog` and the TUI show.

The rest of this page is the flow through those statuses: **capture →
triage → promote → decompose → steer**.

## 1. Capture: get the idea out of your head

Ideas land in `inbox` the moment they occur, at zero scoping cost:

```sh
tuhdoo create "sweep for duplicate code" --status inbox
```

Title-only is legitimate; a fragment description is legitimate. Do not stop
to scope, price, or write acceptance criteria: capture must not cost a
planning session, because an uncaptured idea dies with the context it
occurred in.

Agents capture too, with the same bar: when one notices a refactor or steps
around a bug mid-task, the protocol tells it to file an inbox item rather
than lose the observation. The inbox is the team's shared pile of
not-yet-decisions, from every person and every session.

## 2. Triage: review the inbox, periodically

Every so often, at the end of a session or the start of a planning block,
walk the inbox and give each item whatever rigor it deserves:

- **Cancel** what turned out to be nothing. Cancelling is honest and cheap;
  the record stays on the ledger, and nothing is ever hard-deleted.
- **Park as `held`** what is real but not now. The item passed triage and
  is workable; a human decided to pause it. It sits visibly in the backlog
  without pretending to be claimable.
- **Promote** what should be built, which is where the real bar applies.

Pause and resume are one field: `tuhdoo update <id> --status held` and back
to `--status open`. From the TUI (bare `tuhdoo`), all of this is
cursor-and-keystroke on the inbox section.

## 3. Promote: the description is the prompt

Promotion (`inbox` → `open`) is the moment an idea becomes a commission,
and it has a quality bar: **a prompt-quality description in the five parts
the protocol's
[descriptions-are-prompts section](agent-protocol.md#writing-tasks-descriptions-are-prompts)
defines** (context, the ask, acceptance criteria, pointers, constraints).
One convention serves both audiences: you write it, and an agent works
from it.

Task quality bounds output quality: an agent builds exactly what the
description asks, because it was not in the meeting and its session starts
cold with the description as its brief. Nothing enforces the bar
mechanically, but the protocol closes the loop: a claimant handed an
unscoped task will correctly raise a blocking escalation asking for the
missing criteria, and you will end up writing the description anyway,
later, with an agent parked on it. Write it at promotion time instead.

```sh
tuhdoo update tuh-d83w --status open --desc -   # reads the description from stdin
```

## 4. Decompose: the DAG is the plan

A task too big for one run becomes a **container**: a task that depends on
its children. Children are created in one atomic batch with the dependency
edges between them (the whole subgraph lands or none of it does), and the
container's `depends_on` points at them, so it stays blocked until every
child is done.

Dependency edges are the whole ordering mechanism. A task is **ready** only
when everything it depends on is `done`: not cancelled, not held, done.
The resulting directed acyclic graph, not a sprint plan or a milestone
date, orders the fleet's work: agents drain whatever is ready, and
readiness propagates through the graph as work lands. You never schedule;
you shape the graph.

Add or remove edges with `tuhdoo update <id> --depends-on <ids>` (the list
replaces in full), or from the TUI. Agents decompose mid-task through the
protocol when they discover the task they hold is really five.

## 5. Steer: the ongoing part

Everything above happens per-idea. Steering proper is continuous and
deliberately small: a handful of levers, all available from bare `tuhdoo`
(the interactive TUI) or the one-shot CLI.

- **Priorities.** A single number per task; P0 is highest — `0` is the
  most urgent, larger numbers matter less, and a task with no priority
  set waits behind every prioritized one. Agents take the most urgent
  ready task, oldest first within a rank. Reordering the queue is
  editing numbers: `tuhdoo update <id> --priority 3`.
- **Dependencies.** Add an edge to sequence work, remove one to unblock it.
  Readiness recomputes immediately.
- **Pause and resume.** `held` and back, per task, any time.
- **Answer escalations.** Escalations are your inbox, and each one is
  written to be answerable on its own: the question carries the options the
  agent saw and its recommendation, with background in the context field.
  Answer from the TUI (select, type), or:

  ```sh
  tuhdoo escalations                 # every escalation, open before answered
  tuhdoo answer <id> Use approach B; keep the flag name.
  ```

  Answering a blocking escalation returns its task to the pool; the next
  claimant picks up the question and your answer together. You do not need
  to answer while the asking agent is alive: the protocol is built for
  succession, and usually the answer arrives after that session is gone.
- **Cancel.** A human call, always. Agents never cancel unbidden, and a
  cancelled task keeps its full history on the ledger.
- **Read the ledger.** `tuhdoo task <id>` shows one task with its
  chronological history: who claimed it and when, notes left mid-flight,
  how each run ended, what was asked and answered. `tuhdoo backlog` is the
  whole queue, one line per task. Because every entry is a typed,
  attributed event, catching up after time away is a read of the ledger,
  not a reconstruction. The data branch also renders as browsable markdown
  on your git host for free.

## A worked example: launching tuhdoo.com

This is how tuhdoo's own website shipped, exactly as its ledger records it.

**Capture.** Over about a week, ideas landed as cheap captures:
*"marketing/docs site for tuhdoo"*, *"workflow recipe docs"*, *"ship the
agent protocol with the binary"*, *"user-facing docs"*. Some were
title-only inbox items; the site capture carried one early decision (build
it inside the main repo) and was parked `held`: real, but not yet.

**Triage.** A planning session walked the pile and noticed that the site,
the docs, and the recipes all hung on the same unsettled decisions:
framework, hosting, one site or two, the domain, and where the docs content
should live. Rather than promote anything half-decided, triage created a
decision task (a human-led discussion) and made the build items depend on
it.

**Decide, then promote.** The discussion settled everything: Next.js,
hosting that watches the repo directly, one combined marketing-and-docs
site, the already-owned domain, and docs living in the repo's `docs/`
directory as plain markdown the site consumes at build time. Each build
item was then promoted with a full five-part description recording the
decisions, so every future claimant inherits the *why*, not just the
*what*.

**Decompose.** A container task, *"Launch tuhdoo"*, was created depending
on all of it: the decision task, the site build, the docs, the recipes,
shipping the protocol with the binary, a docs restructuring, and DNS
wiring. The edges encoded the real ordering: the restructuring had to land
before the site could consume the docs, and the DNS wiring depended on the
site landing first. The DNS wiring was also a human-owned task; not
everything in a backlog is agent work.

**Drain.** Agents then worked the ready frontier one task at a time:
claim → branch → pull request → finish, each run recording its PR and the
commit that landed. One task blocked instead of landing: its claimant hit
two genuinely open questions and did what the protocol prescribes. It
raised one blocking escalation carrying both questions with
recommendations, noted where work stopped, released the claim, and finished
the run as blocked. That task now waits, out of the queue, in the human's
escalation inbox: a question owed an answer, not a mystery.

The container stays open until the human declares the launch done; some
calls are judgment, and the ledger records them as such. The page you are
reading was itself one of those children: captured, triaged, promoted,
claimed, and landed through this exact loop.

## Where next

- [`adopting.md`](adopting.md) covers bringing tuhdoo to a team: init,
  joining, connecting agents, and picking a code workflow.
- [`agent-protocol.md`](agent-protocol.md) is the other half of the
  contract: what agents are instructed to do with the tasks you write.
- [`recipes/`](recipes/README.md) collects recommended shapes for the code
  workflow around the backlog.
