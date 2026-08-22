# Status.Collisions undercounts push contention — a lost ref-update race is not classified as non-fast-forward

`tuh-01KZ4TH4HT56TE4CQPKKA37A40`

- **Status:** done
- **Priority:** 1
- **Labels:** `syncer` `t8`
- **Created:** 2026-08-03 22:04 UTC by `brandon/claude-code-1`

## Description

Promoted from inbox 2026-08-05 (release grill; Brandon confirmed the promotion — clear fix, clear test, no embedded judgment calls). Feeds the v0.2.0 cut task (tuh-01KZ9Y3THHH5B8GT22T910R40K), which depends on it.

Context: found by the collision harness (task t-01KYRMFV10W1N28TCN5WVTCB1J). When two peers push to the same bare repo at the same instant, git can reject one with `remote: error: cannot lock ref 'refs/heads/tuhdoo': is at X but expected Y` — a lost ref-update race rather than a stale history. `gitx.Push` (`internal/gitx/cli.go`) classifies a rejection as `ErrNonFastForward` only when git's output contains "non-fast-forward" or "fetch first", so this shape returns a generic error instead: `Syncer.Cycle` returns rather than going round its retry loop, the daemon records `mode=error`, and `Status.Collisions` never counts it.

The next cycle recovers so nothing is lost, but the push-contention counter T8 says the daemon keeps is an undercount, and the cycle takes a slower path (wait for the next tick) than the retry loop it should have entered. Observed once in a 40-burst storm.

The ask: recognise the ref-lock rejection as contention in `gitx.Push`'s classifier so it returns `ErrNonFastForward`, the syncer retry loop engages, and `Status.Collisions` counts it.

Acceptance: a table-driven test in `internal/gitx` over the literal git output shape above (and the existing non-fast-forward / fetch-first shapes) proving classification; existing syncer tests green; `make test lint` green.

Constraints: classifier change only — no retry-loop behavior changes; boring Go; one PR.

## History

### 2026-08-05 21:44 UTC — edit by `brandon/claude-code-1`

description edited · status inbox→open · priority none→1

### 2026-08-05 22:55 UTC — run by `brandon/claude-code-1` — done

- Branch: `tuh-7a40/push-contention-classifier`
- PR: <https://github.com/brandonbews/tuhdoo/pull/43>

Landed via PR #43 (squash-merged, checks green). Push now captures stderr and classification lives in pushRejectionIsContention(stdout, stderr): the ref-lock race shape (remote: cannot lock ref ... is at X but expected Y — which arrives on stderr, why the old stdout-only check missed it) now returns ErrNonFastForward, so the syncer retry loop engages and Status.Collisions counts it. Table-driven test over the literal shapes including the harness verbatim line. Classifier only; retry loop untouched.
