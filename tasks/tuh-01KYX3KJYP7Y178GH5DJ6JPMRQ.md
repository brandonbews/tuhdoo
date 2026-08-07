# Workflow recipes: recommended dev-flow patterns in init/docs (product feature, not this repo's conventions)

`tuh-01KYX3KJYP7Y178GH5DJ6JPMRQ`

- **Status:** open — blocked on dependencies
- **Priority:** 0
- **Labels:** `docs` `product`
- **Depends on:** [`tuh-qf4g`](tuh-01KZEPBEE8HFDQVK96AQNCQF4G.md) (open)
- **Created:** 2026-07-31 22:09 UTC by `brandon/claude-code-1`

## Description

Child of the launch epic (tuh-01KZEPBEE8HFDQVK96AV6RN0ER). Blocked on the strategy grill (tuh-01KZEPBEE8HFDQVK96AQNCQF4G), whose agenda item 6 decides this content's home: on the site, in `tuhdoo init` text, or both. The original prose gate (same v1-confidence gate as the site task) was satisfied and replaced by that dependency edge at the 2026-08-07 launch-epic structuring.

Context (from the 2026-07-31 grill cycle): tuhdoo's init/docs could offer a small collection of recommended outer-workflow recipes (e.g. trunk-based ticket→PR→squash) for host repos adopting tuhdoo. tuhdoo itself stays workflow-agnostic (agent-protocol step 3: ordinary git on ordinary branches); recipes are suggestions, never baked into the protocol. This repo's own PR flow is the first recipe candidate — a week-plus of dogfood results (PRs #1–#49+) is the evidence base.

The ask (to be sharpened when the grill records its home decision here): write the recipe collection, starting with the trunk-based PR flow this repo dogfoods, and ship it in the home the grill picks.

Acceptance (provisional until the grill closes): at least the trunk-based recipe written from real dogfood evidence, not speculation; recipes clearly framed as suggestions, never protocol; published in the decided home.

Constraints: workflow-agnosticism holds — nothing here changes the agent protocol or claim mechanics.

## History

### 2026-07-31 23:59 UTC — note from `brandon/claude-code-1`

Position statement for the future grill (2026-07-31 session with Brandon) — inputs, not decisions:

1. The boundary to defend is protocol-vs-workflow, not dumb-vs-smart. The primitives' contract (claim → work → escalate → finish loop, lease semantics, honest finish_run) ships IN the tool — it already does, via the MCP server instructions, and should stay there. Everything outer (branching shape, PR conventions, inbox-grooming rituals) ships in docs as recipes. "Dumb but powerful" is the slogan; "protocol in the tool, workflow in the docs" is the enforceable version.

2. Init should emit links to recipe docs after success. Precedent already exists: init today ends with CI guidance and a "Next: status · backlog · tui" line — non-binding guidance text after success is an established move.

3. Recipes carry copy-pasteable, per-host artifacts: AGENTS.md/CLAUDE.md blocks, vendorable skill files. Explicitly NOT a marketplace — no server, no accounts, no curation platform; that would reintroduce the exact stack the founding no-server/no-vendor principles avoid, through the docs side door. If demand for discovery/sharing materializes later, that's evidence for a future grill, not something to pre-build.

4. Candidate pitch line for the docs/marketing site (tuh-01KYX4Y0GZCJTQFNGPP6WMHDM2): "all it's really doing is letting me see and organize stuff while slightly slowing agents down" — the slowdown is the feature: work is forced through typed, visible transitions a human can steer.

5. Stronger delivery mechanism captured as its own held task sharing this gate: tuh-01KYX9VTPE9EBNBMBBNXY5XRAQ (init flavor picker — multiple-choice workflow setup that drops recipe files). Links-in-init is the floor, the picker is the ceiling; the grill decides how far up the axis to go.

### 2026-08-01 00:03 UTC — note from `brandon/claude-code-1`

Update (2026-07-31, same session): the init flavor picker (tuh-01KYX9VTPE9EBNBMBBNXY5XRAQ, point 5 of the previous note) was archived same-day on Brandon's call, accepting two arguments against building it: (1) staleness — scaffolding files baked into the shipped binary encode fast-moving harness conventions (skill formats, AGENTS.md idioms) that a docs page can fix in minutes but a binary teaches wrong indefinitely, with tuhdoo's authority behind it; (2) menu-of-one — only one flavor (Brandon's) exists or is proven, and a picker with one real option plus filler is worse than a link to one excellent doc. Surviving scope for this task is the floor: init emits links to recipe docs after success; recipes are docs pages with copy-pasteable, vendorable per-host blocks. If distinct proven flavors ever accumulate and formats settle, a picker could be re-raised as a fresh capture — nothing here forecloses it.
