# Replay letter-vs-code edges: holder's late finish after uncontested expiry; empty-task events malformed despite subject-less envelope support

`tuh-01KZVZT7F8CVJYX1NZZWMSKKDT`

- **Status:** inbox — untriaged capture
- **Priority:** 0
- **Labels:** `audit-finding`
- **Created:** 2026-08-12 21:59 UTC by `brandon/claude-code-bg`

## Description

Go-sweep audit findings, internal/core. (1) Replay evaluates a holder's lease only at a competing claim.made (replay.go ~290) and the final check (~117-126): a run.finished(done) from a holder whose lease lapsed long before (no competing claim) still lands done and suppresses interrupted synthesis. The loser-side analog is designed and tested (TestVoidedClaimClosedByRealRunSkipsSynthesis); the holder-side behavior is arguably right ('real close wins') but no clause or test owns it — a table row would pin it either way. (2) apply treats e.Task=="" as malformed for every event type (replay.go ~176-178) while the event package explicitly supports subject-less events (event.New doc, Encode omits task, tested) — harmless until the first subject-less catalog type hits the wall. One sentence in either package, or reconcile.

## History

_No activity yet._
