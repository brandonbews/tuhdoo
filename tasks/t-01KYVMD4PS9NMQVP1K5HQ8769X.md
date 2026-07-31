# t-01KYVMD4PS9NMQVP1K5HQ8769X — finish_run accepts a claimless finish: opFinishRun never checks holdership

- Status: open — ready
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

_No activity yet._
