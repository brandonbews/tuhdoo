# t-01KYVMD4PS9NMQVP1K5HQ8769X — finish_run accepts a claimless finish: opFinishRun never checks holdership

- Status: done
- Priority: 1
- Labels: `daemon`, `protocol`, `bug`
- Created: 2026-07-31 08:24 UTC by `brandon/claude-fable`

## Description

Context: found dogfooding 2026-07-31. A finish_run sent by brandon/claude-fable with no claim and no open run on t-01KYVD31CNTR1EVCDHPJGSQAGH was accepted and minted run 01KYVM8KY10HP90YNBXMQP8ZCV — a duplicate done run with an empty claim ref, now permanently on the ledger. opFinishRun (internal/daemon/ops.go ~398) validates only task existence and outcome; any principal can close a run on any task at any time, which also means a mistyped task ID silently fabricates a run record.

The ask: reject finish_run at the write side when the acting principal holds no live claim on the task, mirroring the holdership check pattern release_claim uses. CAREFUL: the daemon's lease-expiry path synthesizes interrupted runs — find how that path writes (shared op or direct) and keep it working; the MCP layer already rejects interrupted/superseded (mcp.go ~456, documented as intentional for the HTTP-shared op), so the new check must compose with that, not replace it.

Acceptance: table-driven daemon-op tests: finish_run with no claim on the task is rejected; with a live claim held by a different principal is rejected; with the caller's own live claim succeeds; the synthesized-interrupted path still works (existing lease-expiry tests stay green). Replay of the existing ledger — which now contains a claimless run.finished event — must stay green: this is write-side validation only, never a replay rule (T3: stored events are never rewritten and never retro-invalidated). make test lint green.

Pointers: internal/daemon/ops.go opFinishRun (~398) and opReleaseClaim (holdership pattern); internal/daemon/mcp.go ~450 (finish_run tool, outcome filtering comment); run 01KYVM8KY10HP90YNBXMQP8ZCV on t-01KYVD31CNTR1EVCDHPJGSQAGH is the live evidence.

Constraints: boring Go; deterministic core untouched; stored events untouched.

## History

### 2026-07-31 09:21 UTC — run by `brandon/claude-code-1` — done

- Branch: `main`
- Commits: `7d46356`

opFinishRun now refreshes leases at the current instant (same posture as release_claim) and runs a new finishGuardLocked before writing run.finished. DELIBERATE DEVIATION from the literal ask, flagged for review: a strict live-claim-only check would break the settled blocking protocol (T5 / agent-protocol.md: escalate → release_claim → finish_run blocked arrives with NO live claim), so the rule is "close only your own attempt, once" — admitted shapes: (1) actor holds the live claim; (2) actor's latest claim on the task ended released and no later run of theirs closed it; (3) same for voided (race loser recording superseded while the winner holds — the replay.go:267-blessed flow). Rejected: no claim history (409 — kills the reported bug, run 01KYVM8KY1...), live claim held by another (403 naming the holder), expired claim (409 — replay already synthesized interrupted), double-close (409), unknown task (404 — mistyped IDs fabricate nothing). Lease-expiry interrupted runs are synthesized purely at replay time in internal/core and never pass through opFinishRun (only callers: api.go, mcp.go), so synthesis is untouched; MCP outcome filter composes (runs first). internal/core zero-diff, so replay of the existing claimless run.finished on the ledger stays green — write-side validation only (T3). New table-driven TestFinishRunGuard (9 rows incl. blocked protocol, race-loser superseded, expired-lease synthesis) + TestReplayAcceptsStoredClaimlessRun. Known residual: a released-claim close accepts any catalog outcome, not just blocked — outcome/status coupling deliberately not added; tighten later if wanted. make test lint green; commit 7d46356 on main, pushed. Daemon NOT restarted yet — the running binary still accepts claimless finishes until the end-of-night redeploy.
