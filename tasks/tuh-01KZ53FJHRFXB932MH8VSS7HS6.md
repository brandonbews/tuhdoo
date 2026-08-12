# TUI bar recolors: dim-red BLOCKED, bright-white INBOX (section order confirmed unchanged)

`tuh-01KZ53FJHRFXB932MH8VSS7HS6`

- **Status:** done
- **Priority:** 0
- **Labels:** `tui`
- **Created:** 2026-08-04 00:41 UTC by `brandon`

## Description

Context: triaged 2026-08-04 from the inbox capture "assess order of sections. what would a user really expect in order from top to bottom?". All three embedded questions were settled interactively with Brandon, who judged live swatches rendered in his own terminal (a scratchpad script painting the exact render.go SGRs plus candidates).

Decision 1 — section order is CONFIRMED UNCHANGED: NEEDS INPUT → READY → IN PROGRESS → BLOCKED → ON HOLD → INBOX. Rationale recorded here so it isn't re-litigated: urgency-of-my-action, not taxonomic relatedness, decides screen position. The top region is interrupt real estate — NEEDS INPUT is self-emptying and normally empty, so a magenta bar appearing genuinely means "act". INBOX is unbounded by design (one-keystroke capture) and the TUI deliberately has no promotion key — a read-only section must not outrank the steering sections (READY holds the `p` key; IN PROGRESS is the fleet-health glance). No code or doc change for order; this task record is the decision trail.

Decision 2 — BLOCKED bar goes dim red: `\x1b[2;41m` (dim default-fg on red bg) replacing `\x1b[30;41m` (black on red). Basis: the BLOCKED section shows only unmet-dependency tasks (escalation-blocked tasks render solely under NEEDS INPUT, 2026-07-31 grill) — ordinary sequencing that self-resolves, not a fire. Dim red keeps the hue family while dropping the alarm, and stays inside the 16-color law.

Decision 3 — INBOX bar goes bright-white: `\x1b[30;107m` (black on bright-white) replacing reverse-dim (`rev+dim`). Chosen by eye against the live bars; clearly distinct from ON HOLD's dark gray (`2;100`) at a glance. Inbox ROWS stay dim (the section's dim flag is untouched). Note: this revises the T7 shelves wording ("reverse-dim bars") — bright-white is louder than that phrase implies, and Brandon chose it looking at the rendered screen; carry a dated revision note rather than silently drifting.

The ask:
- cmd/tuhdoo/render.go newColors: change bgRed's value to `\x1b[2;41m`, and introduce a named bg code for the inbox bar (today it's composed inline as `c.rev + c.dim` in topSections) with value `\x1b[30;107m` — the bar joins the named bg* family, defined in exactly one place. Rename fields if the old names now lie (bgRed no longer black-on-red); naming to taste.
- cmd/tuhdoo/top.go topSections (~line 1292): blocked and inbox entries pick up the new codes.
- The blocked row's `waiting:` lead (secondLine with col.red, top.go ~1592): soften to dim red (`2;31`) to match — a full-brightness red lead inside the row would now be louder than its own section bar. If that reads badly in the goldens, escalate with a screenshotable golden diff rather than inventing a third treatment.
- Update the chrome-hierarchy comments in render.go/top.go and the "reverse-dim bars" phrase in docs/design/002-technology.md T7 ("The shelves and quick capture") with a dated revision note (2026-08-04, this task).
- Regenerate/adjust golden tests — cmd/tuhdoo/top_golden_test.go hardcodes the colors struct and must mirror the new values.

Acceptance criteria:
- BLOCKED bar renders `2;41`, INBOX bar `30;107`; every other bar byte-identical to before; goldens prove it.
- blocked `waiting:` lead renders dim red.
- No 256-color or truecolor codes introduced anywhere except the existing selBG exception — the 16-color law ("user themes must survive", render.go header) holds without a new exception.
- Section order in buildRows/topSections untouched; existing order tests still green.
- NO_COLOR / non-TTY degrade path untouched (zero-value colors still render plain text with the same geometry).
- make test lint green from the repo root.

Pointers: cmd/tuhdoo/render.go (colors struct + newColors, lines ~12–46), cmd/tuhdoo/top.go (topSections ~1292, buildRows ~152, blocked second line ~1590), cmd/tuhdoo/top_golden_test.go, docs/design/002-technology.md T7.

Constraints: TUI-only — bg* codes never leak into one-shot command output (that surface is serialization, not design, per the 2026-07-31 contract); boring Go; no new color-capability detection.

## History

### 2026-08-04 08:00 UTC — edit by `brandon`

description edited

### 2026-08-04 17:59 UTC — edit by `brandon/claude-code-1`

retitled · description edited · status inbox→open · labels +tui

### 2026-08-05 02:36 UTC — run by `brandon/claude-code-1` — done

- Branch: `tuh-7hs6/tui-bar-recolors`
- PR: <https://github.com/brandonbews/tuhdoo/pull/31>
- Commits: `cd83645622855639af6ec3d4c7470560ddbffb38`

TUI bar recolors landed (PR #31, squash cd83645). BLOCKED bar now 2;41 (dim-fg on red), INBOX bar new named bgWhite 30;107 (black on bright-white), blocked waiting: lead new dimRed 2;31; all other bars byte-identical, goldens prove it (new TestTopGoldenBlockedWaitingLead covers the previously-unexercised lead). bgWhite/dimRed join the named color family in render.go newColors; no renames needed. 002 T7 shelves wording carries the dated 2026-08-04 revision note per the task ask. 16-color law holds (107 is aixterm 16-color; selBG still the only exception); NO_COLOR/non-TTY path untouched; section order untouched. make test lint green. Binary changed - deploy restart follows this finish.
