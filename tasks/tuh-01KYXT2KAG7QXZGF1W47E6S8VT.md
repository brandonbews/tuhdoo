# Task view field focus ring: up/down selects any editable field, enter opens its editor

`tuh-01KYXT2KAG7QXZGF1W47E6S8VT`

- **Status:** open — ready
- **Priority:** 1
- **Labels:** `cli` `tui` `ux` `design`
- **Created:** 2026-08-01 04:42 UTC by `brandon`

## Description

Context: The task view (modeDetail in cmd/tuhdoo/top.go) focuses only open escalations; e/E edit title/description but E proved undiscoverable (dogfood capture 2026-08-01: "the description doesn't appear editable"). Grilled 2026-08-01; this description is the settled design. (Vocabulary reconciled 2026-08-01 with tuh-01KZ0ES83SFH6MKWP82Y2HNTPK: the archive verb became c cancel.)

The ask: Generalize the task-view focus to a ring over every actionable field, in render order: title line -> priority meta line -> each open escalation -> description body (including the dim "none" placeholder when empty). Nothing else is focusable — bars and read-only meta lines are never stops (labels editing is tuh-01KYXVSRVK2GFW439G1T0GBQKM, separately). j/k/up/down move focus when a further stop exists in that direction (reveal-scroll just enough), else scroll one line — the existing escalation rule generalized. Enter opens the stop's editor: title -> single-line prefilled, priority -> numeric input, escalation -> answer entry, description -> multi-line prefilled; unchanged submit writes nothing (existing editWas rule). The focused stop renders with the dashboard selection treatment (selectedText gutter bar + tint) over its full block. Retire e/E; keep p and c (cancel — renamed from "a archive" by the vocabulary task); update both footer legends. Plain opens focus the title at top-of-view; Needs Input-routed opens keep today's escalation preselect. Watch mode keeps no ring (pure scroll, read-only). Mouse: click selects the stop under the pointer, click on the already-selected stop acts as enter, wheel always scrolls — extend the detailEscAt hit-map approach to all stops.

Acceptance: Interaction tests cover ring traversal order and reveal-scroll; enter opening the correct prefilled mode per stop; empty description focusable and editable; e/E unbound; routed open preselects the escalation; watch mode unchanged; click-to-select/click-to-open on field stops. Golden tests updated for focused-field rendering. make test lint green.

Pointers: cmd/tuhdoo/top.go (updateDetail, detailLines, detailFocusIdx, detailRevealScroll, detailEscAt), textinput.go, selection.go.

Constraints: No new daemon API — existing steeringAPI writes only. Boring Go; the ring is data (a slice of stops built in the same pass as detailLines), not cleverness.

## History

_No activity yet._
