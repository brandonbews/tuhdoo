# Dashboard list hides most row metadata past page width — resurface labels, dep counts, priority

`tuh-01KZ9HDMYDGCM0HKMV3FZ00XJX`

- **Status:** open — ready
- **Priority:** 0
- **Labels:** `tui`
- **Created:** 2026-08-05 18:01 UTC by `brandon/claude-code-1`

## Description

Grilled 2026-08-05 (dashboard-metadata grill) — this description is the settled design; build it as written.

Context: the dashboard list hides row metadata not by overflow but by sacrifice — fitTitle (cmd/tuhdoo/top.go ~1467) drops or ellipsizes the labels/edges suffix whenever the title is long, and this repo's titles usually are, so labels and dep counts are effectively invisible while scanning. Five layouts were mocked up and compared by eye (baseline, two-line, right-aligned gutter, fixed columns, expand-on-cursor); two-line won: it is the only option where neither titles nor metadata lose.

The ask: task rows in the TUI list become two-line — full title plus a dim metadata second line — and the title column aligns across mixed ID prefixes.

Settled design:
- Line 1: id / priority badge / bold title, plain-ellipsized to the width — no suffix, no fitTitle fight. The priority badge stays on line 1 unchanged.
- Line 2: dim metadata indented to the title column, rendered ONLY when non-empty — a one-line row signals "no labels, no edges" at a glance. Skip-when-empty is deliberate; no placeholder (the task-view labels line needs its `none` for focusability; list rows are not editors).
- One meta-line rule across every section: `[labels] · edges · <mode tail>` joined with " · ". Edges keep the existing edgeText content ("in PARENT +n", "N deps"). Mode tails: in-progress = "← holder" in yellow on the otherwise dim line; done/cancelled (history mode) = close stamp · closing actor (moves off line 1); ready/held/inbox have none.
- Blocked rows keep their separate dim-red `waiting:` line below the meta line — a reason, not metadata.
- Escalation rows stay three lines: their existing dim meta line (actor · stamp) becomes `[labels] · edges · actor · stamp` — the same rule with actor·stamp as the mode tail; the metadata sits on line 3 because the question outranks it.
- Gutter alignment (amendment, 2026-08-05): ShortID keeps the ID prefix plus last-4 (internal/event/id.go), so migrated `t-` IDs render 6 wide and minted `tuh-` IDs 8 — but gridIDW is a 6-cell const and padTo never truncates, so `tuh-` rows shove their badge and title 2 cells right. Fix: derive the ID column width per render pass as the widest visible short ID (floor 6); gridTitleCol becomes derived, not const, and the meta line, `waiting:` line, and escalation second/third lines all indent to the derived column — titles and content stay on one column regardless of ID prefix mix.
- No density toggle — revisit only if a grown backlog makes the two-line list painful in practice.
- gridRow drops its suffix/suffixStyle params; fitTitle likely goes dead — delete it if so. labelSuffix/closeSuffix reshape or fold into the meta-line builder as fits.

Acceptance:
- top_test / golden coverage: a ready row with labels+edges renders two lines; a bare row stays one line; the in-progress holder tail is yellow on a dim line; a blocked row stacks title / meta / waiting; history rows carry close stamp · actor on the meta line; an escalation row is three lines with the extended meta line; selection bar covers all of a row's lines; a list mixing `t-` and `tuh-` rows renders every title, meta line, and waiting line at the same derived column. Goldens updated (top_golden_test; oneshot goldens untouched).
- make test lint green; land as one PR per the repo's git shape.

Constraints: TUI list rendering only — the detail view, CLI one-shots (T7 serialization), and internal/views are untouched.

## History

_No activity yet._
