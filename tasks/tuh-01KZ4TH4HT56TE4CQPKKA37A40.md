# Status.Collisions undercounts push contention — a lost ref-update race is not classified as non-fast-forward

`tuh-01KZ4TH4HT56TE4CQPKKA37A40`

- **Status:** inbox — untriaged capture
- **Priority:** 0
- **Labels:** `syncer` `t8`
- **Created:** 2026-08-03 22:04 UTC by `brandon/claude-code-1`

## Description

Found by the collision harness (task t-01KYRMFV10W1N28TCN5WVTCB1J).

When two peers push to the same bare repo at the same instant, git can reject one with `remote: error: cannot lock ref 'refs/heads/tuhdoo': is at X but expected Y` — a lost ref-update race rather than a stale history. `gitx.Push` (`internal/gitx/cli.go`) classifies a rejection as `ErrNonFastForward` only when git's output contains "non-fast-forward" or "fetch first", so this shape returns a generic error instead: `Syncer.Cycle` returns rather than going round its retry loop, the daemon records `mode=error`, and `Status.Collisions` never counts it.

The next cycle recovers so nothing is lost, but the push-contention counter T8 says the daemon keeps is an undercount, and the cycle takes a slower path (wait for the next tick) than the retry loop it should have entered. Observed once in a 40-burst storm.

Likely fix: recognise the ref-lock rejection as contention in `gitx.Push`'s classifier. Worth a test in `internal/gitx` over the literal git output.

## History

_No activity yet._
