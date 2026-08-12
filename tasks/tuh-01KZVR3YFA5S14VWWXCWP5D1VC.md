# Test the escalation flow as a human

`tuh-01KZVR3YFA5S14VWWXCWP5D1VC`

- **Status:** open — waiting on an escalation answer
- **Priority:** 0
- **Labels:** `test` `ux`
- **Created:** 2026-08-12 19:45 UTC by `brandon`

## Description

A throwaway fixture task: it exists so Brandon can walk the answering UX end to end in the task view — two open escalations below, both asking real questions about the escalation UX itself. Archive the task when done playing.

## History

### 2026-08-12 19:46 UTC — escalation from `brandon/claude-code-1` (blocking)

> Fixture question 1 (short, decision-shaped): structured escalation options are still an inbox task (tuh-8x0g — escalate would gain pickable options + a recommendation field, rendered as selectable answers in the TUI). Now that you are answering this through the redesigned task view: (a) keep that task — picking beats typing, especially for yes/no steers; (b) cancel it — free-text answering feels sufficient now that the question is always visible. My recommendation: (a), weakly — most real escalations I have raised were choices between two or three named options.

This escalation is a test fixture: Brandon asked for a throwaway task with real UX questions so he can walk the answering flow end to end. The referenced decision is genuine though — answering it here saves a future triage pass.

**Answer** (`brandon`): i like a, but maybe that's a more robust ux change. i feel like escalations are a tricky ux, and i would want to get it right. we can keep the task, but lets put it on hold and make sure to note that we want grill whether the answering experience should open on enter (and look like claude code's qa module) o rbe something simpler.

### 2026-08-12 19:46 UTC — escalation from `brandon/claude-code-1` (blocking)

> Fixture question 2 (deliberately long context below): at this context length, does collapsed-by-default feel right, or should short contexts render expanded and only collapse past a threshold?

This context block is intentionally long so you can judge the collapse UX at its worst. What PR #73 changed: the structured question is always visible in the task view, while this background section starts collapsed and expands on demand — the bet being that the question alone usually carries the decision and the context is a fallback, not the lead. Things worth noticing while you are here: whether the collapsed indicator makes it obvious there is more to read; whether expanding and re-collapsing leaves the cursor where you expect; whether long lines wrap cleanly at your terminal width inside the expanded block; whether the escalation you are not currently looking at stays selectable without scrolling confusion; and whether answering from the expanded state feels different from answering with context collapsed. A second thing to evaluate: two open escalations on one task is the multi-escalation rendering path — check the Needs Input row for this task only shows it once, and that the task view lists both questions in a sane order (oldest first, presumably). If any of that feels off, that observation is itself the answer to record here.

_Unanswered._

### 2026-08-12 19:46 UTC — run by `brandon/claude-code-1` — blocked

Fixture run: raised two test escalations for Brandon's escalation-UX walkthrough. Nothing to build; archive the task once the walkthrough is done.
