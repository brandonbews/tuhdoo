# Docs and landing copy pass: the skeptical evaluator, against the settled writing bar

`tuh-01KZF97PATRZ1TFWA7CQQCJHQX`

- **Status:** open — ready
- **Priority:** 1
- **Labels:** `docs` `launch`
- **Created:** 2026-08-07 23:34 UTC by `brandon`

## Description

Context: promoted at the 2026-08-10 launch-polish grill (Brandon) from the capture "improve tone, brevity and audience of docs content". The writing bar itself was settled at the 2026-08-07 strategy grill and recorded on tuh-01KZ9YD2Q9V7BSZJBDGVNYM6QY: tight, TanStack/Next-calibre tone, no repo-internal jargon, terms defined on first use, maintainable by Brandon. This task is enforcement of that bar over the published surface, not a new standard.

Audience settled at the grill: the skeptical evaluator — a developer already running coding agents who has never heard of tuhdoo, deciding in minutes whether it beats their current setup (markdown TODO files, issue trackers, etc.). Every page leads with what this is, why git-native matters, and time-to-first-value; depth comes after the hook.

The ask: a rewrite pass over (a) the 7 human-facing published docs — everything in docs/ EXCEPT agent-protocol.md — currently ~5,000 words, and (b) the site landing page copy in site/src/app/page.tsx. Cut repo-internal framing (this repo's conventions are not the reader's), define terms on first use, lead with the evaluator's questions, tighten everywhere.

Acceptance: each touched page opens by answering the evaluator (what is this / why should I care) before mechanics; no unexplained repo-internal jargon survives; overall word count goes down (brevity is a goal, not a quota); the docs contract holds — GFM only, frontmatter restricted to title + description, relative real-file .md links, GitHub rendering stays clean as the semantic baseline; agent-protocol.md is untouched; Brandon reviews the diff before merge (the docs are his voice — escalate with the PR link).

Pointers: writing-bar record on tuh-01KZ9YD2Q9V7BSZJBDGVNYM6QY; content contract (REPRESENTATION block) on the same task; nav/ordering lives in site config (site/src/lib/nav.ts), never in content.

Constraints: agent-protocol.md is exempt and out of bounds — its wording is a load-bearing prompt steering agent behavior; any edit to it is its own deliberate task with behavioral re-verification. Soft ordering: landing before the identity pass (tuh-01KZF973FY9JKJV5F38SM7BAN7) so design works with final copy — start this one first.

## History

_No activity yet._
