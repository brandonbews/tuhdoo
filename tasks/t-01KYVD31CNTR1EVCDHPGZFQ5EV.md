# Rename the cancel interaction: archive as the human verb, task.cancelled stays the plumbing

`t-01KYVD31CNTR1EVCDHPGZFQ5EV`

- **Status:** done
- **Priority:** 1
- **Labels:** `cli` `tui` `ux` `design`
- **Depends on:** [`t-pvg4`](t-01KYVJ2607S5S390CVYSF3PVG4.md) (done)
- **Created:** 2026-07-31 06:16 UTC by `brandon/claude-code-2`

## Description

Context: dogfood steering feedback (Brandon, 2026-07-30): in the TUI, `c cancel` read first as cancelling a UI action, then as deleting the task — it took real use to learn it means "we are not doing this task". Nothing is ever deleted: task.cancelled is an immutable ledger event and full history stays on the data branch. But the word collides with esc-cancels-input on the same screen and under-communicates that history is preserved. Decision CONFIRMED by Brandon (2026-07-30): follow the T7 plumbing/porcelain pattern — the event vocabulary (task.cancelled; status value `cancelled` in events and API JSON) is plumbing and never changes (T3), while the human-facing verb becomes **archive** everywhere humans read or type.

The ask:
1. TUI: footer `c archive`; confirm copy `archive tuh-xxxx (title)? y/n — history stays on the ledger`; status message `archived tuh-xxxx`; cancelled tasks render as `archived` wherever a human-facing status appears (e.g. edge annotations: `(archived — title)`).
2. One-shot output: same rename in human-readable renderings (status sections, task biography status line) — this changes exact-format CLI tests; update them deliberately and say so in the run summary. API/JSON field values stay `cancelled` (machine surface), and the MCP update_task status vocabulary is unchanged.
3. Docs: dated note in 002 T7 recording the porcelain↔plumbing word mapping (archive ↔ task.cancelled).

Acceptance: no human-facing `cancel`/`cancelled` remains for the task-status meaning in TUI or one-shot renderings (esc cancelling input entry keeps its word — that is the collision being resolved); tests updated; make test lint green.

Pointers: cmd/tuhdoo/top.go (modeConfirmCancel, footer, submit), cmd/tuhdoo/commands.go and render.go status renderings, cmd/tuhdoo/snapshot.go taskRef; docs/design/002-technology.md T7.

Constraints: no event, API, or MCP surface changes; stored bytes untouched; boring Go.

## History

### 2026-07-31 06:37 UTC — edit by `brandon/claude-code-3`

description edited

### 2026-07-31 07:54 UTC — edit by `brandon/claude-code-10`

depends_on +t-pvg4

### 2026-07-31 09:11 UTC — run by `brandon/claude-code-1` — done

- Branch: `main`
- Commits: `042b2c0`

Archive is now the human verb everywhere; task.cancelled stays the plumbing. TUI: footer/READY-bar `c archive`, confirm copy "archive t-xxxx (title)? y/n — history stays on the ledger", status message `archived t-xxxx`; modeConfirmCancel→modeConfirmArchive, cancelTask→archiveTask (PATCH body still sends status:"cancelled"). One-shot: new humanStatus() helper in render.go maps cancelled→archived in status/backlog tallies, task-biography status line, and edge annotations `(archived — title)`; JSON/API values pinned as `cancelled` by a new test assertion in write_cli_test.go. `tuhdoo update --status` documents `archived` (mapped to `cancelled` before PATCH; raw `cancelled` still passes for scripts). Dated T7 note in docs/design/002-technology.md records the archive ↔ task.cancelled mapping; Cycle 4 accepted-consequences line amended to match. Exact-format tests deliberately updated: top_golden_test.go (READY bar + footer goldens — footer tally lost its trailing margin space because "archive" is one rune longer and the 80-col drop rule would otherwise discard the done tally), top_test.go (TestTopCancelFlow→TestTopArchiveFlow, taskRef annotation), cli_test.go (Archived tallies), write_cli_test.go (exercises --status archived, pins JSON "cancelled"). esc-cancels-input deliberately keeps the word "cancel" — that collision was the point. make test lint green; single commit 042b2c0 on main, pushed. Deploy note: daemon NOT restarted yet — binary rebuild+restart batched to end of the overnight run.
