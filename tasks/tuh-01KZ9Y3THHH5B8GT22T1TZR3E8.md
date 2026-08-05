# Clone-join: adopt an existing remote tuhdoo branch instead of minting a second root

`tuh-01KZ9Y3THHH5B8GT22T1TZR3E8`

- **Status:** open — in progress, claimed by `brandon/claude-code-1`
- **Priority:** 3
- **Labels:** `go` `store` `syncer`
- **Created:** 2026-08-05 21:43 UTC by `brandon/claude-code-1`

## Description

Context: pre-v0.2.0 audit (2026-08-05). store.Init checks only the local refs/heads/tuhdoo (internal/store/store.go:87-114; DefaultRef at :21), so a fresh clone of a repo whose remote already carries the branch mints a second, unrelated orphan root; convergence then rests on the app-level union merge across histories with no common ancestor — designed for it, but never executed on a real data branch (501 commits, 0 merges). This is the exact path a second machine hits when project #2 gets cloned. The true race — two machines initializing before either pushes — cannot be prevented by any check, so the unrelated-root merge must be proven anyway; adoption just makes the common case clean.

The ask: (1) at daemon startup, when the local ref is absent, check the configured remote (same default as the syncer: origin) for the branch before minting a root — if the remote has it, fetch and adopt it as the local ref; if there is no remote or no remote branch, mint exactly as today. Remoteless stays a normal state (T2), never an error, and startup must not hang on an unreachable remote (bound the attempt; on failure fall back to minting — the race-proof merge is the safety net). (2) A harness test proving the simultaneous-init race converges: two repos each init independently against one bare remote before either pushes, then sync — byte-identical replayed state and views on both sides, both roots preserved in history, no manual repair.

Acceptance: a test proving a fresh clone with a remote-carried branch adopts it (rev-list shows a single root, no second orphan commit); the race harness test green; existing remoteless-init idempotency tests untouched and green; `make test lint` green.

Pointers: internal/store/store.go:87-114, internal/syncer/syncer.go:18-20 and :79-80, cmd/tuhdoo/cli_test.go:210-289, the collision harness (PR #33) for harness patterns.

Constraints: host-agnostic — git protocol only (T2); no force-push on the data branch, ever; `tuhdoo init` must not assume a remote; boring Go; one PR.

## History

_No activity yet._
