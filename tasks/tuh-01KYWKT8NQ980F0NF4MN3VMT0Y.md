# claim_task on an escalation-blocked task reports "unmet dependencies"

`tuh-01KYWKT8NQ980F0NF4MN3VMT0Y`

- **Status:** done
- **Priority:** 0
- **Labels:** `mcp` `dx`
- **Created:** 2026-07-31 17:33 UTC by `brandon/claude-code-1`

## Description

Context: Observed 2026-07-31: claim_task on t-01KYRVCBE83KT62BAE1502VV29 (status open, sole depends_on task done, one open blocking escalation) failed with "task ... is not ready: unmet dependencies". Root cause confirmed by code reading: core.State.Ready (internal/core/state.go:145) returns false for four reasons — not open, actively claimed, unmet dependencies, open blocking escalation — but opClaimTask (internal/daemon/ops.go:281-289) only distinguishes the first two in its switch; deps and escalation both fall through to the "unmet dependencies" default arm.

The ask: make claim_task's not-ready error name the actual blocker. Suggested shape: a pure helper in core (e.g. a NotReadyReasons/ClaimBlockers method on *State returning the unmet dep IDs and open blocking escalation IDs), table-tested; opClaimTask consumes it to build the message. Escalation-blocked reads like: task X is not ready: blocked by open escalation esc-… — include the escalation ID so the caller can act on it. Unmet deps: name the dep IDs. Both at once: name both. Related prior art that already names reasons for display — internal/views/views.go (waitingOn) and cmd/tuhdoo/snapshot.go (blockedReason / blockedReasonTUI); sharing the new core helper with them is optional, but if touched their rendered output must not regress.

Acceptance:
- Table-driven core tests for the reason helper: escalation-only, deps-only, both, plus not-open and actively-claimed cases.
- Daemon-level test: claim_task on an open task blocked only by a blocking escalation returns a conflict whose message names the escalation ID and does NOT say "unmet dependencies"; the deps case names the dep IDs.
- make test lint green from the repo root.

Pointers: internal/daemon/ops.go (opClaimTask), internal/core/state.go (Ready), internal/daemon/mcp.go:426 (claim_task tool description — update its "unmet dependencies" phrasing if it would now mislead), internal/views/views.go (waitingOn), cmd/tuhdoo/snapshot.go (blockedReason).

Constraints: deterministic core stays pure — data in, data out (T1). No MCP surface changes (T5): same eleven tools, better error string. Stored event bytes untouched (errors are not events, T3).

## History

### 2026-07-31 23:31 UTC — run by `brandon/claude-code-1` — done

- Branch: `tuh-mt0y/claim-task-names-blockers`
- PR: <https://github.com/brandonbews/tuhdoo/pull/5>
- Commits: `649e758`

Merged (squash) to main via PR #5 and deployed. New pure helper core.State.ClaimBlockers returns unmet dep IDs and open blocking escalation IDs in stored order; Ready now consumes it so predicate and diagnostic cannot disagree. opClaimTask's not-ready conflict names the actual blockers: 'blocked by open escalation <id>' / 'unmet dependencies <ids>' / both joined with ';'. claim_task tool description updated to say the error names the specific blockers. views/CLI renderers untouched. Covered by table-driven core TestClaimBlockers (escalation-only, deps-only, both, not-open, actively-claimed, ready, unknown) and daemon-level TestClaimTaskConflictNamesBlockers over the HTTP claims surface. make test lint green.
