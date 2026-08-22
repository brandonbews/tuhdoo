# Workflow recipes: recommended dev-flow patterns in init/docs (product feature, not this repo's conventions)

`tuh-01KYX3KJYP7Y178GH5DJ6JPMRQ`

- **Status:** done
- **Priority:** none
- **Labels:** `docs` `product`
- **Depends on:** [`tuh-qf4g`](tuh-01KZEPBEE8HFDQVK96AQNCQF4G.md) (done), [`tuh-kk4j`](tuh-01KZF1DNJ3T77A01NJXF1VKK4J.md) (done)
- **Created:** 2026-07-31 22:09 UTC by `brandon/claude-code-1`

## Description

Child of the launch epic (tuh-01KZEPBEE8HFDQVK96AV6RN0ER). Depends on the strategy grill (tuh-01KZEPBEE8HFDQVK96AQNCQF4G) and on the docs-swap task (tuh-01KZF1DNJ3T77A01NJXF1VKK4J), which creates the content root these pages live in.

HOME — settled at the strategy grill, agenda item 6, 2026-08-07 (Brandon's framing: "they are literally docs, just a specific section of the docs"): recipes live as ordinary pages in the root docs/ content root — a recipes section/subdirectory, writer's call — under the representation contract recorded on tuh-01KZ9YD2Q9V7BSZJBDGVNYM6QY: GFM, title+description frontmatter only, relative .md links, GitHub-renderable standalone. The WRITING BAR recorded there applies here in full (tight, TanStack/Next-calibre tone, no repo-internal jargon). Copy-pasteable per-host artifacts (CLAUDE.md/AGENTS.md blocks, vendorable skill files) are ordinary code fences within those pages. No recipe content ships in the binary — the archived flavor-picker decision stands. init's existing post-success guidance gains one line pointing at the docs on tuhdoo.com; bake the least-specific stable URL (domain root or /docs) — that URL is a permanent promise: site restructures must redirect it, never 404.

Context (from the 2026-07-31 grill cycle): tuhdoo's docs offer a small collection of recommended outer-workflow recipes (e.g. trunk-based ticket→PR→squash) for host repos adopting tuhdoo — common workflow examples describing ways to work with or set up tuhdoo, helping the mental model and getting people up and running. tuhdoo itself stays workflow-agnostic (agent-protocol step 3: ordinary git on ordinary branches); recipes are suggestions, never baked into the protocol. The boundary rationale lives in this task's 2026-07-31 position notes: protocol in the tool, workflow in the docs. This repo's own PR flow is the first recipe candidate — a week-plus of dogfood evidence (PRs #1–#49+).

The ask: write the recipe collection as docs pages, starting with the trunk-based PR flow this repo dogfoods; add the one-line init pointer.

Acceptance: at least the trunk-based recipe written from real dogfood evidence, not speculation; recipes clearly framed as suggestions, never protocol; pages live under docs/ per the contract and writing bar, rendering on GitHub and the site; init's post-success text includes the docs pointer (small Go change, tested, per repo conventions); make test lint green.

Constraints: workflow-agnosticism holds — nothing here changes the agent protocol or claim mechanics.

History: captured 2026-07-31; flavor-picker ceiling archived same day (see this task's notes); promoted into the launch epic 2026-08-07; home decided at the strategy grill 2026-08-07.

## History

### 2026-07-31 22:32 UTC — edit by `brandon/claude-code-1`

description edited · status inbox→held

### 2026-07-31 22:33 UTC — edit by `brandon/claude-code-1`

description edited

### 2026-07-31 23:59 UTC — note from `brandon/claude-code-1`

Position statement for the future grill (2026-07-31 session with Brandon) — inputs, not decisions:

1. The boundary to defend is protocol-vs-workflow, not dumb-vs-smart. The primitives' contract (claim → work → escalate → finish loop, lease semantics, honest finish_run) ships IN the tool — it already does, via the MCP server instructions, and should stay there. Everything outer (branching shape, PR conventions, inbox-grooming rituals) ships in docs as recipes. "Dumb but powerful" is the slogan; "protocol in the tool, workflow in the docs" is the enforceable version.

2. Init should emit links to recipe docs after success. Precedent already exists: init today ends with CI guidance and a "Next: status · backlog · tui" line — non-binding guidance text after success is an established move.

3. Recipes carry copy-pasteable, per-host artifacts: AGENTS.md/CLAUDE.md blocks, vendorable skill files. Explicitly NOT a marketplace — no server, no accounts, no curation platform; that would reintroduce the exact stack the founding no-server/no-vendor principles avoid, through the docs side door. If demand for discovery/sharing materializes later, that's evidence for a future grill, not something to pre-build.

4. Candidate pitch line for the docs/marketing site (tuh-01KYX4Y0GZCJTQFNGPP6WMHDM2): "all it's really doing is letting me see and organize stuff while slightly slowing agents down" — the slowdown is the feature: work is forced through typed, visible transitions a human can steer.

5. Stronger delivery mechanism captured as its own held task sharing this gate: tuh-01KYX9VTPE9EBNBMBBNXY5XRAQ (init flavor picker — multiple-choice workflow setup that drops recipe files). Links-in-init is the floor, the picker is the ceiling; the grill decides how far up the axis to go.

### 2026-08-01 00:03 UTC — note from `brandon/claude-code-1`

Update (2026-07-31, same session): the init flavor picker (tuh-01KYX9VTPE9EBNBMBBNXY5XRAQ, point 5 of the previous note) was archived same-day on Brandon's call, accepting two arguments against building it: (1) staleness — scaffolding files baked into the shipped binary encode fast-moving harness conventions (skill formats, AGENTS.md idioms) that a docs page can fix in minutes but a binary teaches wrong indefinitely, with tuhdoo's authority behind it; (2) menu-of-one — only one flavor (Brandon's) exists or is proven, and a picker with one real option plus filler is worse than a link to one excellent doc. Surviving scope for this task is the floor: init emits links to recipe docs after success; recipes are docs pages with copy-pasteable, vendorable per-host blocks. If distinct proven flavors ever accumulate and formats settle, a picker could be re-raised as a fresh capture — nothing here forecloses it.

### 2026-08-07 18:04 UTC — edit by `brandon/claude-code-1`

description edited · status held→open · depends_on +tuh-qf4g

### 2026-08-07 21:38 UTC — edit by `brandon/claude-code-1`

description edited · depends_on +tuh-kk4j

### 2026-08-07 21:43 UTC — edit by `brandon/claude-code-1`

description edited

### 2026-08-07 22:02 UTC — run by `brandon/claude-code-1` — done

- Branch: `tuh-pmrq/workflow-recipes`
- PR: <https://github.com/brandonbews/tuhdoo/pull/53>
- Merged as: `7ac7e349d1d87b9da794fc14183e38c068d57a10`

Landed on main as 7ac7e34 (PR #53, squash). New docs/recipes/ section: README.md index (states the protocol-vs-workflow boundary; recipes framed as suggestions, never protocol) and trunk-based-pr-flow.md — the trunk-based PR flow written host-agnostically from this repo's dogfood evidence, with a copy-pasteable CLAUDE.md/AGENTS.md block and an Adapting-it section; protocol steps link into agent-protocol.md, nothing copied. Both pages meet the representation contract (title+description frontmatter, relative links) and the writing bar (no repo-internal jargon). docs/README.md index updated. init post-success output gained one line — "Docs & workflow recipes: https://tuhdoo.com/docs" (least-specific stable URL, a permanent promise) — asserted in TestInitRemoteless. Binary changed (cmd/tuhdoo/commands.go): deploy restart happening right after this finish per CLAUDE.md.
