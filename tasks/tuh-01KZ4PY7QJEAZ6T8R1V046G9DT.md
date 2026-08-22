# Task history: render task.updated field edits — every edit, compact old→new lines

`tuh-01KZ4PY7QJEAZ6T8R1V046G9DT`

- **Status:** done
- **Priority:** none
- **Labels:** `go` `tui` `ledger`
- **Created:** 2026-08-03 21:02 UTC by `brandon`

## Description

Context: Brandon's capture 2026-08-03 — after editing the v1 milestone description, the task's History still said "no activity yet"; every field edit (title, description, priority, status, labels, edges) is invisible on the task it happened to. Grilled with Brandon 2026-08-11; decisions below are his, do not re-litigate.

Mechanism (verified at the grill): `task.updated` events carry exactly the changed fields with new values (`event.TaskUpdated`, pointer fields, nil = unchanged). Replayed State applies them but keeps no record; both history surfaces — `historyOf` (`cmd/tuhdoo/commands.go:402`, used by the TUI task view) and the generated task page's History section (`internal/views/views.go:372`) — merge notes, runs, and escalations only. The data is all on the ledger; only the rendering pipeline drops it.

Decisions (2026-08-11 grill):
- **Every edit gets a line — no filtering.** Rationale: the settled priority hierarchy (accuracy at any scale > noise protection, "never hidden"); a curated subset would recreate the invisible-edit bug for whichever fields get deemed noise.
- **Entry shape: actor + stamp + compact per-field summary.** Scalar fields show old→new (`priority 0→2`, `status open→held`); list fields show deltas (`labels +launch −web`, `depends_on +tuh-xxxx`); text fields show name-only (`description edited`, `retitled`) — no text diffs, the current text is visible in the same view. One event touching several fields = one entry listing them.
- **Both surfaces:** the TUI task view History and the generated `tasks/<id>.md` — one mechanism in core, two renderers. The view rendering change takes the T6 view-format bump (cosmetic-only, highest-wins).
- **Retroactive by construction:** history is replay-derived, so all past edits on the existing ledger appear after upgrade — no event changes, no migration.

Implementation shape (pointer, not prescription): the deterministic core records an update entry per `task.updated` at apply time — old values read from state before the field-level apply — kept per task in State; pure function, table-driven tests (T1). Daemon hydration exposes the records; `historyOf` and `views` render them into the existing ULID-ordered history stream. State/views grow with every edit forever — accepted; unbounded-growth concerns broadly belong to the working-set retirement grill (tuh-01KZA0VT234XJYVZWT980V7K2Y), not here.

Acceptance:
- Editing any field (from the TUI or `update_task`) produces a history entry on both surfaces with the decided shape; a multi-field edit renders one entry.
- Existing past edits render retroactively (core tests: old→new capture, list deltas, multi-field events, text-field name-only).
- No new event types or fields (T3 untouched); stored bytes never rewritten; T5 surface untouched; T6 version bumped with the rendering change.
- `make test lint` green; one PR.

## History

### 2026-08-11 22:58 UTC — edit by `brandon/claude-code-4`

retitled · description edited · status inbox→open · labels +go

### 2026-08-12 00:04 UTC — run by `brandon/claude-code-4` — done

- Branch: `tuh-g9dt/task-history-edits`
- PR: <https://github.com/brandonbews/tuhdoo/pull/68>
- Merged as: `25329eef179413cd65f1f3623f86a4013851c747`

task.updated edits now render in task history on both surfaces, per the 2026-08-11 grill decisions. Core: State.Updates records one entry per task.updated inside apply, old values read before the field writes — scalars old→new, list membership deltas (depends_on via short IDs), text fields name-only, one entry per multi-field event; table-driven tests incl. chaining. Daemon hydration exposes updates alongside notes/runs/escalations (no MCP verb changes); historyOf renders into TUI + one-shot task view; internal/views renders into generated task pages with FormatVersion 7→8 and regenerated goldens showing retroactive rendering of pre-existing edits. Judgment calls flagged in the PR body (glyphs, name-only for membership-preserving list replacements, honest same-value renders). make test lint green; merged via PR #68, squash commit 25329ee.
