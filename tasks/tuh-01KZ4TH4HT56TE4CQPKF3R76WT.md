# Nothing writes the superseded run D6 promises — voided claims leave no run at all

`tuh-01KZ4TH4HT56TE4CQPKF3R76WT`

- **Status:** inbox — untriaged capture
- **Priority:** 0
- **Labels:** `daemon` `d6`
- **Created:** 2026-08-03 22:04 UTC by `brandon/claude-code-1`

## Description

Found by the collision harness (task t-01KYRMFV10W1N28TCN5WVTCB1J, PR #25), running real two-machine claim races for the first time.

D6 clause 2 says the losing daemon tells its agent to stand down and the half-done work is recorded as a Run with outcome `superseded` (branch name included). Nothing implements the second half:

- `internal/core/replay.go` voids the loser's claim (`ClaimVoided`) and stops there. The only run replay ever synthesizes is `interrupted`, for lease expiry.
- `internal/daemon/mcp.go` rejects `superseded` from agents on the grounds that it is daemon-synthesized — but no daemon code synthesizes it.
- There is no CLI verb (the work loop is deliberately session-only, T7).
- The only surface that accepts it is `POST /v0/runs` on the unix socket, and `finishGuardLocked` (`internal/daemon/ops.go`) explicitly anticipates "a race loser recording superseded work over HTTP, possibly while the winner still holds".

So the shape is designed for, guarded for, and has no writer. In a real fleet today a race loser's claim is silently voided and no run is recorded — the branch its agent was working on is not on the ledger anywhere.

The harness passes its acceptance line only because it plays the losing daemon's part itself over that HTTP surface (`settle()` in harness/collision/main.go).

Open questions for triage: should replay synthesize the superseded run the way it synthesizes `interrupted` (pure, no branch known — where would the branch come from?), or should the daemon write it on noticing its own claim was voided (knows the session, may know the branch)? Or is the branch simply not recoverable, and the honest fix is a branch-less superseded run plus a doc revision to D6? Reproduce with `go run ./harness/collision`.

## History

_No activity yet._
