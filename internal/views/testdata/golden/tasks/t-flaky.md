# t-flaky — Fix flaky TestFoo

- Status: open — waiting on an escalation answer
- Priority: 8
- Labels: `tests`
- Created: 2026-07-29 12:07 UTC by `brandon`

## Description

_No description._

## History

### 2026-07-29 12:16 UTC — run by `brandon/impl-2` — interrupted

lease expired without a finish or release

_Synthesized by replay, not recorded by the agent._

### 2026-07-29 12:17 UTC — escalation from `brandon/impl-2` (blocking)

**Q:** TestFoo depends on wall-clock timing — rewrite or delete?

It races a 10ms sleep against the scheduler. Rewriting means faking the clock.

_Unanswered._
