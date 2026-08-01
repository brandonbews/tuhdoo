# Principal identity override: stop deriving ugly actors from noreply emails

`t-01KYTQTEQYGXVQ0QY7F95CHXVV`

- **Status:** done
- **Priority:** 1
- **Labels:** `daemon` `identity` `ux`
- **Created:** 2026-07-31 00:05 UTC by `4099114+brandonbews`

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

### 2026-07-31 05:30 UTC — run by `4099114+brandonbews/claude-code-2` — done

- Branch: `main`
- Commits: `10a48e3`

Override lands as `git config tuhdoo.principal <name>` — chosen over a .git/tuhdoo/ file and over data-branch state (per-human per-clone preference doesn't belong in shared fleet state): one familiar command, survives .git/tuhdoo wipes, per-machine reach free via --global with ordinary git precedence, zero new file formats. Implemented inside the single shared derivation (gitEmailLocalPart in cmd/tuhdoo/repo.go), so the mcp shim and TUI steer mode both pick it up with no call-site changes and --as wins automatically (callers only derive when no explicit principal given). Set-but-invalid values (empty, agent-shaped with /, fails ValidateActor) error loudly naming the config key — never a silent fall-back to email derivation. Tests in cmd/tuhdoo/repo_test.go: unit derivation table (override set/absent/unset restores email rule/invalid values), TUI steer-mode actor (override honored, --as steve wins, agent-shaped rejected), and end-to-end shim (no-flag session creates as brandon/claude-code-1 with daemon-minted agent half; --as steve/agent-7 beats override; invalid override kills the shim non-zero at connect). Documented as a dated bullet in 002 T4. Stored events untouched — old actors stay as recorded. To use it here: `git config tuhdoo.principal brandon` in this clone (takes effect for new sessions after the daemon restart that deploys this).
