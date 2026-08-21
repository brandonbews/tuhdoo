# Vercel adopter report: verified fix is vercel.json on the data branch itself; docs rewrite + init/identity design question

`tuh-01M0HF5SS536W9JAS2CB2ZQCT8`

- **Status:** inbox — untriaged capture
- **Priority:** 0
- **Labels:** `docs` `adoption-friction` `design`
- **Created:** 2026-08-21 06:12 UTC by `brandon/claude-code-1`

## Description

## Context (real adopter report; evidence gathered live 2026-08-20/21)

Brandon ran tuhdoo in a second repo whose Vercel project follows `docs/joining.md`'s documented mitigation (Ignored Build Step: `if [ "$VERCEL_GIT_COMMIT_REF" = "tuhdoo" ]; then exit 0; else exit 1; fi`). It did not help: every daemon push to the `tuhdoo` branch still produced a **blocked-deployment warning**.

Root cause (mechanism, confirmed against Vercel docs + live behavior):

- Vercel's pipeline on a push is roughly: webhook → deployment created → **commit-author authorization check** → build starts → **Ignored Build Step command runs**. The author check precedes the Ignored Build Step, so the documented rule never executes.
- The daemon commits as `daemon@tuhdoo.invalid` (hardcoded default, `internal/daemon/daemon.go:69`). `.invalid` can never receive verification mail, so that author can never be a Vercel account/team member — the check fails on every ledger push, forever.
- Why the tuhdoo repo's own Vercel project never showed this: its Root Directory is `site/`, and Vercel's monorepo change-detection skips the push at the **webhook** stage (before deployment creation, so before the author check). The orphan data branch contains no `site/`, so every ledger push is "no relevant changes." Root-directory projects are naturally immune; root-of-repo projects are not.

**Falsified (tested live 2026-08-20):** `vercel.json` with `{ "git": { "deploymentEnabled": { "tuhdoo": false } } }` on the **default branch** does NOT stop the blocked deploys — Vercel reads `vercel.json` from the **pushed commit**, and the orphan data branch carries none. (An initial "it worked" observation was a false positive; the blocked deploy was merely delayed. Weight later observations accordingly.)

**Verified fix (tested live, confirmed 2026-08-21):** commit that same `vercel.json` **onto the data branch itself** — a single ordinary fast-forward hand commit at the branch root. Blocked deploys stopped. The file survives daemon operation by construction: appends overlay changed paths onto the existing HEAD tree (`internal/store/store.go:149-166`) and the syncer merge is a union of both trees with per-area rules only for `events/`/`leases/` (`internal/syncer/merge.go:47-92`), so a foreign path is carried forward indefinitely. Trade-off: it breaches the documented "only daemons write this branch" posture and puts host-specific hygiene files on every peer's ledger.

## The ask

Pieces, likely split at triage:

1. **Docs fix (now unblocked — a verified workaround exists):** rewrite `docs/joining.md` (~lines 136–154) hosted-preview-builder section around the verified mechanism and fix: author check precedes Ignored Build Step (so the Ignored Build Step alone is insufficient wherever the author check applies — team plans / private repos); the working Vercel mitigation is `vercel.json` with `git.deploymentEnabled: {"<data-branch>": false}` committed **on the data branch** (one ordinary commit, never force-pushed — spell out that this is the sanctioned exception to "don't touch the data branch," or reframe that guidance); `git.deploymentEnabled` on the default branch is confirmed ineffective (do not recommend it); monorepo projects with a Root Directory are naturally immune. Check `tuhdoo init` output for the same correction. Verify whether Netlify/Cloudflare Pages have analogous author checks before claiming parity there. No claim may be unverified: anything not tested live or confirmed against vendor docs is labeled as such.
2. **Design question (needs grilling):** two candidate durable mechanisms, not mutually exclusive: (a) `tuhdoo init` offers to seed host-hygiene files (e.g. this `vercel.json`) on the data branch — now validated by the live fix, makes the workaround first-class instead of a posture breach; (b) configurable daemon commit identity (`daemon.Options.Ident` exists, `internal/daemon/daemon.go:66-74`, but no CLI/config path wires it — e.g. default to repo `git config user.email`), which makes host author-checks pass generally, beyond Vercel. (b) is D7/identity territory. Route through a /grill-me cycle; the D1 re-affirmation note (`internal-docs/design/001-core-design.md:54`) says host friction is onboarding-not-architecture — (a) fits that frame naturally, (b) touches identity design.

## Acceptance (for the docs half)

- `docs/joining.md` reflects only verified mechanisms, with the pipeline-ordering explanation stated plainly and the data-branch commit exception addressed explicitly.
- `make test lint` green; one PR per the repo convention.

## Pointers

- `docs/joining.md:125-155` — current hosted-builder section
- `internal/daemon/daemon.go:66-74` — hardcoded ident + unwired `Options.Ident`
- `internal/store/store.go:149-166`, `internal/syncer/merge.go:47-92` — why a foreign file on the data branch survives appends and merges
- `internal-docs/design/001-core-design.md:54` — D1 re-affirmation note: revisit on real adopter reports captured fresh with evidence (this task is that capture); prior verified facts on cancelled task tuh-01KZPW8CZPKF2KTWMA5EAVHQ7F
- Vercel refs: https://vercel.com/docs/project-configuration/git-configuration (git.deploymentEnabled), https://vercel.com/docs/deployments/troubleshoot-project-collaboration (author check)

## Constraints

- Piece 2 must not be implemented from this capture; it needs a design-doc revision first (grilling convention).
- Nothing lands in docs that hasn't been verified live or against vendor docs, labeled which.

## History

### 2026-08-21 06:14 UTC — edit by `brandon/claude-code-1`

retitled · description edited

### 2026-08-21 06:44 UTC — edit by `brandon/claude-code-1`

retitled · description edited
