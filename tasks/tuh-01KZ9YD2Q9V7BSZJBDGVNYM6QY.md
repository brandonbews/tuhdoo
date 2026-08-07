# User-facing docs: the human story of steering tuhdoo, platform-agnostic

`tuh-01KZ9YD2Q9V7BSZJBDGVNYM6QY`

- **Status:** open — blocked on dependencies
- **Priority:** 0
- **Labels:** `docs` `product`
- **Depends on:** [`tuh-qf4g`](tuh-01KZEPBEE8HFDQVK96AQNCQF4G.md) (open)
- **Created:** 2026-08-05 21:48 UTC by `brandon`

## Description

Child of the launch epic (tuh-01KZEPBEE8HFDQVK96AV6RN0ER). Promoted from inbox at the 2026-08-07 launch-epic structuring. Blocked on the strategy grill (tuh-01KZEPBEE8HFDQVK96AQNCQF4G), whose agenda item 5 decides the representation this task then writes in: Brandon's original ask was content that would live at tuhdoo.com but is NOT baked into the site — a platform-agnostic representation renderable on the site or elsewhere.

Context: this task owns the human-facing narrative of using tuhdoo — the prose that explains, for a person (not an agent), the intention→DAG flow the mechanism already supports: capture (inbox, title-only is fine) → triage/grill → promote (prompt-quality description: context / ask / acceptance / pointers / constraints) → decompose (atomic batch create_task with tmp: refs; a container depends_on its children) → steer (priority, edges, held, escalation answers). The mechanism all exists; the prose for humans exists nowhere. The 2026-08-06/07 triage-and-structuring sessions are the living example of the flow — this launch epic itself was built exactly this way and can be the worked example.

The ask (representation per the grill's decision): write the user-facing docs — the steering flow above, plus what adopting tuhdoo looks like for a team (init, the TUI, escalation answering, onboarding a teammate — docs/onboarding material already in-repo is source material).

Acceptance (provisional until the grill records the representation decision here): the capture→triage→promote→decompose→steer flow documented for humans with a worked example; content stored in the platform-agnostic form the grill picked, renderable on the site and readable standalone; make test lint untouched/green.

Constraints — two audiences, two documents, no forking: the agent-facing half of the conventions ships in docs/agent-protocol.md and reaches foreign repos via the tuhdoo-protocol-command task (tuh-01KZANB3J4YYH09F0Z6FSZQ5CD). These docs are for humans; link or reference the protocol doc, never copy its content into a third divergent version.

History: captured by Brandon 2026-08-05; absorbed the plan-materialization open-question at the 2026-08-06 triage grill (tuh-01KZA0VT234XJYVZWT8Q19P9QM, cancelled as subsumed); promoted into the launch epic 2026-08-07.

## History

_No activity yet._
