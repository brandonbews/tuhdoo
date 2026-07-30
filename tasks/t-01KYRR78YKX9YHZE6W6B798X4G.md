# t-01KYRR78YKX9YHZE6W6B798X4G — Auto-derive session principals: git identity + daemon-minted agent names

- Status: open — ready
- Priority: 2
- Labels: `go`, `mcp`, `ux`
- Created: 2026-07-30 05:33 UTC by `brandon/migrator`

## Description

Context: today the shim requires --as <principal> in every harness config, which means identity is static config while the thing it names is a session — nobody will hand-name sessions, so the ledger degrades to one eternal name (or worse, invented root humans like the B11 field test's claude/field-1). D7 already points at the fix: human identity derives from git identity, and agent names are assigned when a harness connects over MCP.

The ask: make --as optional. When absent: (1) the shim derives the human part from git config user.email in the repo it runs in (local part, e.g. brandonbews@gmail.com -> brandonbews; document the rule); (2) the daemon mints the agent part at session bind from the MCP initialize clientInfo.name plus a per-daemon counter — e.g. brandon/claude-code-3 — unique among live sessions. --as stays as a full override for shared machines and scripted actors. Update docs/agent-protocol.md (Connecting section: the zero-config snippet becomes the primary example) and 002 T4's stdio-shim line, with a revision note per convention. Consider .mcp.json in this repo: drop its --as once auto-derivation works.

Acceptance: a harness config with no --as connects and writes events stamped <git-derived-human>/<clientinfo-name>-<n>; two concurrent no-flag sessions get distinct principals; --as still wins when present; invalid derived principals (empty email local part) fail loudly at connect, not silently; existing MCP/daemon tests green, make test lint green.

Pointers: cmd/tuhdoo/mcp_cmd.go (shim, --as handling), internal/daemon/mcp.go (session bind reads X-Tuhdoo-Actor at initialize; clientInfo is in the initialize params), docs/agent-protocol.md Connecting section, 002-technology.md T4.

Constraints: D7 holds — every principal is human or human/agent, agents always trace to a human root; principal validation rules stay as implemented (no spaces, at most one /); the ten-verb surface is untouched (T5); boring Go (T1).

## History

_No activity yet._
