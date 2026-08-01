# Labels editable from the task view

`tuh-01KYXVSRVK2GFW439G1T0GBQKM`

- **Status:** on hold — deliberately paused
- **Priority:** 0
- **Labels:** `tui` `design`
- **Created:** 2026-08-01 05:12 UTC by `brandon`

## Description

Gated: design-shaped — unpark for its own grill cycle before any scoping or code. Held at triage 2026-08-01; this description records the recon, not a settled design.

Original capture: labels should be editable from the task view; needs a design for the edit medium (list-valued field) and the write path.

Recon facts (2026-08-01, so the grill starts from truth):
- The write path already exists end-to-end: PATCH /v0/tasks/{id} accepts labels as a full-replacement *[]string; CLI `tuhdoo update --labels a,b` and MCP update_task already use it. Only a steeringAPI setLabels method is missing — a one-liner over the existing PATCH (cmd/tuhdoo/top.go httpSteering). The capture's "needs a labels write path in daemon API" premise is stale.
- CLI precedent for the medium: comma-separated via splitList (trim, drop empties); an explicitly-set empty --labels clears the list.
- The detail view renders the labels meta line only when non-empty (top.go detailLines) — a task with no labels currently has nothing to focus/edit.

Questions reserved for the grill:
- Interaction model: does labels become a stop in the task-view focus ring (tuh-01KYXT2KAG7QXZGF1W47E6S8VT — its settled design currently says meta lines are never stops and names this task as the separate home for labels), or a standalone binding? Ring membership implies depends_on the ring task and always rendering the labels line with a dim "none" placeholder (description's pattern) so empty is reachable.
- Edit medium: single-line comma-separated text (CLI-consistent, reuses textInput) vs anything fancier.
- Clear semantics: empty submit clears all labels vs no-op, and the interplay with the editWas unchanged-submit-writes-nothing rule.
- Normalization: dedup/trim/case at parse time, and whether that lives TUI-side only (matching splitList) or in ops.

## History

_No activity yet._
