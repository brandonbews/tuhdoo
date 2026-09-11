# TUI keymap rework: r/h/i move status, n captures, tab opens Closed, priority picker with clear

`tuh-01M28YZVWS1PJEMG9J7VM0QK8H`

- **Status:** open — in progress, claimed by `brandon/claude-code-1`
- **Priority:** 1
- **Labels:** `go` `tui` `design-revision` `docs`
- **Depends on:** [`tuh-1drx`](tuh-01M28YZVWS1PJEMG9J7QN11DRX.md) (done), [`tuh-tsv0`](tuh-01M28YZVWS1PJEMG9J7TZKTSV0.md) (done)
- **Created:** 2026-09-11 19:26 UTC by `brandon/claude-code-1`

## Description

Context: decided 2026-09-11 (keyboard/priority grill, Brandon). Moving a task between ready, on hold, and inbox is a one-keystroke steering act, and priority entry is a picker, not a free text field. Today: h opens the done/cancelled shelf ("history"), i is quick capture, p opens a text prompt with no instructions and no way to clear, and section bars carry per-section key hints. The design doc's sentence "The TUI deliberately has no promotion key" (002 T7, shelves paragraph) is withdrawn: users move tasks freely; whether someone captures and promotes in the same breath is not the design's business. Remove the sentence; do not replace it with a rule. Depends on the clearable-priority task (the picker's clear) and the prompt-overlay task (the picker is born in the box).

The ask — keys (armed mode only unless noted; watch mode keeps these dead and the legend does not show them):
1. `r` sets status open ("ready"), `h` sets held ("on hold"), `i` sets inbox. Each is one `PATCH` with the status field, no confirmation — every move is reversible with one keystroke. No-op, silently, when the task is already in that status. Dead, silently, on done/cancelled tasks and on in-progress tasks (a live claim) — pausing work someone holds goes through cancel and its y/n. Blocked tasks are open: r is a no-op, h and i work.
2. Same three keys in the task view on the viewed task, exactly as p and c work there.
3. `n` is quick capture ("new"), replacing i. Dashboard only, as capture is today.
4. `tab` toggles between the backlog and the done/cancelled shelf, which is renamed **Closed** everywhere it is named (the model field, row builder, footer legend, comments, goldens). `esc` from Closed still returns to the backlog. Works in watch mode (browsing is reading). h no longer opens it.
5. Priority picker: `p` opens a one-line prompt in the overlay box: label `priority tuh-xxxx (title) · now p2` (or `· now none`), hint `0-9 sets · - clears · esc`. A digit 0-9 writes that priority at once and closes; `-` sends clear_priority and closes; esc closes; every other key is ignored. A priority outside 0-9 set elsewhere shows in `now` and can be overwritten or cleared, not re-entered. The textInput mode for priority is deleted.
6. Section bars drop their key hints (the fourth column of the sections table near the READY/ON HOLD/INBOX entries, and NEEDS INPUT's "enter answer"): keys work on every task in every bucket, so the footer legend is the single place keys are advertised. The list legend (armed) reads: ↑/↓ (j/k) move · enter open · r ready · h hold · i inbox · p priority · c cancel · n new · tab closed · q quit; watch: move · open · tab closed · quit. The task view legend gains r/h/i beside p and c. Closed's legend: move · open · esc back · q quit. If the legend overflows narrow widths, legendLine's existing wrapping handles it — verify with a golden at 60 columns.
7. Design docs, revised in place with dated revision notes (CLAUDE.md conventions): in 002 T7's shelves-and-quick-capture paragraph delete the "no promotion key" sentence and its justification; change "Quick capture: `i`" to `n`; record r/h/i and tab/Closed in a new dated paragraph after it (what the keys do and the in-progress/closed dead rule — rules only, no enumeration of what users may do); in the 2026-08-02 history-view mention (grep "history view" in 002) add a note that the shelf is now Closed on tab. docs/adopting.md's TUI sentence: add "move tasks between ready, on hold, and inbox" to the verb list. Nothing in docs/agent-protocol.md changes: its promotion bar is protocol for agents, not a TUI rule.

Acceptance (tests must exist and pass):
- TUI interaction tests (existing top_test.go harness) for each of r/h/i on: a ready row (r no-op, h and i send the expected PATCH body), a held row, an inbox row, a blocked row, an in-progress row (all three dead), a done row in Closed (dead), and in watch mode (dead). Same for the task view.
- n opens capture; i no longer does; tab toggles Closed from both modes; h on the dashboard never opens Closed.
- Picker: p then 3 sends priority 3 and closes; p then - sends clear_priority and closes; p then x does nothing and stays open; p then esc closes with no request; p on a closed or in-progress task does nothing; label shows `now none` for an unprioritized task and `now p12` for one set to 12 via the API.
- Goldens: list and task view legends in armed, watch, and Closed; section bars with no hints; the picker box; a 60-column legend.
- No bar or footer in watch mode names r/h/i/n/p/c.
- `make test lint` green; PR title = task title; body opens with this task ID.

Pointers: cmd/tuhdoo/top.go (updateNav, updateDetail, updateInput, submit, the mode consts near line 246, the sections table near line 1581 with its hint column, footerView, detailFooter, legendLine, buildHistoryRows and the m.history field, inputFooter's modePriority arm), cmd/tuhdoo/api.go and client.go (setPriority, cancelTask, captureTask — add setStatus and clearPriority beside them), cmd/tuhdoo/top_test.go and top_golden_test.go, internal-docs/design/002-technology.md T7, docs/adopting.md section 5.

Constraints: boring Go; no new colors; the one-shot CLI output is untouched (T7 output contract); the held display word stays "on hold" from its single mapping; `held` stays the stored and API word — the TUI sends status "held", never "on hold"; no new MCP tools or fields. Deploy after landing per CLAUDE.md.

## History

_No activity yet._
