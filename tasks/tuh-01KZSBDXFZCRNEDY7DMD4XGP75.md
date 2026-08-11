# Copy tightening pass: docs and site to a tighter, utilitarian voice (launch gate)

`tuh-01KZSBDXFZCRNEDY7DMD4XGP75`

- **Status:** open — in progress, claimed by `brandon/claude-code-4`
- **Priority:** 1
- **Labels:** `docs` `web` `launch`
- **Depends on:** [`tuh-7q0m`](tuh-01KZSBC7K0GNYNYTTAM6DW7Q0M.md) (done)
- **Created:** 2026-08-11 21:25 UTC by `brandon`

## Description

Context: Brandon's verdict 2026-08-11, hours after the previous copy pass (#62) landed: docs and marketing copy "need to be significantly tighter, less fluffy, and more utilitarian in message without losing a concise compelling pitch. They read written by an impatient AI right now with em dashes and fragments and vague language all over (like the 12 verb thing and others like it)." The 2026-08-11 announcement grill made this a launch gate: the Show HN moment fires only after this and the protocol slim-down land, because this copy is the first thing every evaluator reads.

The ask: a full tone-and-tightness pass over all published copy — `docs/*.md` (including recipes) and the site (`site/src`, landing page and docs chrome). The named failure modes, from Brandon directly: em-dash cadence, sentence fragments, vague insider shorthand presented as if self-explanatory ("the 12 verb thing" — either say concretely what the twelve verbs buy the reader or don't mention the count), and fluff generally. Utilitarian and concrete, but the pitch must stay compelling and concise — this is not a flattening into blandness.

Acceptance:
- Every published page passes the bar: no em-dash-fragment cadence, no unexplained insider shorthand, no filler sentences; claims concrete and verifiable.
- CRITICAL — calibrate before sweeping: the previous pass (#62) was judged insufficient the same day it landed, so guessing the bar again is the known failure mode. Rewrite ONE high-traffic page first (the landing page), escalate with before/after, and only sweep the rest after Brandon confirms the register.
- Brandon's PR review is the final bar. `make test lint` green (site typecheck/lint included); one PR, or landing+docs as two if cleanly separable.

Constraints: `docs/` publishing rules (GFM, frontmatter title + description only, relative links, GitHub rendering as semantic baseline). Do not change technical meaning anywhere — tone only; if a sentence seems wrong rather than fluffy, escalate instead of silently "fixing" it. `docs/agent-protocol.md` and supplementary protocol material are OWNED by the protocol slim-down task (tuh-01KZSBC7K0GNYNYTTAM6DW7Q0M, a dependency of this one) — this pass may polish the slimmed result's tone but must not restructure it.

Pointers: #62's PR body records the prior "settled writing bar" — read it to see what already failed; this task's bar supersedes it.

## History

### 2026-08-11 23:49 UTC — escalation from `brandon/claude-code-4` (blocking)

> The landing page is rewritten to a stricter register (branch tuh-gp75/copy-tightening, commit 190cb59) as the calibration slice your task mandated. Does it hit the bar, so the full docs+site sweep proceeds against it? Options: (a) register confirmed as-is, sweep everything; (b) mostly right, sweep after applying your corrections; (c) still off, re-calibrate the landing page again from your notes. Recommendation: (a). Three small calls folded in: 1) 'the roadmap' kept on the landing page but may be an overclaim (the ledger holds tasks, priorities, and dependency edges; there is no roadmap object) — keep or cut? 2) landing page says 'twelve tools' where docs say 'twelve verbs' — which is the term? 3) name the TUI concretely ('watch progress from the tuhdoo TUI') or keep the plain 'watch from the terminal'?

Register rules applied, for you to confirm or correct (the rules matter more than the instances): zero em dashes in body copy (parentheticals became colons, semicolons, or second sentences); complete sentences in prose, fragments only in headings where the fragment is the claim; counts and jargon only with what they buy the reader in the same breath ('twelve tools, few enough that the whole protocol fits in the single instruction file a harness loads'; 'fully hydrated' became the actual list); metaphors and filler cut ('same organism', 'one brain', 'markdown files and vibes'); tuhdoo as grammatical subject where it acts. Example, hero lede — before: 'Steering coding agents today is markdown files and vibes: parallel agents trample each other, sessions die with their context, nothing records what happened. tuhdoo is the fix — a shared backlog, work queue, and activity ledger on a git branch inside the repo it plans. No server, no vendor, no accounts.' After: 'Steering coding agents with TODO files breaks at fleet scale: parallel agents trample each other's work, sessions die and take their context with them, and nothing records what actually happened. tuhdoo replaces the TODO file with a shared backlog, work queue, and activity ledger, stored on a git branch inside the repo it plans. It syncs over the remote you already have and needs no server, no vendor, and no accounts.' Full before/after is the commit diff (one file, site/src/app/page.tsx). Not yet touched, in sweep scope: docs/*.md, docs chrome, and layout.tsx's footer tagline and site metadata, which still carry the old cadence.

_Unanswered._
