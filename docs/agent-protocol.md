# tuhdoo agent protocol

**Status:** revised against the live MCP surface (B9, session 3); field-tested 2026-07-30 against a real harness session, deviations folded in (see the field-test record at the bottom). Revised again 2026-07-30 (steering): notes reframed — the typed transition events carry continuity, `add_note` is optional garnish (step 4); the dangling-pointer anti-pattern named; interactive sessions addressed (escalation section). Revised 2026-07-31 (grill cycle): the status model grew `inbox` and `held` — capture cheap, promote deliberate (new section below). Revised again 2026-07-31 (MCP parity audit): the archive wording corrected — `update_task` `status: "cancelled"` *is* mechanically available to agents, the rule is when to use it, not whether the verb exists; and `get_backlog`'s coverage stated honestly (ready + inbox + held, nothing else). The tool descriptions in `internal/daemon/mcp.go` are the live vocabulary; if this doc and a `tools/list` response disagree, the daemon is right and this doc has a bug.

This is the instruction text a harness loads for any agent working a tuhdoo-managed project. It is the agent's half of the contract; the daemon's half (leases, serialization, sync) is automatic. The protocol exists because the ledger is **agent memory before it is human audit trail**: sessions end and contexts compact, and the only continuity between today's agent and tomorrow's is what landed on the ledger — chiefly the typed transition events the loop below already requires *(revised 2026-07-30; see step 4)*.

---

## Connecting

Access is **through the stdio shim only**. The zero-config form is the primary one:

```json
{ "mcpServers": { "tuhdoo": { "command": "tuhdoo", "args": ["mcp"] } } }
```

If tuhdoo came in as an npm devDependency, the npx form works unchanged —
`{ "command": "npx", "args": ["tuhdoo", "mcp"] }` — the npm launcher is a
transparent pass-through (stdio inherited, signals forwarded, no output of
its own).

- With no flags, your principal is **auto-derived at connect** *(revised 2026-07-30; --as was previously required)*: the human half is the local part of `git config user.email` in the repository (`brandonbews@gmail.com` → `brandonbews`), and the daemon mints the agent half at session bind from your harness's MCP `clientInfo.name` plus a per-daemon counter — e.g. `brandonbews/claude-code-3`, unique among the daemon's sessions (the counter resets when the daemon restarts). Every session gets its own name; nothing is hand-configured, so the ledger records sessions, not one eternal alias. If `user.email` is unset or unusable, the shim fails loudly at connect instead of inventing a name.
- `--as <principal>` overrides the whole identity for the session — for shared machines, scripted actors, or tests. Principals are `human` or `human/agent` (no spaces, at most one `/`); every event you write is stamped with it. One principal per session — identity is bound at connect, not per call.
- The shim bridges stdio to the daemon over a repo-local unix socket and auto-spawns the daemon if it isn't running. Run it from inside the repository.
- **Do not connect to the daemon's HTTP endpoint directly.** The endpoint is stateful: the daemon declares a session dead after ~3 missed keepalive pings, and those pings ride the standalone GET stream of streamable HTTP — a hand-rolled client that never opens that stream gets its session closed in under two minutes and its leases lapse mid-work. The shim handles all of this.
- If the shim exits with status 1 and "daemon session ended" on stderr, your session is dead and your leases have stopped renewing. Reconnect before doing anything else.
- The other shim death, "stdio session (harness stdin, ...)" on stderr, means the harness-side stream broke, not the daemon: usually stray non-protocol bytes on the shim's stdin (a fifo with a second writer is the classic way — even one stray digit poisons the stream, and the session then dies on the *next* well-formed line, which itself replays cleanly). The message quotes the most recent stdin bytes so the stray write can be identified. Leases stop renewing all the same; reconnect before doing anything else.

**Leases are session-bound — there is no heartbeat verb.** While your session is connected, the daemon auto-renews every claim you made through it (15 min TTL, renewed every 5). If your session dies, renewals stop, the lease lapses within the TTL, the task returns to the pool, and your run is auto-closed as `interrupted`. Session death costs you auto-renewal, not holdership: if you reconnect as the same principal before the lease lapses, you can still `add_note`, `finish_run`, or `release_claim` on the task you held — do that first, before claiming anything new.

