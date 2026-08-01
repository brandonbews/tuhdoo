# CLI write verbs: a paved path when no MCP session exists

`t-01KYVEXK2BX040KJ244S2WP213`

- **Status:** done
- **Priority:** 1
- **Labels:** `cli` `dx`
- **Created:** 2026-07-31 06:48 UTC by `brandon/claude-code-4`

## Description

Context: observed during the 2026-07-30 dogfood session: deploy-after-landing restarts the daemon, which kills every live MCP session — including the deploying agent's — and the wrap-up acts (filing follow-up tasks, claiming newly-unblocked work, recording finish_run) come after deploy. Harnesses bind MCP at session start and do not reconnect, so the agent had to hand-roll a JSON-RPC script against the stdio shim to file tickets. This exact loop (agent rebuilds+restarts the daemon it is using) is mostly specific to developing tuhdoo itself, but the gap is broader: anything without a live MCP session — shell humans who prefer commands to the TUI, cron jobs, scripts — has no sanctioned write path today.

The ask: expose the write verbs through the CLI portal against the same daemon HTTP API the TUI steer mode uses — at minimum create_task (title, description from stdin or flag, priority, labels), update_task, and answer (escalations). Claim-lifecycle verbs (claim/finish_run) are optional and may be deliberately excluded — decide and document: the lease model is session-oriented and a CLI claim with no session to renew it may be wrong by design (if excluded, say so in the help text, do not leave it implied).

Acceptance: `tuhdoo create`/`tuhdoo update`/`tuhdoo answer` (or equivalent naming consistent with existing CLI verbs) perform the writes through the daemon with the same validation and actor derivation as the TUI (tuhdoo.principal override honored, --as wins); one-shot output conventions kept; CLI tests per cli_test.go patterns; docs: short section in 002 T7 or the CLI docs; make test lint green.

Constraints: the daemon stays the sole writer (D2) — the CLI talks HTTP to it, never touches the data branch; the eleven-verb MCP surface is unchanged (this is the human portal growing, not the agent surface); boring Go.

## History

### 2026-07-31 08:36 UTC — run by `brandon/claude-fable` — done

- Commits: `65200ff`

Commit 65200ff on main. tuhdoo create/update/answer land as one-shot CLI verbs over the daemon HTTP API (client gains writeResp; no daemon changes — all endpoints existed). Actor rules mirror TUI steer mode by reusing topActor: --as wins, tuhdoo.principal override honored, root human principal enforced. create: --desc text or - for stdin, --priority, --labels, --parents/--depends-on with short-ID resolution. update: partial by construction (only set flags are sent), plus --status; field-less update errors. answer: resolves by escalation ID or its task ID (what tuhdoo escalations prints); ambiguity lists candidates; the rest of the line is the answer. Claim lifecycle deliberately excluded per the task's offered option — help text owns the decision (leases are session-bound; a one-shot claim would lapse interrupted); T7 note added in 002-technology.md. Tests: write_cli_test.go — three integration tests through the real binary and daemon covering every verb, stdin desc, full-replacement labels, status curation, daemon validation surfacing, ambiguity, principal override, help contract. make test lint green.
