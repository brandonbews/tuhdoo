# Production-readiness sweep: the last full check before launch

`tuh-01KZFGZD3XWZ8V56P5RKV3R2AC`

- **Status:** open — blocked on dependencies
- **Priority:** 1
- **Labels:** `web` `launch`
- **Depends on:** [`tuh-wyqn`](tuh-01KZF992N0AM2TZGHMH2T5WYQN.md) (done), [`tuh-ban7`](tuh-01KZF973FY9JKJV5F38SM7BAN7.md) (open), [`tuh-jhqx`](tuh-01KZF97PATRZ1TFWA7CQQCJHQX.md) (open)
- **Created:** 2026-08-08 01:49 UTC by `brandon`

## Description

Context: promoted at the 2026-08-10 launch-polish grill (Brandon) from the capture "production ready sweep for accessibility, meta tags, etc". Deliberately the LAST build child of the launch epic — it depends on all three site builds (toolchain, identity, copy) and audits the finished site, not the work in progress. Brandon's framing: "a full check for all the things you would want" on a public production site.

The ask: audit www.tuhdoo.com (and the repo behind it) against a production-readiness checklist, fix what's fixable, and produce the checklist as the run's record. Cover at least:
- Icons and install surface: full favicon set, apple-touch-icon (iOS home-screen), web manifest with icons/name/theme-color.
- Search and social meta: per-page title + description, canonical URLs, Open Graph + Twitter card tags with the og-image, sitemap.xml, robots.txt.
- Accessibility basics: alt text on every image, landmark structure, heading order, skip link, visible focus states, keyboard-only navigation of the docs sidebar, WCAG AA contrast in both themes (the identity task establishes this; verify it survived).
- Agent/machine readability: llms.txt (and llms-full.txt if the docs warrant it) pointing at the content; confirm the published pages degrade sanely without JS (they are statically generated — verify); consider serving the raw .md alongside rendered pages if cheap under the content contract.
- Hygiene: a real 404 page, no dead links (crawl the deployed site), no console errors, Lighthouse pass on landing + one docs page with anything below "good" triaged.
- Anything else a production checklist would flag — the list above is a floor, not a ceiling; "etc etc" is part of the ask.

Acceptance: the completed checklist (item → pass/fixed/deliberately-skipped-with-reason) is recorded in the run note or PR body; every fix is landed and verified against the deployed site; anything requiring Brandon's judgment (e.g. what llms.txt should say about the project, manifest naming) escalated rather than guessed; Brandon reviews the final checklist — this sweep is his launch confidence, so the record must be honest about what was NOT checked.

Pointers: identity task tuh-01KZF973FY9JKJV5F38SM7BAN7 delivers favicon/og baseline — this task verifies and extends (apple-touch-icon, manifest are here, not there); content contract on tuh-01KYX4Y0GZCJTQFNGPP6WMHDM2; site is statically generated, no dynamic APIs.

Constraints: no new .github/workflows files; fixes follow the toolchain gate (Biome, tsc, site CI job) like any other site PR.

## History

_No activity yet._
