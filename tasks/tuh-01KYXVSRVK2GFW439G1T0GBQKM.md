# Labels editable from the task view

`tuh-01KYXVSRVK2GFW439G1T0GBQKM`

- **Status:** done
- **Priority:** 0
- **Labels:** `tui` `design`
- **Created:** 2026-08-01 05:12 UTC by `brandon`

## Description

Grilled 2026-08-05 (labels grill) — this description is the settled design; build it as written.

Context: labels are agnostic decoration to the platform — no label value is weight-bearing (001 D5, label-agnosticism revision 2026-08-05). The TUI is the only surface that cannot edit them (CLI `update --labels` and MCP update_task already write full-replacement lists over PATCH /v0/tasks/{id}); that is a steering gap in the primary steering surface — e.g. tagging a milestone from the TUI means dropping to the CLI.

The ask: make labels editable from the task detail view, as a focus-ring stop.

Settled design:
- The labels meta line always renders, dim `none` placeholder when empty (the description-body pattern, cmd/tuhdoo/top.go detailLines ~975), in watch and armed modes and on terminal tasks alike — one uniform rule replaces render-only-when-non-empty.
- Labels joins the focus ring in render order: title → priority → labels → escalations → description. Terminal tasks lose the labels stop, same rule as priority (closed records are browsed, not steered). No dedicated key binding — ring only.
- enter opens the shared single-line textInput prefilled with the current list comma-joined ("tui, design").
- Submit parses with splitList exactly (split on commas, trim, drop empties) — reuse the parser, not a copy; no dedup, no case-folding; store what was typed. A comma inside a label is unrepresentable on every surface — accepted constraint.
- Empty submit clears all labels (the CLI's explicit-empty --labels precedent). "Unchanged" is semantic and order-sensitive: parse, compare the resulting slice element-wise to the task's current labels, write nothing when equal (respacing is a no-op; reordering is a real edit). The raw-string editWas guard still short-circuits first.
- Write path: steeringAPI gains a setLabels method over the existing PATCH (cmd/tuhdoo/top.go httpSteering); no daemon/ops/CLI/MCP changes.

Acceptance:
- top_test (and goldens as needed) cover: ring order including the labels stop; stop absent on terminal tasks; always-rendered labels line with dim none; editor prefill; empty submit clears; a respaced submit writes nothing; a real edit writes the parsed list.
- The stale comment at top.go ~625-627 ("labels editing is a separate task") is rewritten to describe the shipped ring.
- make test lint green; land as one PR per the repo's git shape.

Constraints: TUI-only change; platform label-agnosticism untouched (no ops-side normalization).

## History

### 2026-08-05 20:12 UTC — run by `brandon/claude-code-1` — done

- Branch: `tuh-bqkm/labels-editable-task-view`
- PR: <https://github.com/brandonbews/tuhdoo/pull/35>
- Commits: `d02ebc8`

Landed as PR #35 (squash-merged to main, checks green). Labels are now editable from the TUI task detail view exactly per the settled design: labels stop in the focus ring after priority (absent on terminal tasks), always-rendered meta line with dim none placeholder, shared single-line editor prefilled comma-joined, splitList reused for parsing, empty submit clears, element-wise unchanged comparison (respace no-op, reorder real edit), steeringAPI.setLabels over the existing PATCH. Goldens updated; stale top.go comment rewritten. No daemon/ops/CLI/MCP changes.
