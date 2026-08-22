# Vercel project + tuhdoo.com DNS wiring (human-led)

`tuh-01KZF2D3MA0P24WKAWK89J0Q0X`

- **Status:** done
- **Priority:** none
- **Labels:** `web` `launch`
- **Depends on:** [`tuh-hdm2`](tuh-01KYX4Y0GZCJTQFNGPP6WMHDM2.md) (done)
- **Created:** 2026-08-07 21:34 UTC by `brandon/claude-code-1`

## Description

HUMAN-LED: this needs Brandon's Vercel and Namecheap accounts. Agents must not work this task — if you are an agent holding this claim, release it with reason "human-led account access".

Context: settled at the 2026-08-07 strategy grill (items 2 and 3 of tuh-01KZEPBEE8HFDQVK96AQNCQF4G). Hosting is Vercel via its GitHub App with project root directory site/, zero .github/workflows files. tuhdoo.com is already Brandon's — registered 2026-03-26 at Namecheap, parked on default nameservers, expires 2027-03-26.

The ask, once the site task (tuh-01KYX4Y0GZCJTQFNGPP6WMHDM2) has landed a buildable site/: create the Vercel project watching this repo (root directory site/, framework preset Next.js); verify a production deploy from main and a preview deploy on a PR; add tuhdoo.com (and www redirect) to the project; point Namecheap DNS at Vercel per Vercel's instructions (A/ALIAS + CNAME, or delegate nameservers); confirm HTTPS issues cleanly.

Acceptance: https://tuhdoo.com serves the production site from main; PR preview deploys work; no .github/workflows files were added; renewal reminder confirmed active on the Namecheap side (expiry 2027-03-26).

Pointers: hosting decision recorded on the site task; Vercel monorepo root-directory docs.

Completed 2026-08-07 by Brandon (Vercel + Namecheap accounts). Verified: https://tuhdoo.com 308s to https://www.tuhdoo.com which serves the production site over clean HTTPS from Vercel (www-canonical, apex redirects); Vercel preview-deploy status green on PR #59's head commit; .github/workflows still contains only the pre-existing release.yml and test.yml. Namecheap renewal reminder asserted by Brandon's report, not independently verified.

## History

### 2026-08-08 01:46 UTC — edit by `brandon`

description edited · status open→done
