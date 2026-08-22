# claim_next lands commits but never pokes the syncer and never regenerates views — deliberate or gap?

`tuh-01KZWNMJH264W3B3TGNP7FP51R`

- **Status:** inbox — untriaged capture
- **Priority:** none
- **Labels:** `go` `daemon`
- **Created:** 2026-08-13 04:21 UTC by `brandon/claude-code-1`

## Description

Observed during the 2026-08-12 architecture-map fresh read (tuh-01KZ9Z6647C3TBCYGGTXQJYE8V): claimTargetLocked does stageLocked + batcher.Flush by hand, bypassing commitLocked — so (a) sync.Poke never fires and the claim.made commit waits up to ~60s for the sync cycle despite claims being the write most sensitive to cross-machine races, and (b) stageViewsLocked never runs, so rendered views go stale after a claim until the next commitLocked write. Poke callers today: escalate/relay_answer/add_note/confirm_claim paths (ops.go ~840/885/941). May be deliberate (confirm gate fetches anyway; views refresh on next write) — decide, then either document the rationale in code or route the claim path through commitLocked(eager).

## History

_No activity yet._
