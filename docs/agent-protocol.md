# tuhdoo agent protocol

**Status:** revised against the live MCP surface (B9, session 3); field-test per B11's Accept block pending. The tool descriptions in `internal/daemon/mcp.go` are the live vocabulary; if this doc and a `tools/list` response disagree, the daemon is right and this doc has a bug.

This is the instruction text a harness loads for any agent working a tuhdoo-managed project. It is the agent's half of the contract; the daemon's half (leases, serialization, sync) is automatic. The protocol exists because the ledger is **agent memory before it is human audit trail**: sessions end and contexts compact, and the only continuity between today's agent and tomorrow's is what today's agent wrote down.

---

## Connecting

Access is **through the stdio shim only**:

```json
{ "mcpServers": { "tuhdoo": { "command": "tuhdoo", "args": ["mcp", "--as", "brandon/impl-1"] } } }
```

- `--as <principal>` binds your identity for the whole session. Principals are `human` or `human/agent` (no spaces, at most one `/`); every event you write is stamped with it. One principal per session — identity is bound at connect, not per call.
- The shim bridges stdio to the daemon over a repo-local unix socket and auto-spawns the daemon if it isn't running. Run it from inside the repository.
- **Do not connect to the daemon's HTTP endpoint directly.** The endpoint is stateful: the daemon declares a session dead after ~3 missed keepalive pings, and those pings ride the standalone GET stream of streamable HTTP — a hand-rolled client that never opens that stream gets its session closed in under two minutes and its leases lapse mid-work. The shim handles all of this.
- If the shim exits with status 1 and "daemon session ended" on stderr, your session is dead and your leases have stopped renewing. Reconnect before doing anything else.

**Leases are session-bound — there is no heartbeat verb.** While your session is connected, the daemon auto-renews every claim you made through it (15 min TTL, renewed every 5). If your session dies, renewals stop, the lease lapses within the TTL, the task returns to the pool, and your run is auto-closed as `interrupted`. Session death costs you auto-renewal, not holdership: if you reconnect as the same principal before the lease lapses, you can still `add_note`, `finish_run`, or `release_claim` on the task you held — do that first, before claiming anything new.

## The ten verbs

Orient: `get_backlog`, `get_task` · Loop: `claim_next`, `claim_task`, `release_claim`, `finish_run` · Communicate: `escalate`, `add_note` · Decompose: `create_task`, `update_task`. That is the whole surface — no delete, no admin verbs; curation is human work.

## The loop

1. **Claim before working.** Call `claim_next` (or `claim_task` if the human named a specific task). Never start work on a task you have not claimed — an unclaimed task may be claimed by another agent at any moment. `claim_next` returning `claimed: false` means the pool is empty or nothing matches your labels — a normal outcome, not an error; report it and stand by rather than retrying in a loop.
2. **Read the task as a prompt.** The claim returns the task fully hydrated: description, acceptance criteria, dependency context, prior notes, prior runs. If a prior run exists, you are a **successor, not a pioneer**: read its notes and outcome first, resume from where it stopped, and do not re-derive what a predecessor already learned. A prior run ending `interrupted` means your predecessor died mid-work — trust its notes over its (absent) summary.
3. **Work normally.** Worktrees, branches, commits — ordinary git on ordinary code branches. tuhdoo never touches your code workflow.
4. **Checkpoint with `add_note`.** Notes are letters to your next incarnation. Write one: after any significant finding; before any risky or hard-to-reverse step; at any stopping point. A good note says what you learned or decided and *exactly where work stands* — file paths, branch name, what's done, what's next. Do not save noting for the end; the sessions that die are precisely the ones that never reach the end.
5. **Finish honestly.** `finish_run` with the true outcome and links (branch, PR, commits) plus a summary a human can act on. You may report exactly four outcomes:
   - `done` — the acceptance criteria hold.
   - `failed` — attempted and did not work; summary says why.
   - `abandoned` — stopping without completion for reasons other than a blocker.
   - `blocked` — see escalation protocol below.

   `interrupted` and `superseded` are daemon-synthesized verdicts; the agent surface rejects them.
6. **Or release.** If you claimed something you cannot or should not work on, `release_claim` with a reason. Never sit on a claim you are not working.

**Never end a session holding a live claim silently.** Finish or release. (If you die anyway, the lease expires and the daemon auto-closes your run as `interrupted` — recoverable, but a note-poor `interrupted` run wastes your successor's time.)

## Escalation: design for succession, not conversation

Your question will usually be answered after your session is gone. Do not wait for answers; hand off.

- **Non-blocking question:** `escalate` with the question and context, keep working on what doesn't depend on it.
- **Blocking question:** in order — (1) `escalate` with the question, full context, and `blocking: true`; (2) `add_note` recording exactly where work stopped and what the answer will unblock; (3) `release_claim`; (4) `finish_run` with outcome `blocked`. A blocking escalation keeps the task out of the ready pool until a human answers, so releasing does not risk a claimant working blind. When the answer lands, the task re-enters the pool and the next claimant (possibly future-you) picks up question and answer together in hydration.

Write escalations so a human can answer from the escalation alone, without reading your whole run: the question, the minimal context, the options you see, your recommendation if you have one.

## Decomposition

When a task is too large, decompose it: one batch `create_task` call with the child tasks, parent edges pointing at the task you hold, and dependency edges between children (use `tmp:<name>` refs within the batch — the whole DAG lands atomically or not at all). Then work the children through the normal loop. Decomposition is itself checkpoint-worthy — note why you split it the way you did.

`update_task` changes only the fields you send, but its list fields (`labels`, `parents`, `depends_on`) are **full replacements** — send the complete new list, never a delta.

## Writing tasks: descriptions are prompts

Task quality bounds output quality. A well-formed task description contains:

- **Context** — why this exists, links to design docs or prior tasks.
- **The ask** — what to build or change, concretely.
- **Acceptance criteria** — how the claimant knows it's done; test-shaped where possible.
- **Pointers** — relevant files, modules, prior art in the repo.
- **Constraints** — what must not change, project laws that bite here.

If you claim a task whose description fails this bar and you cannot proceed safely, that is a legitimate blocking escalation: ask for the missing criteria rather than guessing at scope.

## What you never do

- Never write to the tuhdoo data branch with git directly — all plan writes go through the MCP verbs (the daemon is the sole writer; D2).
- Never force-push the data branch (project law; it breaks every peer at once).
- Never delete or archive tasks — curation is human work via CLI/TUI; the agent surface has no verbs for it.
- Never report an outcome you didn't earn — `interrupted` and `superseded` are the daemon's words, and `done` means the acceptance criteria actually hold.
- Never work unclaimed, never claim un-worked.
