# Task-view escalations: structured question always visible, context collapsed by default

`tuh-01KZPYFK8GFXQ5T2GFPCRQ8E86`

- **Status:** done
- **Priority:** 0
- **Labels:** `tui` `ux`
- **Created:** 2026-08-10 23:00 UTC by `brandon`

## Description

Context: Brandon's capture 2026-08-10 ("escalations are kind of hard to read and navigate in the current ux"), grilled with him 2026-08-11. Confirmed pains, in the task view's NEEDS INPUT section (`cmd/tuhdoo/top.go`, `escalationRow` ~line 1087): (1) the context wall — the full context block renders dim and untruncated, so long escalations bury the screen; (2) the flattened question — `oneLine(e.Question)` collapses any structure the agent wrote into one bold line; (3) the buried ask — options and recommendation are invisible inside prose. Explicitly NOT a pain: the inline answer flow (enter → answer input) — leave it unchanged.

The decided design (do not re-litigate): convention + collapse, no schema change.
- Render the question field faithfully: preserve its line structure, bold, wrapped — stop one-lining it.
- Collapse the context block by default to a stub — first line plus a count, e.g. `(+12 lines — e to expand)` — with a key that toggles expansion per escalation while the view is open. Pick a key that doesn't collide with existing task-view bindings, and update the footer hint. If click-to-toggle falls out cheaply given existing mouse support, take it; don't build machinery for it.
- Blocking badge (`!`, red) and the actor/date meta line unchanged.
- Old-style escalations with fat contexts degrade gracefully into a collapsed stub — no migration, no special cases.
- The convention half — question field carries the whole decision package (question, options, recommendation, short); context field is background only — lands via the protocol slim-down task (tuh-01KZSBC7K0GNYNYTTAM6DW7Q0M). No dependency edge either way: this rendering works regardless of when agents adopt the convention.

Acceptance:
- A multi-line question renders with its structure in the task view; a long context shows collapsed with an accurate line count; the toggle expands and re-collapses it; selection/focus behavior over the escalation block still works (it is a focus stop — the selection bar must cover the collapsed and expanded shapes correctly).
- Answered escalations in History are untouched by this task.
- TUI rendering tests updated/added where output is asserted; `make test lint` green; one PR.

Constraints: TUI only — no MCP schema or verb changes (T5), no event changes (T3), generated views (T6) untouched. Scope is the task view; the dashboard's Needs Input rows have their own renderer and are out of scope.

Pointers: `cmd/tuhdoo/top.go` — `escalationRow`, `detailEscalations` (~line 717), the NEEDS INPUT section (~line 1030), focus-stop machinery (`detailStops`, `detailFocusIdx`). Related: the structured options/recommendation fields idea was deliberately deferred and captured separately in the inbox — do not build it here.

## History

### 2026-08-11 22:53 UTC — edit by `brandon/claude-code-1`

retitled · description edited · status inbox→open · labels +tui +ux

### 2026-08-12 05:23 UTC — run by `brandon/claude-code-bg` — done

- Branch: `tuh-8e86/task-view-escalation-rendering`
- PR: <https://github.com/brandonbews/tuhdoo/pull/73>
- Commits: `f1c8abf`
- Merged as: `883f6a7eb560a2fc3c6b3c38f7c2c4aa7973ed1b`

Landed via PR #73, merged as 883f6a7. Task view now renders escalation questions with their line structure (oneLine removed from that path) and collapses contexts to a first-line-plus-count stub; e toggles per escalation (free key, verified against all task-view bindings; watch mode flips all since it has no focus ring). Focus/selection/click/scroll correct in both shapes by construction and by test; answered escalations in History untouched; two new TUI tests, two goldens updated. One flagged deviation for review: the key hint rides the NEEDS INPUT section bar (enter answer · e context), not the bottom footer, which is full at 80 columns and would have dropped q quit; the PR body explains. Click-to-toggle not built: it did not fall out cheap (click already means select/answer). make test lint green.
