# Site visual identity: Brandon's logo, full pass (landing, docs chrome, favicon, og)

`tuh-01KZF973FY9JKJV5F38SM7BAN7`

- **Status:** open — in progress, claimed by `brandon/claude-code-1`
- **Priority:** 1
- **Labels:** `web` `design` `launch`
- **Depends on:** [`tuh-wyqn`](tuh-01KZF992N0AM2TZGHMH2T5WYQN.md) (done), [`tuh-7sqc`](tuh-01KZPPRW1EKBTVKNR76H6Z7SQC.md) (done)
- **Created:** 2026-08-07 23:33 UTC by `brandon`

## Description

Context: promoted at the 2026-08-10 launch-polish grill (Brandon) from the capture "improve design of docs site (especially logo)". Scope settled as a full identity pass: today's site has no logo, no favicon, no public/ directory, and an agent-improvised generic-clean look. The identity derives from Brandon's logo — a clean Sora "tuhd" wordmark followed by a glassy, frosted, noisy, rich-gradient 3D mark of two lime/emerald cells dividing (the "oo"; fleet multiplication) — delivered as final assets by the human-led export task this depends on.

Direction settled at the grill: the logo carries the richness, the site stays quiet around it. Restrained surfaces, one accent family derived from the mark's lime/emerald, Sora for headings to match the wordmark, light AND dark themes via prefers-color-scheme.

The ask: (1) integrate the delivered assets — wordmark+mark in the nav, favicon, og-image (build the og composition from the mark; every page gets correct metadata); (2) derive design tokens from the mark (accent scale, neutrals) and encode them as the site's Tailwind theme; (3) apply the identity across the landing page and docs chrome (sidebar, article typography, prev/next); (4) keep long-form docs readability the first constraint — the restrained+accent decision exists to protect it.

Acceptance: logo renders in nav at both themes; favicon and og-image present and correct (verify og with a real unfurl check); both themes pass WCAG AA contrast for text; docs pages remain comfortably readable (measure: line length, contrast, heading hierarchy); Brandon approves the result visually — escalate with the Vercel PR-preview URL and hold for his eyes before merging; site builds green through the toolchain gate.

Pointers: assets land via tuh-01KZPPRW1EKBTVKNR76H6Z7SQC (blocking); Tailwind arrives via tuh-01KZF992N0AM2TZGHMH2T5WYQN (blocking — restyle in Tailwind, not twice); component-discipline decision on tuh-01KYX4Y0GZCJTQFNGPP6WMHDM2 (small explicit component inventory, GFM→component map legible in one file); the landing page's SECTION STRUCTURE comes from the messaging brief on the copy task tuh-01KZF97PATRZ1TFWA7CQQCJHQX (pain-led hero, organism beat, maker block, human-loop section) — design to that structure, don't invent one.

Constraints: no glassy panels, gradient washes, or 3D effects anywhere except the mark itself — restrained+accent was the settled answer over glassy-dark-first; ideally the docs copy pass (tuh-01KZF97PATRZ1TFWA7CQQCJHQX) lands first so design works against final copy, but that is soft ordering, not a dependency.

## History

### 2026-08-11 03:16 UTC — escalation from `brandon/claude-code-1` (blocking)

> Review PR #64 (https://github.com/brandonbews/tuhdoo/pull/64) — the site visual-identity pass. Vercel preview: https://tuhdoo-k0l3w44ey-bews-prod.vercel.app (behind your Vercel SSO; the PR page also links it). Approve visually and merge (squash), or leave change requests and answer here. Two derivation judgment calls specifically need your sign-off: (1) the light-mode mark and (2) the flat favicon.

The pass is complete and green (make test lint, incl. site build/Biome/tsc; all PR checks pass). What shipped: your lockup inlined in the nav (wordmark rides currentColor, mark gradients theme-switch via CSS); tokens derived from the mark's delivered values — dark theme seeded from #060806, light accent #0a7a44; Sora (next/font, self-hosted) for headings, system stack for body; docs retuned to 17px/42rem (~65ch); og-image 1200x630 (dark composition, lockup + tagline "A coordination fabric for agent fleets — on a git branch inside the repo it plans." — new copy in an image, eyeball it); apple-touch-icon 180px glassy-on-#060806; full og/twitter metadata on every route, verified against the production build (preview SSO blocks external unfurl checkers; re-verify public unfurl after tuhdoo.com deploys).

The two judgment calls, both easily swappable in site/src/components/logo.tsx + globals.css:
(1) LIGHT-MODE MARK — no light variant was delivered, and the delivered art washes out on white (halo/rim/speculars vanish). Shipped: a light-tuned gradient — same geometry/filters/noise, stops deepened to #d9ffe9/#5df29e/#17b465/#0a7a44 — reads as the same object under daylight. Rejected alternative: dark chip behind the mark (preserves the art verbatim but drops a heavy dark badge into a quiet light header). Check the preview header in light mode.
(2) FAVICON — the glassy mark is an unreadable smudge at 16px, so per the asset task's fallback clause the favicon (icon.svg + favicon.ico) is a FLAT derivation: identical dividing-cells silhouette, no filters, simple vertical gradient from the delivered stops. Check the browser tab on the preview. The 180px apple-icon keeps the full glassy art.

WCAG AA verified for all text pairs both themes (ratios documented in the globals.css header). One token deviates from a delivered value: light link green is #0a7a44 rather than fleck #0c8a4d, which lands at 4.41:1 on white — just under AA.

Options: (a) looks right → approve and squash-merge PR #64 yourself, or answer "merge it" and the next claimant merges and finishes (confirm_claim gate applies); (b) either derivation call or anything else needs work → leave PR comments and answer "address comments". Recommendation: (a) — the derivations follow your delivered values and the evidence sheets back both calls.

_Unanswered._
