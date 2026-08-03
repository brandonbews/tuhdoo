# Tree/parent-grouped rendering in the TUI list

`t-01KYT63MB28Z535SMJCBC7SY1P`

- **Status:** on hold — deliberately paused
- **Priority:** 0
- **Labels:** `cli` `tui` `design`
- **Created:** 2026-07-30 18:55 UTC by `4099114+brandonbews`

## Description

Gated: unpark when the edge-semantics grill resolves what parent edges mean vs depends_on (docs/design/open-questions.md — "Edge semantics, cycle enforcement, and what a milestone is").

Status of that gate as of 2026-08-03: sub-question (3), what a milestone is, is settled — a milestone is a label, not a mechanism, and its done-ness is declared, never computed. Sub-questions (1) semantics (are parents containment and depends_on scheduling? should cross-relation cycles be legal?) and (2) enforcement honesty remain open, and (1) is the load-bearing one here.

Follow-up from Cycle 4. Do NOT build until (1) resolves: a tree view would bake an interpretation of parent-vs-depends_on into the primary steering surface before that interpretation has been decided. Until then the list stays flat with edge-marker suffixes.

When unblocked: decide how parent-grouping coexists with the status-first sections (children span buckets — a parent's children can sit in ready, blocked, and done at once), then implement with tests.

## History

_No activity yet._
