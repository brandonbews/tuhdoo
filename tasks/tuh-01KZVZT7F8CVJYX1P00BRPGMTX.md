# Collision harness bounded extension: natural-expiry arm, confirm-warning assertion, repeat-confirm check

`tuh-01KZVZT7F8CVJYX1P00BRPGMTX`

- **Status:** open — ready
- **Priority:** none
- **Labels:** `go` `audit-finding`
- **Created:** 2026-08-12 21:59 UTC by `brandon/claude-code-bg`

## Description

Context: Go-sweep audit finding, re-verified 2026-08-27; scope decided by Brandon at the triage grill (bounded extension). The harness has one fixed run mode (experiment(), harness/collision/main.go:263-328; flags at 98-107; the assertion set at 996-1216). Confirmed gaps in scope: (arm 4) natural lease-expiry synthesis of a never-reporting loser — the exact arm the 2026-08-04 resurrection bug lived in — is structurally unreachable: every silent close in the harness is release-triggered (main.go:563-573, 703-706) and LeaseTTL is never configured, so daemons run the 15-minute default (internal/daemon/daemon.go:35-36) against a ~4-minute run; (arm 5) claimNextOut (main.go:1684-1694) and claimTask's inline struct (1705-1709) decode no warning field, so "every claim response carries the confirm-before-merge warning" (emitted at mcp.go:390) is asserted nowhere end-to-end; (arm 6) confirm_claim is never re-called after success (retryGate, 766-780, re-calls only on a retryable 503), so verdict stability under repetition is unasserted.

The ask, in scope: (1) configurable LeaseTTL for harness-spawned daemons plus a natural-expiry arm — a loser goes silent, nobody releases, the lease lapses, and the harness asserts the synthesized close appears: the resurrection-bug regression, finally end-to-end; (2) decode the warning field in claim responses and assert every claim carries it; (3) after a successful confirmation, call confirm_claim again and assert the verdict is stable; (4) cosmetics: retire the 7 residual 'verb' diagnostic literals (harness/collision/main.go:541, 991, 1200, 1271 — the original audit misattributed these to cmd/tuhdoo/main.go — plus npm/smoke.sh:76, 111 and harness/README.md:85) and document -spare (defined main.go:99) in the README flag list (README:17-21 omits it).

Out of scope, recorded in the README: remote-severing and remoteless arms stay unit-covered — cite gate_test.go:119-138 (remote-unreachable refusal), gate_test.go:83 (remoteless confirmation), loser_test.go:223-228 and 344-361 (late-loser messaging).

Acceptance: a harness run exercises the natural-expiry arm and passes; the warning and repeat-confirm assertions join the check list; README documents -spare and the covered/not-covered D6 arms; the verb literals are gone; make test lint green, and a full harness run recorded in the PR body.

Constraints: keep the harness single-binary and boring; LeaseTTL plumbing must not change daemon defaults outside the harness.

## History

### 2026-08-27 15:22 UTC — edit by `brandon/claude-code-1`

retitled · description edited · status inbox→open · labels +go
