# Marketing / docs site for tuhdoo (monorepo: site/ in this repo)

`tuh-01KYX4Y0GZCJTQFNGPP6WMHDM2`

- **Status:** on hold — deliberately paused
- **Priority:** 0
- **Labels:** `docs` `product` `web`
- **Created:** 2026-07-31 22:32 UTC by `brandon/claude-code-1`

## Description

Gated: unpark for a grill cycle once v1 confidence exists — same gate as the workflow-recipes task (tuh-01KYX3KJYP7Y178GH5DJ6JPMRQ): the v0 dogfood week has held and the trunk-based PR flow has been exercised. Perfect the dish before writing down the recipe; a marketing site for a pre-v1 tool markets the wrong thing.

Decided at capture (2026-07-31 intake): the site lives IN THIS REPO as a self-contained site/ directory — monorepo — because tuhdoo's ledger is repo-scoped: site work in this repo stays steerable from this backlog, while a separate repo would need its own tuhdoo instance and split the steering surface. Guardrails that keep it from being a mess: site/ owns its package.json and tooling; the Go module never references it; test.yml unchanged; prefer a deploy path that adds zero .github/workflows files (e.g. Vercel/Netlify watching the repo) — any workflow file triggers the eyes-on-review law.

Deliberately UNDECIDED, reserved for the unpark grill: framework (Next is Brandon's team-habit default — challenge it against Astro/Starlight/static for a docs+marketing site), hosting/deploy, domain, docs-content strategy (relationship to docs/design/* and the workflow-recipes task, whose likely eventual home is this site), and whether marketing and docs are one site or two.

History: captured during 2026-07-31 intake session; needs its grill before any scoping or code.

## History

_No activity yet._
