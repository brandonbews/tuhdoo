# Shim died once: 'stdio session: invalid trailing data at the end of stream'

`t-01KYVMD4PS9NMQVP1K5M1PVRD8`

- **Status:** done
- **Priority:** 0
- **Labels:** `mcp` `daemon` `investigation`
- **Created:** 2026-07-31 08:24 UTC by `brandon/claude-fable`

## Description

Context: 2026-07-31 ~08:19 UTC, dogfooding. A live shim session (bin/tuhdoo mcp --as brandon/claude-fable, stdin fed from a fifo, ~2 minutes old, one successful claim_next) exited 1 with exactly that stderr line at the moment a well-formed ~1.5KB tools/call line was written to the fifo. The identical bytes replayed against a fresh fifo-fed session did NOT reproduce; the phrase does not occur in tuhdoo source, so it likely comes from the MCP Go SDK transport (partial line at stream end) — but which stream (stdin vs the daemon bridge) is unknown. A hard shim death silently stops lease renewal, so unexplained session deaths are costly in the field.

The ask: find what emits this error (SDK stdio transport vs daemon-session bridge in cmd/tuhdoo/mcp_cmd.go); establish whether a partial stdin write, a daemon-side disconnect, or an SDK framing edge can trigger it; make the shim's death message say which stream ended and with how much unconsumed data. Fix any real bug found; if it is unreproducible SDK behavior, document the conclusion in the finish summary and land only the diagnostic improvement.

Acceptance: root cause identified with a repro or a defensible written explanation in the finish summary; improved stderr diagnostics covered by a test if wording changes; make test lint green.

Pointers: cmd/tuhdoo/mcp_cmd.go; the MCP Go SDK stdio/ndjson transport sources in the module cache; docs/agent-protocol.md Connecting (session-death guidance).

Constraints: boring Go; no protocol or verb changes.

## History

### 2026-07-31 08:37 UTC — note from `brandon/claude-fable`

Second occurrence, richer evidence. Setup: shim on a fifo (bin/tuhdoo mcp --as brandon/claude-fable < fifo), a `sleep 86400 > fifo` holding the write end. Timeline (local): 01:24 session up, 01:25 claim_next round-trips fine (out mtime 01:25), then NOTHING is written to the fifo; 01:31 the shim dies with the same one-line stderr (err mtime 01:31) — six minutes after the last write, with the sleep writer STILL ALIVE (verified via ps afterwards) and the fifo intact. The 01:31 window coincides with `go test ./cmd/tuhdoo` running the CLI/MCP integration suites (they spawn their own daemons and shims in temp repos and SIGKILL their own pids — no obvious cross-talk, but the coincidence repeated across both occurrences). A byte-identical replay of the pending request against a fresh fifo shim did NOT reproduce. Since the SDK error fires only when a decoded JSON value is followed by a non-newline byte in the stdin decoder buffer (go-sdk v1.7.0 mcp/transport.go:474 newIOConn), something delivered non-newline bytes after a value — or the decode path misfired on an idle stream. Also worth checking: whether srv.Run's error here can originate from the DAEMON-side bridge rather than stdin, which would re-frame all of this. One-shot sessions (printf lines | shim) have never failed across ~8 uses.

### 2026-07-31 09:36 UTC — run by `brandon/claude-code-1` — done

- Branch: `main`
- Commits: `e99854c`

Root cause found WITH a live repro; no tuhdoo bug. The error comes from go-sdk v1.7.0 mcp/transport.go:474 (ndjson decode loop) and can only fire on the shim's OWN STDIN — the daemon bridge is SSE/HTTP (StreamableClientTransport) and takes the separate "daemon session ended" path. Mechanism: the SDK errors only when one decoder chunk holds a complete JSON value immediately followed by a non-newline byte. The killer shape: a stray digit-led byte (e.g. a bare `7`, or a Go-log-style `2026/07/31 ...` line — json.Decoder parses `2026` then chokes on `/`) parks the decoder silently mid-number; the NEXT well-formed line — arbitrarily later — completes the number and itself becomes the "trailing data". That matches occurrence 1 exactly (death at the instant of a valid 1.5KB write; byte-identical replay clean because the replay lacked the parked digit). Occurrence 2's err-mtime proves bytes hit the fifo at 01:31 despite no known writer — so both occurrences reduce to a second writer putting digit-led junk into the fifo. The go-test coincidence was audited and cleared: the cmd/tuhdoo suites pin cwd to MkdirTemp repos, use no fifos, kill only their own pids. Live repro performed in a temp repo with its own daemon: parked `7`, then a valid tools/list line → exact field error. Landed: stdinRecorder in cmd/tuhdoo/mcp_cmd.go (total bytes + 256-byte tail, via mcp.IOTransport with a nop-close stdout wrapper) so the death message now names the stream ("harness stdin, not the daemon bridge") and shows the recent bytes %q-escaped — on this error class the tail contains the junk, which identifies the guilty writer on a third occurrence; daemon-side message clarified symmetrically (existing "daemon session ended" substring preserved for tests/docs); TestMCPShimStdinDeathNamesStreamAndBytes covers both field mechanisms end-to-end; one bullet added to docs/agent-protocol.md on the stdin-side death and the fifo-second-writer hazard. Watch item: if it fires a third time, read the %q tail in stderr — it names the culprit. make test lint green; commit e99854c on main, pushed.
