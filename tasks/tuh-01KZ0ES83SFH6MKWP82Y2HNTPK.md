# One status vocabulary: stored words are the displayed words

`tuh-01KZ0ES83SFH6MKWP82Y2HNTPK`

- **Status:** inbox — untriaged capture
- **Priority:** 0
- **Labels:** `cleanup` `vocabulary`
- **Created:** 2026-08-02 05:22 UTC by `brandon/claude-code-1`

## Description

## Context

A datashape exploration (2026-08-01, session with Brandon) found dual vocabulary on the intent axis: the stored status `cancelled` renders as "archived" and `held` as "on hold" / "on-hold", with the mapping duplicated in two places (`humanStatus` in internal/views/views.go:552 and cmd/tuhdoo/render.go:85) plus a reverse input alias (`archived`->`cancelled`) in cmd/tuhdoo/write_cmds.go:241. "Archived" also collides semantically with `done` — both are terminal and kept forever, yet backlog.md renders "Done" and "Archived" as sibling sections.

Blessed rule (Brandon, 2026-08-01): display words = stored words (`open`, `inbox`, `held`, `done`, `cancelled`); zero synonym mappings anywhere. The one sanctioned non-status name that stays: NEEDS INPUT as the human name for the open-escalations section (settled 2026-07-31 grill cycle). Brandon's blessed-state write-up may land before this is claimed — if it exists, it is the source of truth over this description.

## The ask

- Delete both `humanStatus` copies and the `archived`->`cancelled` input alias in write_cmds.go.
- Update every emitting surface to the stored words:
  - views markdown: section names, README count-table headers, task-page `statusLine`, dep-link annotations (`depLinks`), backlog section headers/prose. Bump `FormatVersion` to 6 with a changelog line, per the existing pattern in views.go.
  - TUI: section bars (ON HOLD -> HELD, and the archive verb in key hints — `a archive` becomes cancel wording; keeping the `a` key is fine, the word changes).
  - CLI `tuhdoo backlog` STATE column tokens: `on-hold` -> `held`, `archived` -> `cancelled`.
- Update docs/agent-protocol.md (and any other docs) where the human words appear as vocabulary.

## Acceptance

- `grep -ri 'archived\|on.hold' internal/ cmd/` finds no emitted strings (comments recording the history are fine).
- Views golden/table tests updated to the new bytes; a test asserts FormatVersion is 6.
- No surface accepts or emits a synonym for a stored status word.
- `make test lint` green from repo root.

## Constraints

- Stored bytes and API/ledger vocabulary are already the target words — this task touches display surfaces and input aliases only; no schema or event changes.
- Do NOT fold in classifier unification (separate task) or any design-shaped changes (TUI blocked-section membership, finish_run semantics — those are grill-cycle material, not cleanup).

## History

_No activity yet._
