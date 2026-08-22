# Collision harness: unexercised D6 arms and unasserted claim-response warning

`tuh-01KZVZT7F8CVJYX1P00BRPGMTX`

- **Status:** inbox — untriaged capture
- **Priority:** none
- **Labels:** `audit-finding`
- **Created:** 2026-08-12 21:59 UTC by `brandon/claude-code-bg`

## Description

Go-sweep audit finding, harness/collision. D6 promises with no harness check: (1) remote-unreachable honest refusal (gate refuses retryably, never guesses) — the harness never severs the remote; (2) remoteless-normal confirmation soundness — every run has an origin; (3) late-returning loser told its attempt is closed and pointed at add_note / one-close-per-attempt under duplicate report; (4) natural lease-expiry-driven synthesis of a never-reporting loser (all harness synthesis is release-triggered; 15-min TTL out of reach of a 4-min run) — the exact arm the 2026-08-04 resurrection bug lived in; (5) 'every claim response carries the confirm-before-merge warning' — claimNextOut decodes no warning field and nothing asserts it; (6) repeated confirm_claim idempotency (dup==0 catches a duplicate event, not a wrong answer). Also cosmetic: 7 residual 'verb' diagnostic string literals (main.go ~557,1030,1243,1314; npm/smoke.sh ~76,111; harness/README.md ~85 quotes one) left by the vocabulary task's string-freeze, and README omits the -spare flag (main.go ~99). Extend the harness (a run mode or two) or accept and record the scope.

## History

_No activity yet._
