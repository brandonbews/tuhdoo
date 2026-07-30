# t-01KYRR78YKX9YHZE6W6B798X4G — Auto-derive session principals: git identity + daemon-minted agent names

- Status: open — in progress, claimed by `brandon/impl-2`
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

### 2026-07-30 05:56 UTC — note from `brandon/impl-2`

Design settled; key wrinkle and the plan:

The daemon's MCP session is with the shim, whose own clientInfo.name is the constant "tuhdoo-shim" — the harness's clientInfo only reaches the shim later, over stdio, in the harness's initialize request. So the shim must be restructured from "connect to daemon, then serve stdio" to "serve stdio, bind the daemon session inside an initialize middleware": AddReceivingMiddleware intercepts method "initialize", reads params.ClientInfo.Name, connects to the daemon then, mirrors the tools before the initialize response goes out (jsonrpc clients can't send tools/list until initialize returns, so no race), and overwrites the initialize result's Instructions with the daemon's (previously passed at NewServer time). go-sdk v1.7.0 has everything needed: AddReceivingMiddleware, req.GetParams() -> *InitializeParams, Session.InitializeParams().

Transport contract: auto mode sends X-Tuhdoo-Actor: <human> plus new header X-Tuhdoo-Agent-Name: <harness clientInfo.name>; the agent-name header is the daemon's signal to mint <human>/<sanitized-name>-<n>. --as mode sends the full principal in X-Tuhdoo-Actor only, exactly as today. Daemon getServer: agent-name header present -> actor must be a root human (no slash), sanitize name (lowercase, non [a-z0-9._-] runs -> "-", fallback "agent"), per-name monotonic counter under a small dedicated mutex (agentSeq map). Monotonic, not reused-after-free: distinct sessions keep distinct names for the daemon's lifetime, which is stronger than the "unique among live sessions" acceptance and better for ledger attribution; counter resets on daemon restart (acceptable, noting it in docs).

Human derivation shared with tuhdoo top: extracting a gitEmailLocalPart helper into repo.go; topActor (top.go, landed in fa8c7d3) switches to it. Shim derives at startup before ensureDaemon, so a missing/empty user.email fails loudly at exit 1 with the rule + "--as" remedy in the message (acceptance: fail at connect, not silently).

Behavior change to an existing test: `tuhdoo mcp` with no --as currently exits "usage"; it becomes auto mode (TestMCPShimRejectsBadPrincipal's no-flag case will assert the derive-failure path instead, since the hermetic test env has no user.email).

.mcp.json: dropping --as per the task. Note for Brandon: this repo's git user.email is the GitHub noreply form, so sessions here will stamp "4099114+brandonbews/<client>-<n>" until he sets a repo-local user.email; flagging in the finish summary.

Files: internal/daemon/mcp.go (+daemon.go field), cmd/tuhdoo/mcp_cmd.go (restructure), cmd/tuhdoo/repo.go, cmd/tuhdoo/top.go (reuse helper), cmd/tuhdoo/main.go usage, .mcp.json, docs/agent-protocol.md Connecting, docs/design/002-technology.md T4 revision note, tests in internal/daemon/mcp_test.go + cmd/tuhdoo/mcp_cli_test.go. Nothing written yet.
