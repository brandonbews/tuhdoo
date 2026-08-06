# claim_next discovery: capability/label filters, affinity hints, priority semantics

`tuh-01KZA0VT234XJYVZWT8EXV78J5`

- **Status:** cancelled
- **Priority:** 0
- **Labels:** `mcp` `design`
- **Created:** 2026-08-05 22:31 UTC by `brandon/claude-code-1`

## Description

Cancelled 2026-08-06 (triage grill, Brandon): split and closed. The capture asked three things; each resolved differently:

(1) Capability/label filters — already shipped before the sweep migrated this capture: claim_next has carried an optional labels input since B9 (2026-07-29), matched all-of via hasAllLabels (internal/daemon/ops.go). What was actually missing is visibility and proof — the protocol doc never mentions the input and no test exercises the matching. Spun out as tuh-01KZCMF7JKMXVDG0HANVVQ05FN (docs + tests, no behavior change).

(2) Affinity hints — dropped at Brandon's direction rather than held. Beyond lacking evidence, affinity would need a persistent agent identity, and the identity posture is deliberately the opposite: principals are minted per session ("the ledger records sessions, not one eternal alias" — session bind in internal/daemon/mcp.go). Label filters already cover capability routing value-agnostically. If the site/ dogfood surfaces routing friction, the monorepo-grain grill (tuh-01KZA0VT234XJYVZWT8YFV8XE2) owns watching for it, and a fresh capture can carry the actual evidence instead of speculation.

(3) Priority semantics — settled in code: higher number first, ULID (creation-order) tie-break, core.ReadyTasks is the single ordering source for both claim_next and get_backlog; priority stored-but-inert while a task is inbox/held; any int legal. The documentation gap (direction stated only in a JSON-schema string) rides the spun-out task.

Standing constraints reaffirmed: no label value ever gains mechanics without a D5 revision first (labels grill, 2026-08-05); product/dogfood separation (Brandon, 2026-08-06) — claim_next docs describe mechanism only, label taxonomies belong to installing repos, never the product.

## History

_No activity yet._
