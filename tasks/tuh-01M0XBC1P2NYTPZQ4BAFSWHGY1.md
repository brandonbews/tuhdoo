# Claim/release writes skip view regen — data-branch markdown never shows in-progress

`tuh-01M0XBC1P2NYTPZQ4BAFSWHGY1`

- **Status:** inbox — untriaged capture
- **Priority:** none
- **Created:** 2026-08-25 20:56 UTC by `brandon/claude-code-2`

## Description

Context (adopter-visible; diagnosed live 2026-08-25 from this repo's own data branch): Brandon watched GitHub during a claimed task (tuh-01M0HF5SS536W9JAS2CB2ZQCT8, claimed 18:59:51Z, finished 19:15:46Z) and the rendered markdown never showed it in progress — it went straight from ready to done. The views are not the problem: backlog.md has an "## In progress" section, README.md counts it, task pages render "open — in progress, claimed by ..." (internal/views/views.go:191,154,425; format 5's design target is exactly someone glancing at the branch on GitHub).

Verified mechanism: view staging happens in exactly one place — commitLocked (internal/daemon/daemon.go:435): stage events → refresh → stageViewsLocked() → (eager) Flush + sync.Poke(). The claim path bypasses it: claimTargetLocked (internal/daemon/ops.go:357) hand-rolls stageLocked(ev) + Flush() + WriteLease + refresh — no stageViewsLocked, no Poke. releaseLocked (ops.go:458) has the identical shape, so releases are stale in reverse. Poke() has exactly one call site (commitLocked's eager arm), so claim/release commits also ride the 60s sync tick instead of getting T8's eager wire time — minor next to the views gap.

Empirical proof (refs/heads/tuhdoo, 2026-08-25, times -07:00): c65cdda 11:59:51 "1 events, 0 files" (claim.made, zero view files) → 1e57f18 11:59:51 "0 events, 1 files" (lease) → three lease renewals → bdbbe04 12:14:28 confirm → 6215087 12:15:46 "1 events, 151 files" (finish_run via commitLocked regenerates everything). Structural, not incidental: a work cycle is claim → silence → confirm → finish, so the in-progress window is precisely the window with no view-regenerating event. TUI/status read live daemon state, which is why this stayed invisible until someone watched GitHub mid-task.

Two constraints on any fix (surface at triage; fix shape has design surface — T6 "views land alongside their events" meets the claim protocol's "the lease lands in its own commit", may warrant a grill):
1. Naively adding stageViewsLocked() before the flush renders the WRONG state: at event-flush time the lease does not exist yet, and replay deliberately treats a leaseless claim as expired (ops.go:361 comment) — views at that instant still say ready. Regen must land after WriteLease: a third commit, or restructure so event + lease + views ride one commit (merge rules are per-file/per-area, so that looks legal, but it touches claim-protocol sequencing).
2. Lease EXPIRY has the same staleness with no natural trigger: an expired claim returns the task to the pool by pure replay — no event — so the branch keeps rendering in-progress until any unrelated event lands. Fixing claim/release does not close this half; it needs a timer- or sync-driven regen (the regen TRIGGER may be clock-driven in the daemon; views.Render itself stays pure — state is already time-evaluated at refresh).

Pointers: internal/daemon/ops.go:357 (claimTargetLocked), ops.go:458 (releaseLocked), internal/daemon/daemon.go:435 (commitLocked), daemon.go:460 (stageViewsLocked), internal/views/views.go:191 (In progress section), internal/syncer/syncer.go:24 (60s interval). Renewals correctly skip regen (rendered words don't change); opConfirmClaim renders nothing view-visible either.

## History

_No activity yet._
