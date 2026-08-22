# v0 definition of done: the dogfood week holds

`t-01KYRMFV10W1N28TCN5NWAGSW5`

- **Status:** done
- **Priority:** none
- **Labels:** `milestone`
- **Created:** 2026-07-30 04:28 UTC by `brandon/migrator`

## Description

Context: v0 shipped — daemon, MCP surface, CLI portal, views, agent protocol (docs/plan/roadmap.md). This task is the v0 milestone: it is done when the proof holds, not when code exists.

The ask: human verification only — no code. Confirm the definition of done from docs/plan/roadmap.md: one week of tuhdoo managing its own development (clock started 2026-07-30, cutover commit on main), agents working exclusively through claim_next -> work -> add_note -> finish_run/escalate, tuhdoo watch running beside sessions, and zero manual repair of the data branch.

Acceptance: the blocking escalation on this task is answered confirming the week held; a human then marks this task done.

Constraints: agents must not work this task. If you are an agent holding this claim, release it with reason "human-verification milestone".

## History

### 2026-07-30 04:28 UTC — escalation from `brandon/migrator` (blocking)

> v0 definition-of-done check: did the dogfood week hold? Answer on or after 2026-08-06. The bar (docs/plan/roadmap.md, v0): one week of tuhdoo managing its own development — sessions driven through claim_next, watch running beside agents — with zero manual repair of the data branch. Answer "yes" to release this milestone (then mark it done), or describe what broke.

Raised at B12 cutover (2026-07-30, brandon/migrator): the markdown backlog was migrated into this data branch in one atomic create_task batch and tombstoned on main. This blocking escalation is the DoD clock and the agent fence in one: it keeps the v0 milestone out of the ready pool until a human verifies the week, and it puts the v0->v1 gate in the steering inbox. Development continues meanwhile through claim_next; the TUI task is the top ready item.

**Answer** (`brandon`): Yes — the week held, and the milestone is closed as of today (2026-08-03) rather than on 2026-08-06, because the evidence is already conclusive and the calendar was never the bar.

The DoD has been rewritten (docs/plan/roadmap.md, v0) from elapsed time to five checkable facts, all true now:

1. the backlog lives on the data branch; docs/plan/backlog.md is a tombstone — done at the B12 cutover, 2026-07-30;
2. every commit on refs/heads/tuhdoo is daemon-authored — 369 of 369, zero human commits, no hand repair ever;
3. an event-schema version bump landed on the live branch and replayed correctly — task.created/task.updated v1 to v2, 2026-07-31, with identity upcasters registered in core.NewReplayer;
4. agents drove the full loop through claim_next to finish_run/escalate, with no direct git writes to the data branch;
5. the daemon was restarted mid-session on every deploy since the cutover, with no lost or corrupted events.

Why the week was retired rather than waited out: the binary changed every few minutes throughout, so a strict reading reset the clock on every deploy and the criterion was unsatisfiable for as long as development continued. The load-bearing clause was never the week — it was 'no manual repair', which is point 2 and is mechanically checkable. The rapid iteration made this a harsher test than the week intended, not a weaker one: a live schema bump on a running ledger and dozens of mid-claim restarts are exactly the failure modes the criterion existed to catch.

Filing note: this escalation was the wrong fence. Nothing had stalled — the milestone was simply not to be worked yet, which is 'held'. It was chosen on 2026-07-30, one day before the held status existed. The rule is now written down: docs/agent-protocol.md, 'no attempt, no escalation'.

### 2026-08-03 20:07 UTC — edit by `brandon`

status open→done
