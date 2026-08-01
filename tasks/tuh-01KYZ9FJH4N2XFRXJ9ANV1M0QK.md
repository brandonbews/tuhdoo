# get_backlog scope input: MCP read parity with the TUI sections (T5 revision)

`tuh-01KYZ9FJH4N2XFRXJ9ANV1M0QK`

- **Status:** open — ready
- **Priority:** 1
- **Labels:** `mcp` `go` `design-revision` `dx`
- **Created:** 2026-08-01 18:30 UTC by `brandon/claude-code-1`

## Description

Context: The MCP parity audit (tuh-01KYWVNF91Y7H9GK0X1RAE2SBW, PR #7) verified all ten write-side steering actions but found the read side blind: no MCP path lists in-progress, blocked, done, or cancelled tasks, nor open escalations — a blocking escalation even removes its task from ready, so an MCP-only agent cannot discover the task ID to get_task it, and relay_answer needs escalation IDs no verb can produce. Grilled with Brandon 2026-08-01 (escalation 01KYXB0WN9QKZ1PHPQ8WWM8HQG carries the evidence; answered pointing here). Settled: MCP must be read-self-sufficient — the CLI one-shots remain the scripting surface, not the excuse. The verb count stays eleven.

The ask, docs first per project law:

1. Commit 1 — T5 revision in docs/design/002-technology.md, in place, dated 2026-08-01: (a) get_backlog gains an optional scope input (design below); (b) the "No delete, no admin verbs" paragraph rewritten — keep "no delete: the ledger never deletes; cancelled is the archive", replace the false "curation is human work via CLI/TUI" claim with the real doctrine: curation is mechanically open to agents through update_task; the norm that it happens at a human's direction is protocol (agent-protocol.md), not schema — the same permissive-mechanics/documented-semantics split as the status model. Update docs/agent-protocol.md orientation guidance to mention scope.

2. get_backlog grows one optional input: scope, an array of section names from {in_progress, blocked, done, cancelled, escalations} (plumbing vocabulary — cancelled, never archived, matching update_task). Omitted or empty scope returns exactly today's response, byte-identical — the worker hot path (get_backlog -> claim_next) never pays a new token. Each requested value adds one response array keyed by the scope name. Unknown scope values are rejected with a clear error, not ignored.

3. Slim rows — the token-bloat guard, a design constraint not an optimization: every new scope's rows carry id, title, status, priority, labels and NO description (get_task is the hydration step; orientation lists, hydration digs). Per-scope payoff fields:
   - in_progress: holder principal + lease expiry (from the active claim).
   - blocked: waiting reasons as condensed IDs — dep:<task-id> / esc:<escalation-id> — the same condensation as the T7 one-shot contract; the story lives in get_task.
   - done / cancelled: closed_at + closed_by, newest first (recency is the browse axis).
   - escalations: full escalation records (id, task, question, context, blocking, raised_at) in raise order — open only (relay_answer serves open only; answered ones hydrate via get_task). Not slim by design: the question text is the payload.

4. Plumbing: closed_at/closed_by come from the ClosedAt/ClosedBy replay fields specced identically in the history-view task (tuh-01KYX7303WN3RSBXXB9CAGZB01): in-memory only (T3), set by the status-change event entering done/cancelled, cleared on leaving terminal, fallback to the creation event for born-terminal tasks (B12 migration shape). No edge between the two tasks — whichever lands first builds the fields, the second consumes them; check before building.

Acceptance:
- Test: omitted scope produces a response with exactly today's keys and content (guards the hot path).
- Tests per scope value: correct array, slim-row fields present, description absent; in_progress rows name holder and lease expiry; blocked rows carry dep:/esc: reasons; done/cancelled ordered newest-first by close time including the born-terminal fallback; escalations open-only in raise order.
- Unknown scope value errors clearly.
- Replay table-tests for ClosedAt/ClosedBy if this task builds them (see coordination note above).
- Tool description updated (short and directive: omit for the claimable backlog; pass scope to orient on other sections); T5 and agent-protocol.md revisions landed; make test lint green.

Pointers: internal/daemon/ops.go (opBacklog), internal/daemon/mcp.go (get_backlog registration ~line 380), internal/daemon/api.go (backlog/state handlers — HTTP passthrough of scope is optional, only if trivial; the CLI one-shots already read state and need nothing), internal/core/replay.go + state.go (ClosedAt/ClosedBy), docs/design/002-technology.md T5, docs/agent-protocol.md.

Constraints: eleven verbs stay eleven; omitted-scope response byte-identical; no stored-byte changes (T3); host-agnostic (T2); boring Go — scope handling is a loop over requested names, not cleverness.

## History

_No activity yet._
