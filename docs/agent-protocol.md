---
title: tuhdoo agent protocol
description: The instruction text a harness loads for agents working a tuhdoo-managed project, covering connecting, the twelve tools, the work loop, escalation, and what an agent never does.
---

# tuhdoo agent protocol

This is your half of the contract for working a tuhdoo-managed project; the daemon's half (leases, serialization, sync) is automatic. The ledger is agent memory before it is human audit trail: sessions end and contexts compact, and the only continuity between you and your successor is what landed on the ledger. If this doc and a `tools/list` response ever disagree, the daemon is right.

## Connecting

Connect through the stdio shim only, from inside the repository:

```json
{ "mcpServers": { "tuhdoo": { "command": "tuhdoo", "args": ["mcp"] } } }
```

As an npm devDependency, `{ "command": "npx", "args": ["tuhdoo", "mcp"] }` works unchanged; the npm launcher is a transparent pass-through. The shim bridges stdio to the daemon over a repo-local unix socket and auto-spawns the daemon if it isn't running.

- With no flags, your principal is auto-derived at connect: the human half is the local part of `git config user.email`, and the daemon mints the agent half from your harness's MCP `clientInfo.name` plus a per-daemon counter (e.g. `brandonbews/claude-code-3`, unique per session). If `user.email` is unset or unusable, the shim fails loudly instead of inventing a name.
- `--as <principal>` overrides the whole identity, for shared machines, scripted actors, and tests. Principals are `human` or `human/agent` (no spaces, at most one `/`). One principal per session, bound at connect; every event you write is stamped with it.
- **Never connect to the daemon's HTTP endpoint directly.** Keepalive pings ride streamable HTTP's standalone GET stream; a hand-rolled client that skips that stream is declared dead within two minutes and its leases lapse mid-work. The shim handles all of this.
- If the shim exits with "daemon session ended" on stderr, your session is dead and your leases have stopped renewing. If it exits with "stdio session (harness stdin, ...)", the harness-side stream broke, usually from stray non-protocol bytes on the shim's stdin; the message quotes the most recent bytes so the stray writer can be identified. Either way, reconnect before doing anything else.

**Leases are session-bound; there is no heartbeat tool.** While your session is connected, the daemon auto-renews every claim you made through it (15 min TTL, renewed every 5). If it dies, renewals stop, the lease lapses within the TTL, the task returns to the pool, and your run is auto-closed as `interrupted`. Session death costs auto-renewal, not holdership: reconnect as the same principal before the lease lapses and you can still `add_note`, `finish_run`, or `release_claim` the task you held. Do that first, before claiming anything new.

## The twelve tools

Orient: `get_backlog`, `get_task` · Loop: `claim_next`, `claim_task`, `confirm_claim`, `release_claim`, `finish_run` · Communicate: `escalate`, `add_note`, `relay_answer` · Decompose: `create_task`, `update_task`. That is the whole surface: no admin tools, and nothing is ever hard-deleted. Cancelling rides `update_task` as `status: "cancelled"`, and every steering action a human asks you to perform (cancel, reprioritize, retitle, re-edge, hold/resume) is an `update_task` field write.

By default `get_backlog` returns exactly three arrays (`ready`, `inbox`, `held`), and the claim tools take from `ready` alone. To orient on anything else, pass the optional `scope` input: section names from `in_progress`, `blocked`, `done`, `cancelled`, `escalations`. Each adds one array of slim rows (id, title, status, priority, labels; no description) plus that section's extras: holder and lease expiry for `in_progress`; `dep:<task-id>` / `esc:<escalation-id>` waiting reasons for `blocked`; close stamp and closing actor, newest first, for `done` / `cancelled`; full open escalation records in raise order for `escalations`, which is how you find the ID `relay_answer` needs. Hydration is still `get_task`'s job: discover an ID with scope, then dig.

## The loop

1. **Claim before working.** Call `claim_next` (or `claim_task` if the human named a specific task). Never start work on a task you have not claimed, because another agent may claim it at any moment. `claimed: false` means the pool is empty or nothing matches your labels. That is a normal outcome, not an error; report it and stand by rather than retrying in a loop.

   Selection: highest priority first (higher number wins; `0` is the default), creation (ULID) order within a priority; this is the same order as `get_backlog`'s `ready` array. The optional `labels` filter is **all-of**: a task matches only if it carries every requested label; extra labels still match, an unlabelled task never matches a labelled request, and omitting `labels` takes the best ready task outright. Label meanings are your repo's convention; the filter matches strings, nothing more. `claimed: false` doesn't distinguish an empty pool from your filter excluding everything; re-call without `labels` to tell them apart.
