# t-01KYVD31CNTR1EVCDHPGZFQ5EV — Rename the cancel interaction: archive as the human verb, task.cancelled stays the plumbing

- Status: open — ready
- Priority: 1
- Labels: `cli`, `tui`, `ux`, `design`
- Created: 2026-07-31 06:16 UTC by `brandon/claude-code-2`

## Description

Context: dogfood steering feedback (Brandon, 2026-07-30): in the TUI, `c cancel` read first as cancelling a UI action, then as deleting the task — it took real use to learn it means "we are not doing this task". Nothing is ever deleted: task.cancelled is an immutable ledger event and full history stays on the data branch. But the word collides with esc-cancels-input on the same screen and under-communicates that history is preserved. Direction (recommended by the 2026-07-30 session, subject to Brandon's confirmation on the word before implementing): follow the T7 plumbing/porcelain pattern — the event vocabulary (task.cancelled; status value `cancelled` in events and API JSON) is plumbing and never changes (T3), while the human-facing verb becomes **archive** everywhere humans read or type.

The ask:
1. TUI: footer `c archive`; confirm copy `archive t-xxxx (title)? y/n — history stays on the ledger`; status message `archived t-xxxx`; cancelled tasks render as `archived` wherever a human-facing status appears (e.g. edge annotations: `(archived — title)`).
2. One-shot output: same rename in human-readable renderings (status sections, task biography status line) — this changes exact-format CLI tests; update them deliberately and say so in the run summary. API/JSON field values stay `cancelled` (machine surface), and the MCP update_task status vocabulary is unchanged.
3. Docs: dated note in 002 T7 recording the porcelain↔plumbing word mapping (archive ↔ task.cancelled).

Acceptance: no human-facing `cancel`/`cancelled` remains for the task-status meaning in TUI or one-shot renderings (esc cancelling input entry keeps its word — that is the collision being resolved); tests updated; make test lint green.

Pointers: cmd/tuhdoo/top.go (modeConfirmCancel, footer, submit), cmd/tuhdoo/commands.go and render.go status renderings, cmd/tuhdoo/snapshot.go taskRef; docs/design/002-technology.md T7.

Constraints: no event, API, or MCP surface changes; stored bytes untouched; boring Go.

## History

_No activity yet._
