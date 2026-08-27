# Pin replay edges: holder's late finish counts as done; reconcile the subject-less-event stance

`tuh-01KZVZT7F8CVJYX1NZZWMSKKDT`

- **Status:** open — in progress, claimed by `brandon/claude-code-2`
- **Priority:** none
- **Labels:** `go` `core` `audit-finding`
- **Created:** 2026-08-12 21:59 UTC by `brandon/claude-code-bg`

## Description

Context: Go-sweep audit finding, re-verified 2026-08-27 with one correction: the holder-late-finish behavior IS mechanically pinned, but only incidentally — TestFinishRunDone (replay_test.go:239-261) replays with nil leases, which under replay.go:122-127/496-499 is identical to lapsed, and its assertions would fail if interrupted synthesis were ever added (same incidental pin in TestFinishRunMergedAs and TestFinishRunDoneStampsClose). What's missing is INTENT: no test names the case "holder's lease lapsed long ago, no competing claim, run.finished(done) still lands done" — the branch at replay.go:384-394 takes no lease into account (lease reads exist only at 141, 291, and 114-127), and the loser-side analog has a named test (TestVoidedClaimClosedByRealRunSkipsSynthesis). (2) apply rejects e.Task == "" for every event type (replay.go:177-179) while the event package canonically supports subject-less events (event.go:19, 29-31, 48-53; TestEncodeOmitsEmptyTaskAndAbsentSig mints one) — the first subject-less catalog type hits the wall as malformed and fail-stops.

The ask (no behavior change): (1) add a named table row: explicitly past-dated holder lease, no competing claim, run.finished(done) → lands done, no interrupted synthesis — "a real close wins over a lapsed lease"; (2) reconcile the subject-less stance with one sentence in each package: replay rejects subject-less events because every current catalog type has a subject; event.New's subject-less support is for future types, which must be given an apply path when they arrive.

Acceptance: the named test row exists; doc sentences in internal/core and internal/event; behavior identical (no production diffs beyond comments); make test lint green.

## History

### 2026-08-27 07:06 UTC — edit by `brandon/claude-code-1`

retitled · description edited · status inbox→open · labels +go +core
