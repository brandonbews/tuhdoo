# Site visual identity: Brandon's logo, full pass (landing, docs chrome, favicon, og)

`tuh-01KZF973FY9JKJV5F38SM7BAN7`

- **Status:** open — blocked on dependencies
- **Priority:** 1
- **Labels:** `web` `design` `launch`
- **Depends on:** [`tuh-wyqn`](tuh-01KZF992N0AM2TZGHMH2T5WYQN.md) (open), [`tuh-7sqc`](tuh-01KZPPRW1EKBTVKNR76H6Z7SQC.md) (open)
- **Created:** 2026-08-07 23:33 UTC by `brandon`

## Description

Context: promoted at the 2026-08-10 launch-polish grill (Brandon) from the capture "improve design of docs site (especially logo)". Scope settled as a full identity pass: today's site has no logo, no favicon, no public/ directory, and an agent-improvised generic-clean look. The identity derives from Brandon's logo — a clean Sora "tuhd" wordmark followed by a glassy, frosted, noisy, rich-gradient 3D mark of two lime/emerald cells dividing (the "oo"; fleet multiplication) — delivered as final assets by the human-led export task this depends on.

Direction settled at the grill: the logo carries the richness, the site stays quiet around it. Restrained surfaces, one accent family derived from the mark's lime/emerald, Sora for headings to match the wordmark, light AND dark themes via prefers-color-scheme.

The ask: (1) integrate the delivered assets — wordmark+mark in the nav, favicon, og-image (build the og composition from the mark; every page gets correct metadata); (2) derive design tokens from the mark (accent scale, neutrals) and encode them as the site's Tailwind theme; (3) apply the identity across the landing page and docs chrome (sidebar, article typography, prev/next); (4) keep long-form docs readability the first constraint — the restrained+accent decision exists to protect it.

Acceptance: logo renders in nav at both themes; favicon and og-image present and correct (verify og with a real unfurl check); both themes pass WCAG AA contrast for text; docs pages remain comfortably readable (measure: line length, contrast, heading hierarchy); Brandon approves the result visually — escalate with the Vercel PR-preview URL and hold for his eyes before merging; site builds green through the toolchain gate.

Pointers: assets land via tuh-01KZPPRW1EKBTVKNR76H6Z7SQC (blocking); Tailwind arrives via tuh-01KZF992N0AM2TZGHMH2T5WYQN (blocking — restyle in Tailwind, not twice); component-discipline decision on tuh-01KYX4Y0GZCJTQFNGPP6WMHDM2 (small explicit component inventory, GFM→component map legible in one file).

Constraints: no glassy panels, gradient washes, or 3D effects anywhere except the mark itself — restrained+accent was the settled answer over glassy-dark-first; ideally the docs copy pass (tuh-01KZF97PATRZ1TFWA7CQQCJHQX) lands first so design works against final copy, but that is soft ordering, not a dependency.

## History

_No activity yet._
