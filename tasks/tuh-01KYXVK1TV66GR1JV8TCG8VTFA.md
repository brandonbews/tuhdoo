# Task-view history: blank line between entries, bold entry descriptors

`tuh-01KYXVK1TV66GR1JV8TCG8VTFA`

- **Status:** done
- **Priority:** 0
- **Labels:** `cli` `tui` `ux`
- **Created:** 2026-08-01 05:08 UTC by `brandon`

## Description

Context: History entries (notes, runs, answered escalations) render via the shared historyOf (cmd/tuhdoo/commands.go:348), used by both the TUI task view (top.go detailLines) and the one-shot `tuhdoo task <id>`. Entries currently stack with no separation, and each entry's header line is a dim timestamp plus a plain descriptor. Dogfood capture by Brandon 2026-08-01; interpretation confirmed with him 2026-08-01 (original title had a typo: "shield" = "should").

The ask, two changes in historyOf so both surfaces stay identical:
1. One blank line between consecutive history entries (not before the first or after the last — the section framing already handles the edges).
2. Bold the descriptor on each entry's header line — "note by <actor>", "run by <actor> — <outcome>", "escalation from <actor>" — while the timestamp keeps its dim treatment (the dim-stamp convention holds everywhere else). The run outcome is already bold; folding it into a bold descriptor is fine.

Decided at triage: both surfaces change together (one rendering to maintain); descriptor-only bold, stamp stays dim.

Acceptance: TUI golden/interaction tests and any `tuhdoo task` output tests updated to show the blank separator and bold descriptors on all three entry kinds; empty-history and single-entry cases produce no stray blank lines; make test lint green.

Pointers: cmd/tuhdoo/commands.go historyOf, cmd/tuhdoo/top.go detailLines (note it TrimRights each entry's text — the separator must survive that, e.g. emit it between entries rather than trailing), printTask.

Constraints: rendering only — no daemon/API/event changes; boring Go.

## History

### 2026-08-03 06:11 UTC — run by `brandon/claude-code-1` — done

- Branch: `tuh-vtfa/history-entry-formatting`
- PR: <https://github.com/brandonbews/tuhdoo/pull/22>
- Commits: `8a604f6`

Landed via PR #22 (squash-merged to main 2026-08-03). Both changes in the shared historyOf so the TUI task view and one-shot tuhdoo task render identically: one blank line between consecutive history entries (leading newline on entries 2..n — survives the TUI's per-entry TrimRight; section framing owns the edges) and bold descriptors on all entry headers (note by / run by — outcome / escalation from) with stamps staying dim. Unanswered escalations on terminal tasks (PR #20 routing) share the escalation header and are covered. Golden + one-shot + seeded CLI tests updated; empty-history and single-entry goldens unchanged byte-for-byte, pinning the no-stray-blanks edge cases. make test lint green.
