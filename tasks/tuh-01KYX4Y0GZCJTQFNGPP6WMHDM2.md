# Marketing / docs site for tuhdoo (monorepo: site/ in this repo)

`tuh-01KYX4Y0GZCJTQFNGPP6WMHDM2`

- **Status:** open — blocked on dependencies
- **Priority:** 0
- **Labels:** `docs` `product` `web`
- **Depends on:** [`tuh-qf4g`](tuh-01KZEPBEE8HFDQVK96AQNCQF4G.md) (open)
- **Created:** 2026-07-31 22:32 UTC by `brandon/claude-code-1`

## Description

Child of the launch epic (tuh-01KZEPBEE8HFDQVK96AV6RN0ER). Blocked on the strategy grill (tuh-01KZEPBEE8HFDQVK96AQNCQF4G), which owns every decision this capture had reserved: framework, hosting/deploy, domain, one-site-or-two, docs-content strategy. The original prose gate ("v0 dogfood week held + PR flow exercised") was satisfied and replaced by that dependency edge at the 2026-08-07 launch-epic structuring.

DECIDED at capture (2026-07-31 intake) — the grill inherits this, doesn't re-litigate: the site lives IN THIS REPO as a self-contained site/ directory — monorepo — because tuhdoo's ledger is repo-scoped: site work in this repo stays steerable from this backlog, while a separate repo would need its own tuhdoo instance and split the steering surface. Guardrails that keep it from being a mess: site/ owns its package.json and tooling; the Go module never references it; test.yml unchanged; prefer a deploy path that adds zero .github/workflows files (e.g. Vercel/Netlify watching the repo) — any workflow file triggers the eyes-on-review law.

The ask (to be sharpened by the grill's outputs, recorded here when it closes): build the marketing/docs site under site/ per the grill's framework/hosting/content decisions.

Acceptance (provisional until the grill records its decisions here): site/ builds and deploys per the chosen path; the Go side of the repo is untouched (make test lint green, no Go-module reference to site/); no new .github/workflows files without Brandon's eyes-on review.

Cross-links: the site/ work is the designated first live test of the monorepo-grain question (tuh-01KZA0VT234XJYVZWT8YFV8XE2, held) — note steering-friction signals as they appear. User-facing docs content is owned by tuh-01KZ9YD2Q9V7BSZJBDGVNYM6QY; workflow-recipes content by tuh-01KYX3KJYP7Y178GH5DJ6JPMRQ.

History: captured 2026-07-31 intake; held behind v1-confidence gate; gate declared satisfied and task restructured into the launch epic 2026-08-07.

## History

_No activity yet._
