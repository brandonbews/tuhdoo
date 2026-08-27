# Police escalation.answered task mismatch as malformed, matching claim.confirmed

`tuh-01KZVZT7F8CVJYX1NZZV9YNXSM`

- **Status:** done
- **Priority:** none
- **Labels:** `go` `core` `audit-finding`
- **Created:** 2026-08-12 21:59 UTC by `brandon/claude-code-bg`

## Description

Context: Go-sweep audit finding, re-verified 2026-08-27. claim.confirmed whose envelope task mismatches the named claim's task is ErrMalformedEvent (internal/core/replay.go:315-318; pinned by TestConfirmationTaskMismatchIsMalformed, confirm_test.go:227-240). escalation.answered never compares esc.Task to e.Task (replay.go:413-436, existence check only at 418-421) — a mismatched answer mutates state silently, and no test anywhere covers the shape. Both are "something wrote garbage" shapes; T3's fail-safe posture (fail loudly, never best-effort) decides the direction: police both. Compat is safe for the same reason the claim.confirmed check was: no writer has ever produced a mismatched escalation.answered (the daemon derives the task from the escalation), so the added strictness cannot brick a real branch.

The ask: reject escalation.answered whose envelope task mismatches the named escalation's task as ErrMalformedEvent, mirroring the claim.confirmed check.

Acceptance: a table row mirroring TestConfirmationTaskMismatchIsMalformed for escalation.answered; the test comment records why tightening is compat-safe; existing suite green; make test lint green.

Constraints: T3 — new strictness may only reject shapes no writer produces; stored event bytes untouched. The audit also noted nothing checks that escalation-bearing events name an existing task at all — if the implementer finds more of that family, capture it fresh rather than scope-creep this task.

## History

### 2026-08-27 07:06 UTC — edit by `brandon/claude-code-1`

retitled · description edited · status inbox→open · labels +go +core

### 2026-08-27 08:24 UTC — run by `brandon/claude-code-2` — done

- Branch: `tuh-nxsm/answered-task-mismatch`
- PR: <https://github.com/brandonbews/tuhdoo/pull/95>
- Merged as: `5325625`

Landed via PR #95 (squash 5325625). Replay now rejects escalation.answered whose envelope task mismatches the named escalation's task as ErrMalformedEvent, a four-line check mirroring claim.confirmed's. TestAnswerTaskMismatchIsMalformed mirrors the confirmation test; its comment records the compat-safety argument (no writer ever produced the mismatch — the daemon derives the task from the escalation). The audit's adjacent family note: escalation.raised already polices unknown tasks; nothing further surfaced in scope, so no fresh capture needed. make test lint green. Binary changed: rebuilt and daemon restarted post-finish.
