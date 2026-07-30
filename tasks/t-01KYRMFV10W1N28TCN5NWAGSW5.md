# t-01KYRMFV10W1N28TCN5NWAGSW5 — v0 definition of done: the dogfood week holds

- Status: open — waiting on an escalation answer
- Priority: 0
- Labels: `milestone`
- Created: 2026-07-30 04:28 UTC by `brandon/migrator`

## Description

Context: v0 shipped — daemon, MCP surface, CLI portal, views, agent protocol (docs/plan/roadmap.md). This task is the v0 milestone: it is done when the proof holds, not when code exists.

The ask: human verification only — no code. Confirm the definition of done from docs/plan/roadmap.md: one week of tuhdoo managing its own development (clock started 2026-07-30, cutover commit on main), agents working exclusively through claim_next -> work -> add_note -> finish_run/escalate, tuhdoo watch running beside sessions, and zero manual repair of the data branch.

Acceptance: the blocking escalation on this task is answered confirming the week held; a human then marks this task done.

Constraints: agents must not work this task. If you are an agent holding this claim, release it with reason "human-verification milestone".

## History

### 2026-07-30 04:28 UTC — escalation from `brandon/migrator` (blocking)

**Q:** v0 definition-of-done check: did the dogfood week hold? Answer on or after 2026-08-06. The bar (docs/plan/roadmap.md, v0): one week of tuhdoo managing its own development — sessions driven through claim_next, watch running beside agents — with zero manual repair of the data branch. Answer "yes" to release this milestone (then mark it done), or describe what broke.

Raised at B12 cutover (2026-07-30, brandon/migrator): the markdown backlog was migrated into this data branch in one atomic create_task batch and tombstoned on main. This blocking escalation is the DoD clock and the agent fence in one: it keeps the v0 milestone out of the ready pool until a human verifies the week, and it puts the v0->v1 gate in the steering inbox. Development continues meanwhile through claim_next; the TUI task is the top ready item.

_Unanswered._
