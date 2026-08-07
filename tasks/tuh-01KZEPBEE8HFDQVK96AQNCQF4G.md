# Grill: site stack & content strategy (human-led)

`tuh-01KZEPBEE8HFDQVK96AQNCQF4G`

- **Status:** open — ready
- **Priority:** 2
- **Labels:** `design` `product` `web`
- **Created:** 2026-08-07 18:04 UTC by `brandon/claude-code-1`

## Description

HUMAN-LED: this is a grill session Brandon runs interactively. Agents must not work this task — if you are an agent holding this claim, release it with reason "human-led grill".

Context: the launch epic's decision gate. Three build tasks depend on this one session: the marketing/docs site (tuh-01KYX4Y0GZCJTQFNGPP6WMHDM2), user-facing docs (tuh-01KZ9YD2Q9V7BSZJBDGVNYM6QY), and workflow recipes (tuh-01KYX3KJYP7Y178GH5DJ6JPMRQ). Their captures reserved these decisions on purpose ("needs its grill before any scoping or code"); the 2026-08-06 triage observed the site's docs strategy and the user-docs ask are the same decision surface, so the reserved grills were consolidated here (2026-08-07 launch-epic structuring).

The agenda — every deliberately-reserved decision:
1. Framework: Next is Brandon's team-habit default — challenge it against Astro/Starlight/plain static for a docs+marketing site.
2. Hosting/deploy: prefer a path adding zero .github/workflows files (e.g. Vercel/Netlify watching the repo); any workflow file triggers the eyes-on-review law.
3. Domain (tuhdoo.com assumed, not decided) — and who buys it, when.
4. One site or two: marketing and docs together or split.
5. Docs representation: Brandon's ask is platform-agnostic content renderable on the site or elsewhere — pick the format and where it lives relative to site/.
6. Workflow recipes' home: on the site, in `tuhdoo init` text, or both.

Acceptance: each agenda item resolved and recorded into the depending tasks' descriptions (edit them in place); any launch-execution work the grill surfaces (domain purchase, announcement moment, launch checklist) created as new children of the launch epic; this task closed done with a run note pointing at where the decisions landed.

Pointers: the three depending tasks' descriptions carry the accumulated capture context; docs/design/001-core-design.md and 002-technology.md for the laws (T2 host-agnosticism, workflow-file review law); one week of dogfood evidence in this repo's PR history.

Constraints: the monorepo site/ decision is already made (2026-07-31 capture) — the grill inherits it, doesn't re-litigate it. The site/ work is also the designated live test for the monorepo-grain question (tuh-01KZA0VT234XJYVZWT8YFV8XE2, held) — note friction signals as they appear.

## History

### 2026-08-07 21:18 UTC — note from `brandon/claude-code-1`

Checkpoint 2026-08-07 (Brandon-led session): agenda item 5 (docs representation) SETTLED. The decision: render targets are tuhdoo.com + GitHub file browsing + raw terminal text (not the binary); format is GFM + YAML frontmatter restricted to title + description, with navigation/ordering in site config never in content; root docs/ becomes the published content root (directory = publish boundary) with working docs moving to internal-docs/, and joining.md + uninstall.md + agent-protocol.md going public; links are relative real-file .md paths with GitHub as the semantic baseline, the site rewrites at build. Rationale thread: Brandon's framing — one surface to maintain, maximum reuse (site now, maybe Mintlify/TUI viewer later), established conventions over bespoke magic.

Recorded into: tuh-01KZ9YD2Q9V7BSZJBDGVNYM6QY (full contract, REPRESENTATION block) and tuh-01KYX4Y0GZCJTQFNGPP6WMHDM2 (site-side consequences). Execution surfaced: docs-swap task tuh-01KZF1DNJ3T77A01NJXF1VKK4J (new epic child, claimable now; user-docs and site tasks now also depend on it) and inbox capture tuh-01KZF1DNJ3T77A01NJXHW4QGAW (views.go:141 generated-README wording).

Carry-forward for item 1 (framework): the contract narrows it — the framework must consume ../docs as plain GFM with .md-link rewriting cheaply; this demotes framework from architecture decision to first consumer of a stable contract. Items 1, 2, 3, 4, 6 still open.

### 2026-08-07 21:33 UTC — note from `brandon/claude-code-1`

Checkpoint 2026-08-07 (same Brandon-led session, continued): items 1, 2, and 4 SETTLED, recorded into the site task (tuh-01KYX4Y0GZCJTQFNGPP6WMHDM2). Item 4: one site at launch (marketing + docs), split-ready by construction via the content contract; Mintlify-later stays an option, not a plan. Item 1: Next.js — Brandon's call with the Astro challenge heard; docs rendered by a thin unified/remark pipeline (established plugins + small link-rewrite visitor), hard constraint GFM-never-MDX; Fumadocs/Nextra explicitly ruled out. Item 2: Vercel GitHub App, root=site/, PR previews, zero .github/workflows files; all pages statically generated, output:'export' deferred. Remaining agenda: item 3 (domain — tuhdoo.com assumed, who buys/when) and item 6 (workflow recipes' home).

### 2026-08-07 21:43 UTC — note from `brandon/claude-code-1`

Final checkpoint, 2026-08-07 (Brandon-led session): items 3 and 6 settled; agenda complete; task closed done.

Item 3 (domain): tuhdoo.com is already Brandon's — Namecheap, registered 2026-03-26, parked, expires 2027-03-26 (verified via whois this session). No purchase needed; Vercel project + DNS wiring is the new human-led epic child tuh-01KZF2D3MA0P24WKAWK89J0Q0X, blocked on the site task. Item 6 (recipes' home): recipes are literally docs — a section of the published docs/ root under the same content contract; no recipe content in the binary (archived flavor-picker decision stands); init's post-success text gains one line pointing at the least-specific stable tuhdoo.com URL (permanent redirect promise). Recorded on tuh-01KYX3KJYP7Y178GH5DJ6JPMRQ.

Late additions from Brandon at close: (a) site component discipline — docs renderer assembled from a discrete, explicitly-listed inventory of small components with the GFM→component mapping legible in one file (recorded on the site task); (b) writing bar for all published docs — tight, TanStack/Next-calibre tone, no repo-internal jargon, terms defined on first use, maintainable by Brandon (recorded on the user-docs task, referenced from recipes).

Where every decision landed: site task tuh-01KYX4Y0GZCJTQFNGPP6WMHDM2 (items 1, 2, 3, 4, 5-site-side + component discipline); user-docs task tuh-01KZ9YD2Q9V7BSZJBDGVNYM6QY (item 5 full contract + writing bar); recipes task tuh-01KYX3KJYP7Y178GH5DJ6JPMRQ (item 6). Execution surfaced: docs-swap tuh-01KZF1DNJ3T77A01NJXF1VKK4J (epic child, claimable), DNS wiring tuh-01KZF2D3MA0P24WKAWK89J0Q0X (epic child, human-led), views.go:141 wording capture tuh-01KZF1DNJ3T77A01NJXHW4QGAW (inbox). No announcement/launch-checklist child created — deliberate; capture when launch nears.

Closed by status update at Brandon's direction — human-led task, no agent claim/run by design.
