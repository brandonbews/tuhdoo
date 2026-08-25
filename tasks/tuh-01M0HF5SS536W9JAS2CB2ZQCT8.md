# Autodeploy hosts and the data branch: joining.md guidance + Vercel recipe page + init pointer

`tuh-01M0HF5SS536W9JAS2CB2ZQCT8`

- **Status:** done
- **Priority:** 1
- **Labels:** `docs` `adoption-friction`
- **Created:** 2026-08-21 06:12 UTC by `brandon/claude-code-1`

## Description

Context (real adopter report; evidence gathered live 2026-08-20/21; scoped to docs-only at the 2026-08-21 grill, Brandon): Brandon ran tuhdoo in a second repo whose Vercel project used joining.md's documented mitigation (Ignored Build Step). Every daemon push to the data branch still produced a blocked-deployment warning. Verified mechanism: Vercel's commit-author authorization check runs BEFORE the Ignored Build Step, so the documented rule never executes; the daemon commits as daemon@tuhdoo.invalid (internal/daemon/daemon.go:69), which can never be a Vercel team member, so the check fails on every ledger push forever. FALSIFIED live: vercel.json with git.deploymentEnabled {"tuhdoo": false} on the DEFAULT branch does nothing — Vercel reads vercel.json from the pushed commit, and the orphan branch carries none. VERIFIED FIX (live, 2026-08-21): commit that vercel.json ONTO THE DATA BRANCH itself — one ordinary fast-forward hand commit at the branch root; blocked deploys stopped. The file survives daemon operation by construction: appends overlay changed paths onto the existing HEAD tree (internal/store/store.go:149-166) and the syncer merge unions both trees with per-area rules only for events//leases/ (internal/syncer/merge.go:47-92). Monorepo projects with a Root Directory (like this repo's site/) are naturally immune — change-detection skips at the webhook stage, before the author check.

Grill outcome for the capture's design half (settled, don't reopen here): docs + an init-output pointer ARE the mechanism. (a) init-seeding of host-hygiene files onto the data branch: DROPPED — revisit only on fresh adopter evidence that the manual commit is a real stumbling block. (b) configurable daemon commit identity: split to held capture tuh-01M0K9B230MNDYYH8D8XJS1DJR (note: making the author check pass would multiply Vercel deploy attempts, not eliminate them — identity is not the Vercel fix).

The ask:
1. Rewrite docs/joining.md's hosted-preview-builder section (~lines 136-154) around the verified mechanism: general guidance that autodeploying hosts may need the data branch silenced; the author-check-precedes-Ignored-Build-Step ordering stated plainly (so that mitigation alone is insufficient wherever the author check applies — team plans / private repos); default-branch git.deploymentEnabled explicitly called out as ineffective (do not recommend it); the data-branch hand commit named as the sanctioned exception to "only daemons write this branch" (one ordinary commit, never force-push) — spell the exception out or reframe that guidance; link to the recipe page.
2. New docs/recipes/vercel.md: the proven walkthrough — vercel.json with git.deploymentEnabled {"<data-branch>": false} committed on the data branch, why it survives (append overlay + union merge), Root Directory immunity. Follows recipes/README.md framing (recommendations, never protocol) and docs/ conventions (GFM, title+description frontmatter, relative links).
3. tuhdoo init success output: add a pointer line ("if your host autodeploys this repo, see <doc>") — cmd/tuhdoo/commands.go runInit's success print (~line 85).
4. Verify whether Netlify / Cloudflare Pages have analogous author checks against vendor docs before claiming any parity; anything not tested live or confirmed against vendor docs is labeled as such.

Acceptance: joining.md reflects only verified mechanisms with the pipeline ordering stated plainly and the data-branch exception addressed explicitly; recipe page exists and is linked; init output carries the pointer; every claim labeled verified-live / vendor-docs / unverified; `make test lint` green; one PR.

Pointers: docs/joining.md:125-155; docs/recipes/README.md; cmd/tuhdoo/commands.go:85; internal/store/store.go:149-166 and internal/syncer/merge.go:47-92 (survival mechanism); Vercel refs: https://vercel.com/docs/project-configuration/git-configuration (git.deploymentEnabled), https://vercel.com/docs/deployments/troubleshoot-project-collaboration (author check).

## History

### 2026-08-21 06:14 UTC — edit by `brandon/claude-code-1`

retitled · description edited

### 2026-08-21 06:44 UTC — edit by `brandon/claude-code-1`

retitled · description edited

### 2026-08-21 23:09 UTC — edit by `brandon/claude-code-1`

retitled · description edited · status inbox→open · priority none→2 · labels −design

### 2026-08-21 23:59 UTC — edit by `brandon`

priority 2→1

### 2026-08-25 19:15 UTC — run by `brandon/claude-code-2` — done

- Branch: `tuh-qct8/autodeploy-data-branch`
- PR: <https://github.com/brandonbews/tuhdoo/pull/84>
- Merged as: `c21e27c661e5aa89f303f3f38e87d6cd9d4068f0`

All four asks landed in one PR (#84, squash c21e27c). joining.md's preview-builder bullet rewritten around the verified mechanism: author-check-precedes-Ignored-Build-Step stated plainly, default-branch git.deploymentEnabled called out as ineffective, the data-branch hand commit named as the sanctioned exception (bullet 1 reworded to match), recipe linked. New docs/recipes/vercel.md carries the walkthrough; its command block was dry-run in a scratch repo and deliberately never writes local refs/heads/tuhdoo (detached worktree from FETCH_HEAD, push to HEAD:refs/heads/tuhdoo) so it cannot race the daemon's CAS loop. Item 4 verification corrected an existing docs error: Netlify does NOT deploy every branch by default (branch deploys are opt-in, per its docs; its private-repo analogue is "Pending approval" deploy requests for unrecognized authors), Cloudflare Pages does build all non-production branches by default with dashboard exclusion; both labeled "Not tested live" with vendor-doc links. init's Auto-deploys stanza points at https://tuhdoo.com/docs/joining (host-neutral; the recipe URL would be wrong for non-Vercel hosts), asserted in cli_test.go. site/src/lib/nav.ts gained the recipe entry (nav is manual; required to publish). make test lint green; no workflow files touched. Note for successors: no vercel.json was committed to THIS repo's data branch — this repo's site deploys via Root Directory and is immune; the recipe is for adopter repos.
