# t-01KYTQTEQYGXVQ0QY7F95CHXVV — Principal identity override: stop deriving ugly actors from noreply emails

- Status: open — in progress, claimed by `4099114+brandonbews/claude-code-2`
- Priority: 1
- Labels: `daemon`, `identity`, `ux`
- Created: 2026-07-31 00:05 UTC by `4099114+brandonbews`

## Description

Context: D7 derives the human principal from the local part of git user.email. When user.email is a GitHub noreply address (Brandon's is), the derived principal is `4099114+brandonbews` — correct and traceable, but ugly everywhere it surfaces: claim rows in the TUI ('claimed by 4099114+brandonbews/…'), event actors on the data branch, acting-as headers. Observed live during the 2026-07-30 Cycle-4 dogfood session (see the claims on t-01KYT63MB28Z535SMJC9B0D83W).

The ask: a durable, low-ceremony way to set the preferred human principal once per repo (or per machine) so both the MCP shim's auto-derivation and the TUI's steer-mode actor use it without --as on every invocation. Likely shape: a key in the daemon/repo config read by the same code path that does git-email derivation (cmd/tuhdoo/repo.go — the derivation shared by the mcp shim and the TUI); --as still overrides everything. Consider whether the config lives in .git/tuhdoo/ (per-clone, not shared) or on the data branch (shared — but then it's per-human config in shared state; probably per-clone is right). Decide and document in 002 T-section or agent-protocol as appropriate.

Acceptance:
- With the override set, bare `tuhdoo` steer mode and `tuhdoo mcp` derive the configured principal; without it, behavior is unchanged (email local part).
- --as beats the override; validation (ValidateActor, root-vs-agent rules) still applies.
- Tests cover: override set, override absent, --as wins, invalid override rejected loudly.
- make test lint green; stored event bytes untouched.

Constraints: boring Go; no new dependencies; do not rewrite historical events — old actors stay as recorded.

## History

_No activity yet._
