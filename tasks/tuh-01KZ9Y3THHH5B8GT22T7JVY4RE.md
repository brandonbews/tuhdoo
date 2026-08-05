# init hardening: loud unknown-flag errors and the MCP snippet in init output

`tuh-01KZ9Y3THHH5B8GT22T7JVY4RE`

- **Status:** open — in progress, claimed by `brandon/claude-code-1`
- **Priority:** 2
- **Labels:** `cli`
- **Created:** 2026-08-05 21:43 UTC by `brandon/claude-code-1`

## Description

Context: pre-v0.2.0 audit (2026-08-05). `tuhdoo init --as brandon` is silently ignored — main.go:36 calls runInit() and discards the remaining args, so a user believes they configured something and didn't. And init's output (cmd/tuhdoo/commands.go:64-76) prints the CI path-filter snippet but not the MCP harness config, which otherwise only lives in docs — "init + docs alone" is v1 clause 3's onboarding bar, and project #2's first act after init is wiring an agent.

The ask: (1) init parses its args; any flag → loud error with usage. --as specifically gets a helpful rejection: init writes no events; the principal comes from git identity or `git config tuhdoo.principal`. (2) Add the universal MCP harness snippet (command: tuhdoo, args: [mcp]) to init's output alongside the CI snippet.

Acceptance: cli_test covers the unknown-flag error and the --as-specific message; init output contains the MCP snippet; existing init tests (idempotent, remoteless) green; `make test lint` green.

Pointers: cmd/tuhdoo/main.go:36, cmd/tuhdoo/commands.go:40-78, cmd/tuhdoo/cli_test.go:210-232, docs/design/002-technology.md T4 (the snippet's current home).

Constraints: init must not assume a remote (T2); output stays plain and line-oriented (T7); boring Go; one PR.

## History

_No activity yet._