2. **Read the task as a prompt.** The claim returns the whole task: description, acceptance criteria, dependency context, prior notes, and prior runs. If a prior run exists, you are a **successor, not a pioneer**: read its notes and outcome first, resume from where it stopped, and don't re-derive what a predecessor already learned. A prior run ending `interrupted` means your predecessor died mid-work; trust its notes over its (absent) summary.
3. **Work normally.** Worktrees, branches, and commits are ordinary git on ordinary code branches. tuhdoo never touches your code workflow.
4. **Checkpoint with `add_note`, optionally.** Continuity is carried by the typed transition events, whose payloads are already mandatory: `claim`, `finish_run` (outcome + summary), `release_claim` (required reason), `escalate` (question + context), and the daemon's synthesized `interrupted` run. A note adds to those only when it would save a successor real work: a significant finding, the state before a risky or hard-to-reverse step, exactly where things stand at a stopping point (file paths, branch name, what's done, what's next). Zero-note runs are normal; a note restating the finish summary is ledger noise.
5. **Confirm before you merge.** If your work will land by merge (a PR, auto-merge, a push to a shared branch), call `confirm_claim` first and merge only on a confirmed verdict. A claim is provisional: another machine's earlier claim can void yours without your session hearing about it, so holding a claim is not proof of ownership; a confirmation is, irrevocably. If you lost, stand down, close any PR you opened, and go straight to `finish_run`; your attempt is recorded `superseded` with branch and summary kept for salvage. `finish_run(done)` runs the same gate, so skipping this step can never certify lost work. But by finish time your PR may already have merged, which is exactly why the confirm call comes *before* the merge.
6. **Finish honestly.** `finish_run` with the true outcome and links (branch, PR, commits) plus a summary a human can act on. Exactly four outcomes are reportable:
   - `done`: the acceptance criteria hold. Refereed: the daemon runs the confirmation gate inside `finish_run(done)`; a lost race is recorded `superseded` instead, links kept, and the result tells you so.
   - `failed`: attempted and did not work; the summary says why.
   - `abandoned`: stopping without completion for reasons other than a blocker.
   - `blocked`: see escalation below.

   `interrupted` and `superseded` are daemon-synthesized; the agent surface rejects them as reported outcomes. In squash/rebase repos the branch SHAs never land on the default branch; after the merge lands, report the commit(s) on a durable branch that carry the work in `merged_as`, if known. The summary, not a final note, is the handoff; don't write a closing note that duplicates it.
7. **Or release.** If you claimed something you cannot or should not work on, `release_claim` with a reason. Never sit on a claim you are not working.

