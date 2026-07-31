# t-01KYVMD4PS9NMQVP1K5M1PVRD8 — Shim died once: 'stdio session: invalid trailing data at the end of stream'

- Status: open — ready
- Priority: 0
- Labels: `mcp`, `daemon`, `investigation`
- Created: 2026-07-31 08:24 UTC by `brandon/claude-fable`

## Description

Context: 2026-07-31 ~08:19 UTC, dogfooding. A live shim session (bin/tuhdoo mcp --as brandon/claude-fable, stdin fed from a fifo, ~2 minutes old, one successful claim_next) exited 1 with exactly that stderr line at the moment a well-formed ~1.5KB tools/call line was written to the fifo. The identical bytes replayed against a fresh fifo-fed session did NOT reproduce; the phrase does not occur in tuhdoo source, so it likely comes from the MCP Go SDK transport (partial line at stream end) — but which stream (stdin vs the daemon bridge) is unknown. A hard shim death silently stops lease renewal, so unexplained session deaths are costly in the field.

The ask: find what emits this error (SDK stdio transport vs daemon-session bridge in cmd/tuhdoo/mcp_cmd.go); establish whether a partial stdin write, a daemon-side disconnect, or an SDK framing edge can trigger it; make the shim's death message say which stream ended and with how much unconsumed data. Fix any real bug found; if it is unreproducible SDK behavior, document the conclusion in the finish summary and land only the diagnostic improvement.

Acceptance: root cause identified with a repro or a defensible written explanation in the finish summary; improved stderr diagnostics covered by a test if wording changes; make test lint green.

Pointers: cmd/tuhdoo/mcp_cmd.go; the MCP Go SDK stdio/ndjson transport sources in the module cache; docs/agent-protocol.md Connecting (session-death guidance).

Constraints: boring Go; no protocol or verb changes.

## History

_No activity yet._
