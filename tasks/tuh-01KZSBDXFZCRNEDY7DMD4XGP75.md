# Copy tightening pass: docs and site to a tighter, utilitarian voice (launch gate)

`tuh-01KZSBDXFZCRNEDY7DMD4XGP75`

- **Status:** open — blocked on dependencies
- **Priority:** 1
- **Labels:** `docs` `web` `launch`
- **Depends on:** [`tuh-7q0m`](tuh-01KZSBC7K0GNYNYTTAM6DW7Q0M.md) (open)
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

_No activity yet._
