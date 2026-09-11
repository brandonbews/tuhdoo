# TestMCPShimStdinDeathNamesStreamAndBytes fails locally under Go 1.27 (CI 1.26 green): stdin byte count 64 vs expected 154

`tuh-01M26JSHKSEQ0MJYBK1NK18X1S`

- **Status:** cancelled
- **Priority:** none
- **Labels:** `bug` `tests`
- **Created:** 2026-09-10 21:15 UTC by `brandon/claude-code-1`

## Description

Seen 2026-09-10 on main (541b56d) with go1.27.0 darwin/arm64: subtest dormant_digit_then_well-formed_line expects "delivered 154 bytes", shim reports 64. Deterministic 4/4. CI pins Go 1.26 and is green at the same HEAD. Likely a 1.27 change in stdin/JSON-decoder read buffering; the test asserts an implementation-dependent count. Triage: decide whether the shim's byte accounting or the test's expectation is wrong before the CI toolchain bumps.

## History

### 2026-09-11 22:56 UTC — note from `brandon/claude-code-1`

Cancelled at Brandon's direction 2026-09-11: resolved in PR #105 (tuh-01M26H99QVANVJCXR9FM2Z8Q7X). The shim's byte accounting was right; the test had encoded the old decoder's buffering (Go 1.27's v2-backed encoding/json issues 64-byte first reads). TestMCPShimStdinDeathNamesStreamAndBytes now asserts the contract bounds: at least the bytes that killed the decoder, at most everything written, tail still named.

### 2026-09-11 22:56 UTC — edit by `brandon/claude-code-1`

status inbox→cancelled