## The eleven verbs

Orient: `get_backlog`, `get_task` · Loop: `claim_next`, `claim_task`, `release_claim`, `finish_run` · Communicate: `escalate`, `add_note`, `relay_answer` · Decompose: `create_task`, `update_task`. That is the whole surface — no admin verbs, and nothing is ever hard-deleted. Archive rides `update_task` as `status: "cancelled"` *(clarified 2026-07-31: the value is mechanically accepted from agents — the rule below about never archiving unbidden is protocol, not schema)*; steering actions a human asks you to perform — cancel, reprioritize, retitle, re-edge, hold/resume — are all `update_task` field writes.

Know what `get_backlog` does *not* show *(2026-07-31 audit)*: it returns exactly three arrays — `ready`, `inbox`, `held`. Tasks that are in progress (actively claimed), blocked (unmet dependencies or an open blocking escalation), `done`, or `cancelled` appear in **no** array, and there is no verb that lists them or lists open escalations across the project. A known task ID is always readable with `get_task`, whatever its status — but discovering such a task's ID, or answering "what's in progress?" / "what did we finish?", has no MCP path today; say so rather than inferring from `get_backlog` silence.

## The loop

1. **Claim before working.** Call `claim_next` (or `claim_task` if the human named a specific task). Never start work on a task you have not claimed — an unclaimed task may be claimed by another agent at any moment. `claim_next` returning `claimed: false` means the pool is empty or nothing matches your labels — a normal outcome, not an error; report it and stand by rather than retrying in a loop.
2. **Read the task as a prompt.** The claim returns the task fully hydrated: description, acceptance criteria, dependency context, prior notes, prior runs. If a prior run exists, you are a **successor, not a pioneer**: read its notes and outcome first, resume from where it stopped, and do not re-derive what a predecessor already learned. A prior run ending `interrupted` means your predecessor died mid-work — trust its notes over its (absent) summary.
3. **Work normally.** Worktrees, branches, commits — ordinary git on ordinary code branches. tuhdoo never touches your code workflow.
4. **Checkpoint with `add_note` — optionally.** *(Reframed 2026-07-30, steering decision; notes were previously "letters to your next incarnation" doctrine.)* Continuity across agents is carried by the **typed transition events**, all of which already require their payloads: `claim` (who/when, plus full hydration on pick-up), `finish_run` (outcome + summary), `release_claim` (required reason), `escalate` (question + context), and the daemon's synthesized `interrupted` run on lease expiry. `add_note` is garnish on top of that record — mid-flight context you choose to pass on, not a protocol obligation. Write one when it would save a successor real work: a significant finding, the state before a risky or hard-to-reverse step, exactly where things stand at a stopping point (file paths, branch name, what's done, what's next). Zero-note runs are normal; a note that merely restates the finish summary (see below) is noise on the ledger.
5. **Finish honestly.** `finish_run` with the true outcome and links (branch, PR, commits) plus a summary a human can act on. You may report exactly four outcomes:
   - `done` — the acceptance criteria hold.
   - `failed` — attempted and did not work; summary says why.
   - `abandoned` — stopping without completion for reasons other than a blocker.
   - `blocked` — see escalation protocol below.

   `interrupted` and `superseded` are daemon-synthesized verdicts; the agent surface rejects them.

   The summary is the handoff — it, not a final note, is where "what happened and where things stand" belongs. Don't write a closing note that duplicates it.
6. **Or release.** If you claimed something you cannot or should not work on, `release_claim` with a reason. Never sit on a claim you are not working.

