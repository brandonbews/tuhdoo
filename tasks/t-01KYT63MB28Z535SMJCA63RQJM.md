# Arm the TUI detail screen: selectable escalation, enter to answer; p/c on the viewed task

`t-01KYT63MB28Z535SMJCA63RQJM`

- **Status:** done
- **Priority:** 1
- **Labels:** `cli` `tui`
- **Depends on:** [`t-d83w`](t-01KYT63MB28Z535SMJC9B0D83W.md) (done), [`t-73kw`](t-01KYVD31CNTR1EVCDHPC5973KW.md) (done)
- **Created:** 2026-07-30 18:55 UTC by `4099114+brandonbews`

## Description

Context: originally parked pending dogfooding friction; the friction is now felt (steering feedback, Brandon, 2026-07-30): opening a blocked task's detail gives no way to answer its escalation, forcing esc → navigate → answer round-trips. Unparked to p1 and respecced: the original a/p/c key idea is replaced for answering — the escalation should be a selectable element, because enter-on-the-thing is how the rest of the TUI now works (see the same-day Needs Input task: enter on a Needs Input row answers in place).

The ask: in the armed detail screen (modeDetail): the task's open escalation(s) render as focusable items; focus moves with arrows/j-k and enter opens answer input with the standard footer; submit behaves exactly like answering from the list (same API call, same refresh). p (reprioritize) and c (cancel — its display verb may become 'archive'; see the same-day vocabulary task 'Rename the cancel interaction' and coordinate if both are in flight) act on the viewed task with the same y/n confirm as the list. Watch mode: detail stays fully read-only — no focusable affordances, no input.

Acceptance: input modes work from modeDetail with correct footers; answering targets the escalation belonging to the viewed task; watch mode fully disarmed (tested); model-driven tests per top_test.go patterns; make test lint green.

Constraints: boring Go; display/input only — no event or API changes.

## History

### 2026-07-31 06:16 UTC — edit by `brandon/claude-code-2`

retitled · description edited · priority none→1

### 2026-07-31 07:53 UTC — edit by `brandon/claude-code-9`

depends_on +t-73kw

### 2026-07-31 15:06 UTC — run by `brandon/claude-code-1` — done

- Branch: `main`
- Commits: `4acfc07`

Landed in commit 4acfc07. Armed detail (modeDetail) now steers in place: the viewed task's unanswered escalations render as focusable items (ULID order); enter opens the standard answer input targeting the focused escalation — same API call and refresh as the list path; p and c act on the viewed task with the same prompts/confirms as the list. New `back` field routes esc/submit back to whichever screen opened the input (list-opened prompts still return to the list, tested). Focus/scroll rule: j/k move focus when a further open escalation exists in that direction (window scrolls minimally to reveal it) and scroll one line otherwise — so with zero or one escalation, behavior is plain scrolling as before. Archive of the viewed task: y/n confirm; after archiving the detail stays open showing the archived status; esc returns to a list without its row. Watch mode fully disarmed: zero focusables by construction, read-only footer, enter/p/c dead (tested). One-shot `tuhdoo task` rendering byte-identical — the focus marker (string rewrite of the k-th "unanswered" line) applies only on the TUI path. Detail header now surfaces the status line (action results/errors were previously invisible from detail). make test lint green. Known edges (accepted, boring): p/c/answer on a task that reached terminal status under refresh aren't client-guarded — the daemon is the authority and errors surface in the status line; the focus marker anchors on the exact "unanswered" line historyOf emits (pinned by tests — a future wording change must move it); multi-escalation tasks trade some upward line-scrolling for focus jumps.
