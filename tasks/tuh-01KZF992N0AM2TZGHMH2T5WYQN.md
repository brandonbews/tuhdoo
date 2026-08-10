# Site toolchain: current create-next-app baseline — Tailwind v4, Biome, full CI gate

`tuh-01KZF992N0AM2TZGHMH2T5WYQN`

- **Status:** open — ready
- **Priority:** 1
- **Labels:** `web` `tooling` `launch`
- **Created:** 2026-08-07 23:35 UTC by `brandon`

## Description

Context: promoted at the 2026-08-10 launch-polish grill (Brandon) from the capture "make sure doc site is using all of the best latest next dependencies and recommendations". The audit finding that reframed it: site/ (Next 16.3, React 19.2, thin unified/remark pipeline) has current dependencies — the gap is tooling discipline, not version rot. There is no linter or formatter at all (no ESLint/Biome/Prettier, no lint script), no typecheck gate, and the site sits entirely outside the repo quality gate: root make test lint is Go-only and the required PR check is the Go test job, so a PR that breaks the site build merges green today (only the non-required Vercel check complains).

The ask, in one PR (or two if the workflow change is cleaner separate):
1. Adopt Tailwind v4. Migrate the existing globals.css styling into Tailwind faithfully — behavior- and appearance-preserving, no redesign; the identity task (tuh-01KZF973FY9JKJV5F38SM7BAN7) does the restyle afterward, in Tailwind. Grill decision of record: Brandon chose Tailwind over keeping plain CSS.
2. Adopt Biome for lint + format, plus tsc --noEmit as the typecheck; package.json gains real lint / format / typecheck scripts.
3. Full quality gate: add a site job to .github/workflows/test.yml (npm ci, biome check, tsc --noEmit, next build) and make it a required status check; extend root make lint and make test to also run the site equivalents so the repo-wide definition of done is honest again. If making the check required needs ruleset access the session token doesn't have, escalate to Brandon with the exact setting to flip.
4. While in there, audit against a fresh create-next-app scaffold and adopt any other current-baseline conventions that fit (tsconfig strictness, next.config shape) — the settled content contract (GFM-never-MDX, thin explicit component inventory) is not up for revision.

Acceptance: biome check, tsc --noEmit, and next build all green from site/; make test lint from the repo root runs them; the site job is a required check on PRs; the deployed site is visually unchanged (spot-check the landing page and one docs page against production www.tuhdoo.com); the .github/workflows diff is called out separately for Brandon's eyes-on review (repo law — never folded silently into a larger change).

Pointers: site/AGENTS.md → bundled Next 16 docs in node_modules (Next 16 idioms postdate model training data — read before writing code); strategy-grill decisions recorded on tuh-01KYX4Y0GZCJTQFNGPP6WMHDM2; Vercel project root is site/, no deploy config changes needed.

Constraints: no new .github/workflows files (extend test.yml only); GFM-never-MDX stands; keep the component inventory small and explicitly listed.

## History

_No activity yet._
