# pnpm/yarn install lines in docs + -h/--help on every subcommand

`tuh-01KZWX46MBVN8BHVMB7537364K`

- **Status:** open — ready
- **Priority:** 1
- **Labels:** `docs` `go` `cli`
- **Created:** 2026-08-13 06:32 UTC by `brandon`

## Description

Context: adopter report from an agent dogfooding `tuhdoo init` in a second repo; triaged at the 2026-08-21 grill (Brandon). Two frictions: (1) README and joining.md show only `npm i -D tuhdoo`; `pnpm add -D tuhdoo` works identically, but pnpm users hit the "do the os/cpu optional deps resolve?" question (they do — verified live by the adopter agent). (2) `tuhdoo init --help` errors with `unexpected argument "--help"` instead of printing usage. Grill decision: fix --help across ALL subcommands, not just init — one coherent behavior, muscle memory works everywhere.

The ask:
1. Docs: add pnpm (`pnpm add -D tuhdoo`) and yarn install lines next to the npm line in README.md (~line 17) and docs/joining.md (~line 51), with a brief note that the os/cpu-specific optional dependencies resolve correctly under pnpm.
2. CLI: every subcommand accepts `-h`/`--help` and prints that subcommand's usage to stdout, exit 0. Extract per-command usage from the global usage text (cmd/tuhdoo/main.go usage()) so there is one source of truth, not a second copy to drift. init keeps its bespoke friendly explanation for `--as` specifically (commands.go:45-56).

Acceptance:
- `tuhdoo <cmd> --help` and `tuhdoo <cmd> -h` exit 0 and print usage for every subcommand that takes input (init, task, create, update, answer, mcp at minimum); a table-driven test covers the full dispatch list.
- Both docs carry the pnpm/yarn lines with the optional-deps note.
- `make test lint` green; one PR.

Pointers: cmd/tuhdoo/main.go (dispatch ~line 28, usage() ~line 87), cmd/tuhdoo/commands.go:45 (runInit's argument rejection), README.md:17, docs/joining.md:51.

## History

### 2026-08-21 23:09 UTC — edit by `brandon/claude-code-1`

retitled · description edited · status inbox→open · priority 0→1 · labels +docs +go +cli