**Never end a session holding a live claim silently.** Finish or release. (If you die anyway, the lease expires and the daemon auto-closes your run as `interrupted` — recoverable, but a note-poor `interrupted` run wastes your successor's time.)

## Escalation: design for succession, not conversation

Your question will usually be answered after your session is gone. Do not wait for answers; hand off.

- **Non-blocking question:** `escalate` with the question and context, keep working on what doesn't depend on it.
- **Blocking question:** in order — (1) `escalate` with the question, full context, and `blocking: true`; (2) `add_note` recording exactly where work stopped and what the answer will unblock; (3) `release_claim`; (4) `finish_run` with outcome `blocked`. A blocking escalation keeps the task out of the ready pool until a human answers, so releasing does not risk a claimant working blind. When the answer lands, the task re-enters the pool and the next claimant (possibly future-you) picks up question and answer together in hydration.

  Say each thing once: the full story lives in the escalation (that's what the human reads); the note adds only resume-state the escalation doesn't carry; the `blocked` summary can be one line pointing at the escalation. All three land on the same task and the next claimant hydrates all of them — three near-copies is ledger noise.

Write escalations so a human can answer from the escalation alone, without reading your whole run: the question, the minimal context, the options you see, your recommendation if you have one.

**Human live in your session?** *(2026-07-30)* Then escalate/release is legitimately bypassed — ask directly, keep working. Claim discipline and an honest, self-contained `finish_run` still apply in full: the ledger, not the session transcript, is what the human monitors, and an interactive run that ends without a real summary leaves the same hole as any other. Record any decision the human makes on an open escalation with `relay_answer` (below).

**Answered out of band?** Sometimes the human answers your question in your own live session instead of a steering surface. Record it with `relay_answer` (escalation ID + the answer as given) — otherwise the settled question lingers open in the inbox, polluting the signal escalations exist to provide. The answer lands exactly like a steering-surface answer: attribution goes to your root principal (the daemon derives it; you cannot attribute to anyone else), the ledger marks you as the relay, and a blocking escalation returns its task to the pool immediately — if you still hold the claim, just keep working; the handoff dance is only for answers that haven't arrived. You are the scribe, not the decider: relay only a decision a human actually made, verbatim, never an answer you inferred. Open escalations only — amending a settled answer is curation, human work on the steering surfaces.

## Capture cheap, promote deliberate: inbox and held *(2026-07-31)*

Tasks have five statuses: `open`, `inbox`, `held`, `done`, `cancelled`. Only `open` tasks are ever served by `claim_next`/`claim_task`; `get_backlog` returns `inbox` and `held` as their own arrays so you can orient on them without ever being handed one.

- **`inbox` is the chuck-it-in tier.** When an idea surfaces mid-work — a refactor you noticed, a bug you stepped around, a "we should eventually…" — capture it *now* with `create_task` and `status: "inbox"`. Title-only is legitimate; a fragment description is legitimate **for inbox items only**. Do not stop to scope it, price it, or write acceptance criteria: the whole point of the tier is that capture must not cost a planning session. An uncaptured idea dies with your context (the dangling-pointer anti-pattern's quieter sibling).
- **`held` is deliberately paused.** It passed triage and is workable, but a human (or an agent with reason) has parked it. Pause and resume are `update_task` with `status: "held"` / `"open"`. Creating directly into `held` is allowed.
- **Promotion (`inbox` → `open`) is where the bar applies.** Anyone — human or agent — may promote, by setting `status: "open"` **and supplying a prompt-quality description in the same call** (context, ask, acceptance criteria, pointers, constraints — the "descriptions are prompts" section below). The schema will not stop you from promoting a bare title; the protocol does: **promoting a task to sneak it past scoping is the same failure as writing a bad task**, and the claimant it burns will file the same blocking escalation.
- **Shelved tasks are ordinary shared state.** Labels and edges are allowed at capture; a dependency on an inbox/held task blocks its dependents naturally (it is not `done` — nothing pretends parked work is finished). Priority is stored but inert until the task is `open`.
- **Never archive a capture you merely disagree with** — curation stays human work; the no-delete rule above applies unchanged.

## Decomposition

When a task is too large, decompose it: one batch `create_task` call with the child tasks, parent edges pointing at the task you hold, and dependency edges between children (use `tmp:<name>` refs within the batch — the whole DAG lands atomically or not at all). Then work the children through the normal loop. Decomposition is itself checkpoint-worthy — note why you split it the way you did.

`update_task` changes only the fields you send, but its list fields (`labels`, `parents`, `depends_on`) are **full replacements** — send the complete new list, never a delta.

## Writing tasks: descriptions are prompts

Task quality bounds output quality. This bar applies to every task created as (or promoted to) `open`; inbox captures are exempt until promotion — that exemption is the capture tier's entire purpose, not a loophole for open tasks. A well-formed task description contains:

- **Context** — why this exists, links to design docs or prior tasks.
- **The ask** — what to build or change, concretely.
- **Acceptance criteria** — how the claimant knows it's done; test-shaped where possible.
- **Pointers** — relevant files, modules, prior art in the repo.
- **Constraints** — what must not change, project laws that bite here.

If you claim a task whose description fails this bar and you cannot proceed safely, that is a legitimate blocking escalation: ask for the missing criteria rather than guessing at scope.

## Anti-pattern: dangling pointers *(named 2026-07-30)*

The ledger is read by people and agents who were never in your session: the steering human at the TUI, a successor claiming the task, a teammate browsing the data branch. Every ledger entry humans monitor — task descriptions, run summaries, escalation questions and answers, release reasons — must be **self-contained, or point only at durable repo state**: committed files, design docs, task IDs, branches, PRs. Never reference "this session", chat context, scratchpads, plan-mode output, or uncommitted local paths — those die with your session, and the pointer dangles forever.

Cautionary tale (Cycle-4 build, 2026-07-30): an agent created a task whose description said "full plan in the session that created this task." The session ended; the plan died with it; the claimant inherited a pointer to nothing. This is the natural lazy move when your context already contains the plan — the fix is to write the plan into the description (or commit it to the repo) before the call, every time.

## What you never do

- Never write to the tuhdoo data branch with git directly — all plan writes go through the MCP verbs (the daemon is the sole writer; D2).
- Never force-push the data branch (project law; it breaks every peer at once).
- Never archive a task unbidden *(reworded 2026-07-31: `update_task` `status: "cancelled"` is the archive and the surface does accept it from agents)* — archiving is a human decision; write it only when a human directed it, in your session or in a task you claimed. Nothing is ever hard-deleted: the history stays on the ledger.
- Never report an outcome you didn't earn — `interrupted` and `superseded` are the daemon's words, and `done` means the acceptance criteria actually hold.
- Never work unclaimed, never claim un-worked.

---

## Field-test record (B11 Accept)

**2026-07-30 — session 1.** Scratch repo (`greeter`), Claude Code headless (`claude -p`) through `tuhdoo mcp --as claude/field-1`, given only this doc and "work the backlog until `claim_next` returns `claimed:false`". Backlog seeded by `brandon` over the HTTP API: one well-specified coding task (priority 2) and one deliberately unscoped trap task ("migrate the deploy pipeline to the new region", priority 1, no pointers, no criteria — and no pipeline exists).

**Conformed:** read the doc before touching tools; claimed before working, both times via `claim_next`; worked on a task branch with passing tests; ran the blocking flow in the exact prescribed order — `escalate(blocking: true)` → `add_note` → `release_claim` → `finish_run(blocked)` — rather than guessing at the trap task's scope; treated `claimed: false` as a normal stopping point and ended cleanly holding nothing. Every event landed with the right actor stamp; the blocking escalation held the task out of the ready pool, and answering it (as `brandon`, over the HTTP API) returned the task to ready — the full succession round trip works as this doc describes it.

**Deviated, and what changed because of it:**

1. *Noting collapsed into the finish.* The agent's only note on the coding task came after all work was done, immediately before `finish_run`, and restated the summary nearly verbatim — no checkpoint existed during the window where dying would actually have cost a successor something. The loop's step 4 now carries the "could a successor resume from your notes right now?" test and licenses zero-note short runs, and step 5 states that the summary, not a closing note, is the handoff.
2. *The blocked flow triplicated its content.* Escalation context, note, and `blocked` summary were three near-copies of the same paragraph. The escalation section now says: full story in the escalation, resume-state only in the note, one-line summary pointing at the escalation.

Neither deviation broke coordination — both were redundancy, not protocol violations. The order-sensitive parts (claim discipline, the blocking sequence, honest outcomes) were followed to the letter on the first read, by an agent that had never seen the tool surface before.