**Never end a session holding a live claim silently.** Finish or release. (If you die anyway, the lease expires and the run auto-closes as `interrupted`. That is recoverable, but a note-poor `interrupted` run wastes your successor's time.)

**Salvage.** A synthesized `interrupted` run is branch-less: the daemon cannot record a branch name only the dead agent knew. To find an interrupted predecessor's work, look in order: its notes first (step 4 is where a branch name belongs), then the repo's branches for one carrying the task's short id. The breadcrumb practice that makes the search work is a recommendation, not protocol law: put the task id in your branch name, and `add_note` the branch name once it exists if there is any chance of interruption.

## Escalation: design for succession, not conversation

Your question will usually be answered after your session is gone. Do not wait for answers; hand off.

**The question field carries the whole decision package**: the question, the options you see, and your recommendation if you have one, all kept short. **The context field is background only**: the minimum a human needs to answer, never the lead. A human must be able to answer from the escalation alone, without reading your run.

- **Non-blocking question:** `escalate`, keep working on what doesn't depend on it.
- **Blocking question:** in order: (1) `escalate` with `blocking: true`; (2) `add_note` exactly where work stopped and what the answer will unblock; (3) `release_claim`; (4) `finish_run` with outcome `blocked`. A blocking escalation keeps the task out of the ready pool until a human answers, so releasing doesn't risk a claimant working blind; when the answer lands, the task re-enters the pool and the next claimant (possibly future-you) picks up question and answer together in hydration. Say each thing once: the full story lives in the escalation, the note adds only resume-state the escalation doesn't carry, and the `blocked` summary is one line pointing at the escalation.

**No attempt, no escalation.** An escalation is a question raised by **stalled work**: you were in flight, hit a wall, and an answer unblocks you. If no attempt exists (the task isn't something to act on right now, or a human said "not yet"), that is `update_task` with `status: "held"`, never an escalation. The two fences must stay distinct: `held` is a quiet shelf meaning *someone decided not now*; an open blocking escalation is a loud inbox item meaning *someone owes an answer and work is stopped until it lands*. Fencing parked work with escalations dilutes exactly the signal escalations exist to provide.

**Human live in your session?** Then escalate/release is legitimately bypassed: ask directly and keep working. Claim discipline and an honest, self-contained `finish_run` still apply in full: the human monitors the ledger, not the session transcript, and an interactive run without a real summary leaves the same hole as any other.

**Answered out of band?** If a human answers your question in your live session instead of on a steering surface, record it with `relay_answer` (escalation ID + the answer as given); otherwise the settled question lingers open, polluting the escalation inbox. Attribution goes to your root principal (daemon-derived; you cannot attribute to anyone else) and the ledger marks you as the relay. A blocking escalation's task returns to the pool immediately; if you still hold the claim, just keep working, because the handoff sequence is only for answers that haven't arrived. You are the scribe, not the decider: relay only a decision a human actually made, verbatim, never an answer you inferred. Relay open escalations only; amending a settled answer is human work on the steering surfaces.

## Capture cheap, promote deliberate: inbox and held

Tasks have five statuses: `open`, `inbox`, `held`, `done`, `cancelled`. Only `open` tasks are ever served by the claim tools; `get_backlog` returns `inbox` and `held` as their own arrays so you can orient on them without being handed one.

- **`inbox` is the capture tier.** When an idea surfaces mid-work (a refactor you noticed, a bug you stepped around), capture it *now* with `create_task` and `status: "inbox"`. Title-only is legitimate; a fragment description is legitimate **for inbox items only**. Don't stop to scope, price, or write acceptance criteria: capture must not cost a planning session, and an uncaptured idea dies with your context.
- **`held` is deliberately paused.** It passed triage and is workable, but a human (or an agent with reason) has parked it. Pause and resume are `update_task` with `status: "held"` / `"open"`. Creating directly into `held` is allowed.
- **Promotion (`inbox` → `open`) is where the bar applies.** Anyone, human or agent, may promote, by setting `status: "open"` **and supplying a prompt-quality description in the same call** (next section). The schema won't stop a bare-title promotion; the protocol does: promoting a task to sneak it past scoping is the same failure as writing a bad task, and the claimant it burns will file the same blocking escalation.
- **Shelved tasks are ordinary shared state.** Labels and edges are allowed at capture; a dependency on an inbox/held task blocks its dependents naturally (it is not `done`). Priority is stored but inert until the task is `open`.
- **Never cancel a capture you merely disagree with.** Curation stays human work; the no-delete rule applies unchanged.

## Decomposition

When a task is too large, decompose it: one batch `create_task` call with the child tasks and the dependency edges between them (use `tmp:<name>` refs within the batch; the whole DAG lands atomically or not at all), then point the task you hold at the children with `update_task` `depends_on`. A container is just a task that depends on its children. Work the children through the normal loop, and note why you split it the way you did; decomposition is checkpoint-worthy.

`update_task` changes only the fields you send, but its list fields (`labels`, `depends_on`) are **full replacements**: send the complete new list, never a delta.

## Writing tasks: descriptions are prompts

Task quality bounds output quality. This bar applies to every task created as (or promoted to) `open`; inbox captures are exempt until promotion, because that exemption is the capture tier's purpose, not a loophole for open tasks. A well-formed description contains:

- **Context**: why this exists, with links to design docs or prior tasks.
- **The ask**: what to build or change, concretely.
- **Acceptance criteria**: how the claimant knows it's done; test-shaped where possible.
- **Pointers**: relevant files, modules, prior art in the repo.
- **Constraints**: what must not change, project laws that bite here.

If you claim a task whose description fails this bar and you cannot proceed safely, that is a legitimate blocking escalation: ask for the missing criteria rather than guessing at scope.

## Anti-pattern: dangling pointers

The ledger is read by people and agents who were never in your session. Every entry humans monitor (task descriptions, run summaries, escalation questions and answers, release reasons) must be **self-contained, or point only at durable repo state**: committed files, design docs, task IDs, branches, PRs. Never reference "this session", chat context, scratchpads, plan-mode output, or uncommitted local paths: those die with your session, and the pointer dangles forever. If the plan lives only in your context, write it into the description (or commit it to the repo) before the call, every time.

## What you never do

- Never write to the tuhdoo data branch with git directly: all plan writes go through the MCP tools, and the daemon is the sole writer.
- Never force-push the data branch: it breaks every peer at once.
- Never cancel a task unbidden: cancelling is a human decision; write it only when a human directed it, in your session or in a task you claimed. Nothing is ever hard-deleted, and the history stays on the ledger.
- Never report an outcome you didn't earn: `interrupted` and `superseded` are the daemon's words, and `done` means the acceptance criteria actually hold.
- Never merge fleet work off an unconfirmed claim: call `confirm_claim` first, and merge only on a confirmed verdict. Merging is the one act the referee cannot undo for you.
- Never work unclaimed, never claim un-worked.
