# HTTP surface accepts daemon-only outcomes and carries a dead manual renew endpoint

`tuh-01KZVZT7F8CVJYX1NZZNQZ5X37`

- **Status:** inbox — untriaged capture
- **Priority:** 0
- **Labels:** `audit-finding`
- **Created:** 2026-08-12 21:59 UTC by `brandon/claude-code-bg`

## Description

Go-sweep audit finding (tuh-01KZ9YBF1N06FQ37XV65940SSG). (1) opFinishRun (internal/daemon/ops.go ~527-529) accepts `interrupted` and `superseded` as reported outcomes; only the MCP layer narrows (mcp.go ~536-541). Any HTTP caller holding a claim can record outcome interrupted via POST /v0/runs — but T5/protocol say those are daemon-synthesized and the agent surface rejects them, and D6 makes the daemon the referee of how attempts ended. (2) POST /v0/claims/renew (api.go ~243-263, ops.go opRenewClaim ~379-404, holderClaimLocked ~1148-1160) has zero non-test callers — a manual heartbeat on a system whose design is 'no heartbeat tool; leases are session-bound' (T5); MCP renewal goes through store.WriteLease directly. Decide: deliberate escape hatches (add a design-doc sentence) or fossils to remove (~70 lines + tests). Removal is a behavior change, so it was out of the sweep's zero-behavior scope.

## History

_No activity yet._
