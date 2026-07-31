# tuh-01KYWKT8NQ980F0NF4MN3VMT0Y — claim_task on an escalation-blocked task reports "unmet dependencies"

- Status: inbox — untriaged capture
- Priority: 0
- Labels: `mcp`, `dx`
- Created: 2026-07-31 17:33 UTC by `brandon/claude-code-1`

## Description

Observed 2026-07-31: claim_task on t-01KYRVCBE83KT62BAE1502VV29 (open blocking escalation, sole depends_on task done) failed with "task ... is not ready: unmet dependencies". The real blocker was the unanswered escalation; the error names the wrong cause. Either the readiness check lumps escalation-blocks under a dependencies message, or done deps are misread. Reproduce, then make the error name the actual blocker (e.g. "blocked by open escalation <id>").

## History

_No activity yet._
