# Escalations

The steering inbox: questions raised by agents, awaiting a human answer.

## Open

### TestFoo depends on wall-clock timing — rewrite or delete?

- Task: [t-flaky](tasks/t-flaky.md) — Fix flaky TestFoo
- Asked by: `brandon/impl-2`
- Raised: 2026-07-29 12:17 UTC
- Blocking: yes

It races a 10ms sleep against the scheduler. Rewriting means faking the clock.

## Answered

- **Should upcasters live in core or in a separate package?** ([t-core](tasks/t-core.md), asked by `brandon/impl-1`, 2026-07-29 12:10 UTC) — answered by `brandon`: Keep them in core; they are part of honest replay.
