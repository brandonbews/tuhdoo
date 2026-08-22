# Task-view edges: one-line dep rows, a needed-by section, edge navigation with a back stack

`tuh-01KZ9Y3THHH5B8GT22T92BPEZ8`

- **Status:** done
- **Priority:** none
- **Labels:** `tui` `ux`
- **Created:** 2026-08-05 21:43 UTC by `brandon/claude-code-1`

## Description

Context: the 2026-08-05 edge-grill capture asked whether any epic UX earns its keep after the parents removal. Grilled with Brandon 2026-08-11; the epic framing dissolved and a concrete edge-UX rework came out of it. Decisions below are his — do not re-litigate.

Settled at the grill:
- **No epic-specific UX** — no container marker, no children rollup. Defining "containerhood" over edge shapes is tree-imposing that D5's flat model deliberately avoids; anything of the sort needs a D5 revision first.
- **Sweep half: verified clean, nothing to do.** No parent/epic code traces remain (only git commit-tree parents and doc-strings describing the new model); old stored events carrying a `parents` array are covered by `TestRetiredParentsFieldTolerated` (internal/core/replay_test.go:655).

The build (task view, `cmd/tuhdoo/top.go`):
1. **Depends-on list de-blobbed.** Today `joinRefs` (commands.go:380) comma-joins annotated refs into one wrapping paragraph — "kind of ridiculous" with multiple deps (Brandon). Replace with one row per edge: **bold short-id · dim status word · plain title**, hard-truncated to one line with an ellipsis, never wrapped. (Chosen against id+title-only because a container's dep list read top-to-bottom is its progress checklist — status is the glanceable signal.)
2. **New NEEDED BY section**: reverse edges, computed at render from the snapshot (reverse index over tasks' `DependsOn`), same row shape, all dependents shown regardless of status (accuracy over noise — the dim status word carries the story), ULID order.
3. **Edge rows are selectable stops** in the existing focus machinery; enter opens the target task's view. If click-to-open falls out cheaply from the existing mouse support, take it; don't build machinery for it.
4. **Back stack navigation**: hopping along edges pushes the current task; esc pops to the previous task view, and from the first task back to the dashboard. Plain slice of detail IDs — boring Go, no channels.
5. **One-shot CLI consistency** (`tuhdoo` task output, commands.go:355): same one-line-per-edge format for depends-on (and a needed-by block), minus selection.

Acceptance:
- A task with several deps renders them as aligned single lines, statuses visible; a task with dependents shows the NEEDED BY section; a task with neither shows neither.
- Enter on an edge row opens that task's view; esc walks back through visited tasks to the dashboard; the selection bar covers edge rows correctly.
- TUI rendering/navigation tests updated or added where output and key handling are asserted; `make test lint` green; one PR.

Constraints: TUI + one-shot CLI only — no schema or verb changes (T5), no event changes (T3). Generated views (`internal/views`) are out of scope: their edge rendering is unchanged and no T6 bump happens here; if a needed-by section on generated task pages seems wanted later, capture it separately.

Pointers: `cmd/tuhdoo/top.go` — the depends-on field line (~1020), focus-stop machinery (`detailStops`, `detailFocusIdx`, `openStop`), `openRow` (~404) and the detail esc cases (~561, ~598) for the mode/back-stack shape; `cmd/tuhdoo/snapshot.go` `taskRef` for the annotation source; `edgeText` (~1627) dashboard dep counts are unchanged.

## History

### 2026-08-06 22:41 UTC — note from `brandon/claude-code-1`

Partial progress on the "sweep for leftover parent/epic traces" item, 2026-08-06 triage session: docs/agent-protocol.md's two stale parents references (the Decomposition section's "parent edges pointing at the task you hold" and update_task's list-field note) were fixed directly in PR #44 — decomposition now reads "create children in one batch, then point the held task at them with depends_on". The eventual sweep still owes the rest of the codebase/docs; internal/event/catalog.go's "stored events may still carry parents (retired)" comment is deliberate read-side legacy handling, not drift — leave it.

### 2026-08-11 23:19 UTC — edit by `brandon/claude-code-4`

retitled · description edited · status inbox→open · labels +ux −design

### 2026-08-12 00:24 UTC — run by `brandon/claude-code-4` — done

- Branch: `tuh-pez8/task-view-edges`
- PR: <https://github.com/brandonbews/tuhdoo/pull/69>
- Merged as: `44fac367a5345f51209d430c7657f4df92ecb69d`

Task view edges reworked per the 2026-08-11 grill: the comma-joined depends-on blob is gone; DEPENDS ON and NEEDED BY render as bar sections with one aligned row per edge (bold short-id, dim status word, plain title, hard-ellipsized, never wrapped). Needed-by computed at render from a plain reverse loop over the snapshot, ULID order, all statuses shown. Edge rows are focus stops: enter (and mouse click, which fell out of existing machinery) opens the target task; esc pops a plain back-stack slice, first task pops to the dashboard with cursor intact. One-shot tuhdoo task prints the same shape with full IDs plus a needed-by block. No epic UX built. Scope held to cmd/tuhdoo/; internal/views untouched, no T6 bump. Judgment calls in the PR body (section order, watch-mode rows unselectable, stored status word, pop resets scroll). make test lint green; merged via PR #69, squash commit 44fac36.
