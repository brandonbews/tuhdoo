# Undesigned strictness asymmetry: escalation-answer task mismatch accepted, confirmation task mismatch malformed

`tuh-01KZVZT7F8CVJYX1NZZV9YNXSM`

- **Status:** inbox — untriaged capture
- **Priority:** 0
- **Labels:** `audit-finding`
- **Created:** 2026-08-12 21:59 UTC by `brandon/claude-code-bg`

## Description

Go-sweep audit finding, internal/core. claim.confirmed whose envelope task mismatches the named claim's task is ErrMalformedEvent (replay.go ~314-317, pinned by TestConfirmationTaskMismatchIsMalformed). escalation.answered whose envelope task mismatches the named escalation's task is silently accepted (replay.go ~412-435 never compares esc.Task to e.Task). Both are 'something wrote garbage' shapes under T3's fail-safe posture; only one is policed. Not wrong by any written clause — the asymmetry is undesigned. Decide the posture and pin it.

## History

_No activity yet._
