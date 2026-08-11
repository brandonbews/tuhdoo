# Docs and landing copy pass: the skeptical evaluator, against the settled writing bar

`tuh-01KZF97PATRZ1TFWA7CQQCJHQX`

- **Status:** done
- **Priority:** 1
- **Labels:** `docs` `launch`
- **Created:** 2026-08-07 23:34 UTC by `brandon`

## Description

Context: promoted at the 2026-08-10 launch-polish grill (Brandon) from the capture "improve tone, brevity and audience of docs content". The writing bar itself was settled at the 2026-08-07 strategy grill and recorded on tuh-01KZ9YD2Q9V7BSZJBDGVNYM6QY: tight, TanStack/Next-calibre tone, no repo-internal jargon, terms defined on first use, maintainable by Brandon. This task is enforcement of that bar over the published surface, not a new standard.

Audience settled at the grill: the skeptical evaluator — a developer already running coding agents who has never heard of tuhdoo, deciding in minutes whether it beats their current setup (markdown TODO files, issue trackers, etc.). Every page leads with what this is, why git-native matters, and time-to-first-value; depth comes after the hook.

The ask: a rewrite pass over (a) the 7 human-facing published docs — everything in docs/ EXCEPT agent-protocol.md — currently ~5,000 words, and (b) the site landing page copy in site/src/app/page.tsx. Cut repo-internal framing (this repo's conventions are not the reader's), define terms on first use, lead with the evaluator's questions, tighten everywhere.

MESSAGING BRIEF (landing page contract — settled at the 2026-08-10 messaging grill, absorbing capture tuh-01KZFH07P6GB1X1J827PHREHMT):
1. The hero leads with the pain, then the fix in one breath. The villain: steering coding agents today is TODO.md files and vibes — parallel agents trample each other, sessions die and take their context with them, nothing records what actually happened.
2. The shared-organism story is the immediate second beat, elevated — not a mid-page card: the backlog, roadmap, and activity ledger are part of the same organism as the application. One repo, one clone, one history. The serverless / owned / git-tracked / still-distributed emphasis is this beat plus the ownership section, never a footnote.
3. Product voice throughout, plus ONE deliberate first-person maker block — grown from the existing pull-quote — where "why I built this / why I love using it" lives in Brandon's words.
4. Brandon's four loves are woven where each is strongest, in product voice, AND echoed in the maker block: (a) same-organism up top; (b) inbox → sculpt into workability with an agent → watch the drain gets its own section as the HUMAN's loop, peer to the agent loop (currently absent from the page); (c) working with a team without worrying about who has what on which branch, and (d) no service / no subscription / no signup, both carry the ownership section. No fixed order among the loves — place by strength.
The docs README opening echoes this same story in miniature. The identity task (tuh-01KZF973FY9JKJV5F38SM7BAN7) designs to the section structure this brief produces — another reason this task lands first.

Acceptance: each touched page opens by answering the evaluator (what is this / why should I care) before mechanics; the landing implements the messaging brief above (pain-led hero, organism second beat, maker block, human-loop section); no unexplained repo-internal jargon survives; overall word count goes down (brevity is a goal, not a quota); the docs contract holds — GFM only, frontmatter restricted to title + description, relative real-file .md links, GitHub rendering stays clean as the semantic baseline; agent-protocol.md is untouched; Brandon reviews the diff before merge (the docs are his voice — escalate with the PR link).

Pointers: writing-bar record on tuh-01KZ9YD2Q9V7BSZJBDGVNYM6QY; content contract (REPRESENTATION block) on the same task; nav/ordering lives in site config (site/src/lib/nav.ts), never in content.

Constraints: agent-protocol.md is exempt and out of bounds — its wording is a load-bearing prompt steering agent behavior; any edit to it is its own deliberate task with behavioral re-verification. Soft ordering: landing before the identity pass (tuh-01KZF973FY9JKJV5F38SM7BAN7) so design works with final copy — start this one first.

## History

### 2026-08-10 22:09 UTC — escalation from `brandon/claude-code-1` (blocking)

> Review PR #62 (https://github.com/brandonbews/tuhdoo/pull/62) — the docs + landing copy pass. Approve and merge (squash), or leave change requests on the PR and answer here so the next claimant addresses them.

The rewrite is complete and green (make test lint + site build): all 7 human-facing docs tightened for the skeptical evaluator (6,464 → 6,133 words, agent-protocol.md untouched, links/anchors/frontmatter contract verified, uninstall's tested blocks byte-identical), and the landing page implements the full messaging brief — pain-led hero, organism second beat, new human-loop section ("capture, sculpt, drain"), ownership section, and the one first-person maker block. The task's acceptance requires your eyes on the diff before merge because the docs and especially the maker block are written in your voice — please check that block's wording closely (it grew from the old pull-quote and now signs "— Brandon, tuhdoo's maintainer"). Auto-merge is deliberately NOT set. Options: (a) wording is right → approve and squash-merge PR #62 yourself, or answer "merge it" and the next claimant merges and finishes; (b) wording needs work → leave line comments on the PR and answer here with "address comments"; the task returns to the pool and the next claimant picks them up. My recommendation: option (a) after any direct edits you want to push to the branch yourself — copy nits are faster edited than round-tripped.

**Answer** (`brandon`, relayed by `brandon/claude-code-1`): the copy changes are good to merge

### 2026-08-10 22:09 UTC — note from `brandon/claude-code-1`

Resume state: branch tuh-jhqx/docs-landing-copy, PR #62 open, no auto-merge (Brandon's review is the gate — escalation 01KZPVJHTJ31GSH2XRM6TAJY43). Next claimant after the answer lands: if approved/merged, record the squash commit in merged_as and finish done (confirm_claim before merging if the merge is yours to do); if change requests, address PR comments on the same branch. Soft ordering from the task: the identity/design task tuh-01KZF973FY9JKJV5F38SM7BAN7 designs to this PR's section structure (hero / organism / agent loop / human loop / ownership / maker block / get started).

### 2026-08-10 22:09 UTC — run by `brandon/claude-code-1` — blocked

- Branch: `tuh-jhqx/docs-landing-copy`
- PR: <https://github.com/brandonbews/tuhdoo/pull/62>
- Commits: `7803083`

Rewrite done and green on PR #62; blocked on Brandon's review per the task's acceptance — see escalation 01KZPVJHTJ31GSH2XRM6TAJY43.

### 2026-08-11 02:39 UTC — run by `brandon/claude-code-1` — done

- Branch: `tuh-jhqx/docs-landing-copy`
- PR: <https://github.com/brandonbews/tuhdoo/pull/62>
- Merged as: `10afcb614d7df4120d0518523f6a4bae3a1c1bf3`

PR #62 squash-merged to main as 10afcb6 after Brandon's approval ("the copy changes are good to merge"). All 7 human-facing docs rewritten for the skeptical evaluator (word count 6,464 → 6,133, agent-protocol.md untouched, docs contract verified); landing page implements the full messaging brief (pain-led hero, organism second beat, human-loop section, ownership section, maker block). Landing section structure for the identity task: hero / organism / agent loop / human loop / ownership / maker block / get started. No Go changes — no daemon restart needed.
