# HTTP portal: narrow finish outcomes to the agent set at the op layer; delete the dead renew endpoint

`tuh-01KZVZT7F8CVJYX1NZZNQZ5X37`

- **Status:** open — ready
- **Priority:** none
- **Labels:** `go` `daemon` `audit-finding`
- **Created:** 2026-08-12 21:59 UTC by `brandon/claude-code-bg`

## Description

Context: Go-sweep audit finding, re-verified 2026-08-27; direction decided by Brandon at the triage grill (remove both). (1) opFinishRun (internal/daemon/ops.go:528-529) accepts all six outcomes including daemon-synthesized interrupted/superseded; only the MCP layer narrows (mcp.go:533-541); handleFinishRun (api.go:289-304) applies no filter — and no test posts to POST /v0/runs at all (the route registration at api.go:28 is the only repo reference). D6 makes the daemon the referee of how attempts ended; the op layer is where every caller meets that. (2) POST /v0/claims/renew is dead: route api.go:27, handler api.go:243-263, opRenewClaim ops.go:379-404, holderClaimLocked ops.go:1146-1160 (exclusive to renew) — zero non-test callers anywhere (only daemon_test.go:644,653); MCP renewal is an independent path (renewSessionLeases → renewOnce → store.WriteLease at mcp.go:236/257/280); write_cmds.go:13-15 already documents why no CLI renew exists (leases are session-bound, T8).

The ask: (1) move the outcome narrowing into opFinishRun: reported outcomes are done/failed/abandoned/blocked; interrupted/superseded are daemon-synthesized only, rejected from every caller. The MCP layer's narrowing may stay as a friendlier early error or collapse onto the op's. (2) Delete the renew endpoint wholesale: route, handler, opRenewClaim, holderClaimLocked, and TestClaimLifecycle's renew section (daemon_test.go ~634-659) — ~90 lines. (3) One design-doc revision note (002, T5/T8 vicinity) recording both: lease renewal is session-bound with no manual surface on any layer, and outcome refereeing is enforced at the op layer for every caller.

Acceptance: an HTTP POST /v0/runs reporting interrupted (or superseded) is rejected with a clear error, pinned by a test — the first test of /v0/runs, so add a happy-path case alongside; /v0/claims/renew is gone and no repo reference remains; the design-doc note landed; make test lint green.

Constraints: the T5 twelve-tool agent surface is behaviorally unchanged; the HTTP behavior change is deliberate and Brandon-approved 2026-08-27.

## History

### 2026-08-27 14:49 UTC — edit by `brandon/claude-code-1`

retitled · description edited · status inbox→open · labels +go +daemon

### 2026-08-27 14:56 UTC — edit by `brandon/claude-code-1`

retitled · description edited · status open→open · labels edited
