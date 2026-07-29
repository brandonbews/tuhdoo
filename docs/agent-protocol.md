# tuhdoo agent protocol

**Status:** draft (B11) — written ahead of the MCP surface (B9); to be field-tested against a real harness and amended with observed deviations.

This is the instruction text a harness loads for any agent working a tuhdoo-managed project. It is the agent's half of the contract; the daemon's half (leases, serialization, sync) is automatic. The protocol exists because the ledger is **agent memory before it is human audit trail**: sessions end and contexts compact, and the only continuity between today's agent and tomorrow's is what today's agent wrote down.

---

## The loop

1. **Claim before working.** Call `claim_next` (or `claim_task` if the human named a specific task). Never start work on a task you have not claimed — an unclaimed task may be claimed by another agent at any moment.
2. **Read the task as a prompt.** The hydrated task carries description, acceptance criteria, dependency context, prior notes, and prior runs. If a prior run exists, you are a **successor, not a pioneer**: read its notes and outcome first, resume from where it stopped, and do not re-derive what a predecessor already learned.
3. **Work normally.** Worktrees, branches, commits — ordinary git on ordinary code branches. tuhdoo never touches your code workflow.
4. **Checkpoint with `add_note`.** Notes are letters to your next incarnation. Write one: after any significant finding; before any risky or hard-to-reverse step; at any stopping point. A good note says what you learned or decided and *exactly where work stands* — file paths, branch name, what's done, what's next. Do not save noting for the end; the sessions that die are precisely the ones that never reach the end.
5. **Finish honestly.** `finish_run` with the true outcome and links (branch, PR, commits) plus a summary a human can act on. Outcomes:
   - `done` — the acceptance criteria hold.
   - `failed` — attempted and did not work; summary says why.
   - `abandoned` — stopping without completion for reasons other than a blocker.
   - `blocked` — see escalation protocol below.
6. **Or release.** If you claimed something you cannot or should not work on, `release_claim` with a reason. Never sit on a claim you are not working.

**Never end a session holding a live claim silently.** Finish or release. (If you die anyway, the lease expires and the daemon auto-closes your run as `interrupted` — recoverable, but a note-poor `interrupted` run wastes your successor's time.)

## Escalation: design for succession, not conversation

Your question will usually be answered after your session is gone. Do not wait for answers; hand off.

- **Non-blocking question:** `escalate` with the question and context, keep working on what doesn't depend on it.
- **Blocking question:** in order — (1) `escalate` with the question, full context, and `blocking: true`; (2) `add_note` recording exactly where work stopped and what the answer will unblock; (3) `release_claim`; (4) `finish_run` with outcome `blocked`. The answer lands on the task; the next claimant (possibly future-you) picks up question and answer together in hydration.

Write escalations so a human can answer from the escalation alone, without reading your whole run: the question, the minimal context, the options you see, your recommendation if you have one.

## Decomposition

When a task is too large, decompose it: one batch `create_task` call with the child tasks, parent edges pointing at the task you hold, and dependency edges between children (use `tmp:` refs within the batch). Then work the children through the normal loop. Decomposition is itself checkpoint-worthy — note why you split it the way you did.

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
- Never delete or archive tasks — curation is human work via CLI/TUI.
- Never work unclaimed, never claim un-worked.
