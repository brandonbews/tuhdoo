# Collision harness bounded extension: natural-expiry arm, confirm-warning assertion, repeat-confirm check

`tuh-01KZVZT7F8CVJYX1P00BRPGMTX`

- **Status:** done
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

### 2026-09-11 17:47 UTC — edit by `brandon`

priority none→2

### 2026-09-11 21:51 UTC — edit by `brandon`

priority 2→none

### 2026-09-12 00:19 UTC — run by `brandon/claude-code-1` — done

- Branch: `tuh-01KZVZT7F8CVJYX1P00BRPGMTX/harness-bounded-extension`
- PR: <https://github.com/brandonbews/tuhdoo/pull/112>
- Merged as: `1980d2f`

Landed as PR #112 (squash 1980d2f on main). Natural-expiry arm (-expiry-contests, -lease-ttl default 4m via TUHDOO_LEASE_TTL read by tuhdoo daemon; unset = today's default), confirm-warning assertion on every claim response, repeat-confirm stability check, verb literals retired, README documents -spare and the unit-covered D6 arms with corrected line citations. Two prerequisites beyond the ask: the harness was ported from the retired GET /v0/state to /v0/snapshot, and the first run exposed a real store bug (tombstones truncated to the second re-adjudicated same-second contests) landed separately as PR #111 / tuh-01M29FENAVT4FSGZR3W2D3NQ54. Recorded run: 20 checks passed, 0 failed, 4m08s, 394 events, identical trees. Deploy follows this finish.
