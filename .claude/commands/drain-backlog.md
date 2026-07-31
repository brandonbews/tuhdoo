---
description: Drain the tuhdoo backlog — claim_next → work → PR-land → finish_run, until claimed:false
---

Read CLAUDE.md and docs/agent-protocol.md, then work the backlog through the
tuhdoo MCP tools until it is drained:

1. Before claiming, check your context: at ~25% used or more, stop and
   report what you landed instead of claiming — landing a task costs
   several points and the operator's ceiling is 30% used. Otherwise
   `claim_next`. If it returns `claimed: false`, the pool is empty —
   report what you landed this session and stop cleanly, holding nothing.
2. Work the claimed task by fan-out, per the CLAUDE.md pattern: spawn a
   sub-agent pointed at CLAUDE.md, the relevant design-doc sections, and
   the task description as its prompt, with explicit file boundaries and
   "do not commit; report back." You review the report and diff, and you
   own the branch, commits, PR, and every ledger verb — leases are
   session-bound, so a sub-agent never claims or finishes. Inline work is
   the exception, for trivially small tasks. Prior runs and notes are
   your memory of earlier attempts; pass what matters into the
   sub-agent's prompt.
3. Land it per the CLAUDE.md build loop: branch `tuh-<short-id>/<slug>` off
   fresh main, commit freely, `make test lint` green locally, PR (title =
   task title, body opens with the task ID), `gh pr merge --auto --squash`,
   wait for the merge to land.
4. `finish_run(done)` only after the merge lands. Red CI: fix it, or
   `finish_run(blocked)` — unmerged work is not done.
5. If the landed task changed the binary, do the deploy-after-landing daemon
   restart from CLAUDE.md before claiming again — the restart kills your MCP
   session, so finish_run first and reconnect after.
6. Repeat from 1. Finishing a task recomputes readiness on the ledger, so
   each `claim_next` automatically serves whatever your last finish
   unblocked — there is no separate "check for unblocked" step.

Escalate rather than guess (blocking escalation → release → finish_run
blocked, per the protocol doc). The context budget lives at step 1 as a
pre-claim check; if you somehow run low mid-task anyway, finish the task
you hold, land it, and stop instead of claiming another.
