# Tree/parent-grouped rendering in the TUI list

`t-01KYT63MB28Z535SMJCBC7SY1P`

- **Status:** cancelled
- **Priority:** none
- **Labels:** `cli` `tui` `design`
- **Created:** 2026-07-30 18:55 UTC by `4099114+brandonbews`

## Description

Cancelled 2026-08-05 (edge grill, at Brandon's direction): the parents field is being removed entirely (see the edge-grill revision to 001 D5) — epics are now depends_on containers, so a parent-grouped tree view has no relation left to render. The successor exploration is the inbox capture "Epics after parents removal: is any epic UX worth having?" (tuh-01KZ9Y3THHH5B8GT22T92BPEZ8).

Original description follows for the record:

Gated: unpark when the edge-semantics grill resolves what parent edges mean vs depends_on (docs/design/open-questions.md — "Edge semantics, cycle enforcement, and what a milestone is").

Status of that gate as of 2026-08-03: sub-question (3), what a milestone is, is settled — a milestone is a label, not a mechanism, and its done-ness is declared, never computed. Sub-questions (1) semantics (are parents containment and depends_on scheduling? should cross-relation cycles be legal?) and (2) enforcement honesty remain open, and (1) is the load-bearing one here.

Follow-up from Cycle 4. Do NOT build until (1) resolves: a tree view would bake an interpretation of parent-vs-depends_on into the primary steering surface before that interpretation has been decided. Until then the list stays flat with edge-marker suffixes.

When unblocked: decide how parent-grouping coexists with the status-first sections (children span buckets — a parent's children can sit in ready, blocked, and done at once), then implement with tests.

## History

### 2026-07-31 15:49 UTC — edit by `brandon`

description edited · status open→held

### 2026-08-01 05:42 UTC — edit by `brandon/claude-code-1`

depends_on −t-d83w

### 2026-08-03 21:07 UTC — edit by `brandon`

description edited

### 2026-08-05 21:43 UTC — edit by `brandon/claude-code-1`

description edited · status held→cancelled
