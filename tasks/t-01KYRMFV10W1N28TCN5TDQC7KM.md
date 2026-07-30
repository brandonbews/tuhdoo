# t-01KYRMFV10W1N28TCN5TDQC7KM — Grow watch into the interactive steering TUI (tuhdoo top)

- Status: done
- Priority: 3
- Labels: `go`, `tui`
- Parents: [t-01KYRMFV10W1N28TCN5SH4QM7A](t-01KYRMFV10W1N28TCN5SH4QM7A.md)
- Created: 2026-07-30 04:28 UTC by `brandon/migrator`

## Description

Context: 002 T7 / 001 D8 — v1's steering surface is a TUI grown from the read-only watch skeleton, shipped in the same binary as `tuhdoo top`. watch was built deliberately as the TUI's skeleton with interactivity amputated.

The ask: add input handling to the watch view loop: answer an escalation, reprioritize the queue, cancel/archive a task. Human/admin verbs already exist on the daemon HTTP API; the TUI speaks that API — no new write paths.

Acceptance: a blocking escalation raised by an agent can be answered from the TUI and the task returns to the ready pool; a reprioritize is visible to a subsequent get_backlog; cancel/archive lands with the acting human principal. Tests follow the project pattern: pure state->render functions with table-driven tests; interaction logic covered without a live terminal where possible. make test lint green.

Pointers: cmd/tuhdoo/watch.go (the skeleton), cmd/tuhdoo/render.go, internal/daemon/api.go (admin verbs), internal/daemon/ops.go.

Constraints: boring Go (T1); TUI writes go through the daemon HTTP API only — never git, never the ops layer directly; the agent MCP surface stays at ten verbs (T5).

## History

### 2026-07-30 05:42 UTC — note from `brandon/impl-2`

Design settled after reading cmd/tuhdoo (watch.go, render.go, snapshot.go, client.go, commands.go) and internal/daemon (api.go, ops.go). Plan:

- New `tuhdoo top` subcommand in cmd/tuhdoo/top.go; `watch` stays untouched as the read-only pane. top reuses watch's tick/fetch messages and render.go helpers; selection/interactivity lives in a new topModel.
- Rows: pure buildRows(snapshot) -> flat []topRow (open escalations, then ready/in-progress/blocked open tasks); cursor is an index into that; selection kept stable across 2s refreshes by re-finding the row id.
- Keys: j/k or arrows move; a = answer (escalation rows), p = reprioritize (task rows), c = cancel with y/n confirm (task rows); q/ctrl+c quit in nav mode; esc cancels input. Text/priority input is hand-rolled rune append + backspace (only bubbletea is vendored; no bubbles/textinput — boring wins).
- Writes go through a small steeringAPI interface (answerEscalation / setPriority / cancelTask) implemented over the daemon HTTP API: POST /v0/escalations/answer, PATCH /v0/tasks/{id} {priority}, PATCH {status:cancelled}, with X-Tuhdoo-Actor set. Tests use a fake steeringAPI; no new write paths (constraint honored).
- "Archive" is status:cancelled — the API has open/done/cancelled only; no new status invented.
- Acting human principal: D7 says git-derived from user.email but never specifies the compression. Decision: `tuhdoo top [--as <human>]`; default = local-part of `git config user.email`; must be a root human (no "/"), validated by daemon.ValidateActor. Flagging this derivation choice in the finish summary for Brandon's review since the ledger so far uses bare "brandon".
- Tests: table-driven pure render/row tests (watch_test.go pattern), Update interaction tests against the fake API, plus an integration test (cli_test.go harness) driving the real model against a spawned daemon: answer blocking escalation -> task claimable again; priority patch visible in backlog; cancel event actor verified by reading the task.updated event off refs/heads/tuhdoo.

Nothing implemented yet; next step is cmd/tuhdoo/top.go.

### 2026-07-30 05:50 UTC — run by `brandon/impl-2` — done

- Branch: `main`
- Commits: `fa8c7d3`

tuhdoo top shipped: the watch skeleton plus input handling (cmd/tuhdoo/top.go), wired into main.go. Cursor (j/k, arrows) moves over open escalations and open tasks; `a` answers an escalation, `p` reprioritizes, `c` cancels with y/n confirm; esc backs out; q/ctrl+c quits (q types text while in an input prompt). All writes go through the daemon HTTP API's existing admin verbs (POST /v0/escalations/answer, PATCH /v0/tasks/{id}) via a small steeringAPI interface — no git, no ops-layer access, MCP surface untouched at ten verbs.

Acceptance verified by tests (make test lint green, commit fa8c7d3 pushed): unit tests cover pure row-building/rendering and every interaction flow against a fake API; an integration test drives the real model against a spawned daemon — answering a blocking escalation from the TUI makes the task claimable again, a reprioritize is served first by the next claim (same ReadyTasks pool get_backlog reads), and the cancel event lands on the data branch stamped "actor":"brandon".

Two judgment calls to review: (1) acting principal — D7 says git-derived from user.email but not how; I made `top` default to the email's local part, overridable with --as <human>, and rejected agent-style principals (slash) since steering is human work. Note your git email here is a GitHub noreply address, so the derived default on this machine would be "4099114+brandonbews" — you'll likely want `tuhdoo top --as brandon` (or we later add a config knob). (2) "archive" = status:cancelled — the API has open/done/cancelled only; I didn't invent a new status.
