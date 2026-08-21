# Vercel adopter report: Ignored Build Step is insufficient — commit-author check blocks first; verified fix is vercel.json git.deploymentEnabled

`tuh-01M0HF5SS536W9JAS2CB2ZQCT8`

- **Status:** inbox — untriaged capture
- **Priority:** 0
- **Labels:** `docs` `adoption-friction` `design`
- **Created:** 2026-08-21 06:12 UTC by `brandon/claude-code-1`

## Description

## Context (real adopter report, evidence verified live 2026-08-20)

Brandon ran tuhdoo in a second repo whose Vercel project follows `docs/joining.md`'s documented mitigation (Ignored Build Step: `if [ "$VERCEL_GIT_COMMIT_REF" = "tuhdoo" ]; then exit 0; else exit 1; fi`). It did not help: every daemon push to the `tuhdoo` branch still produced a **blocked-deployment warning**.

Root cause (mechanism, confirmed against Vercel docs + live behavior):

- Vercel's pipeline on a push is roughly: webhook → deployment created → **commit-author authorization check** → build starts → **Ignored Build Step command runs**. The author check precedes the Ignored Build Step, so the documented rule never executes.
- The daemon commits as `daemon@tuhdoo.invalid` (hardcoded default, `internal/daemon/daemon.go:69`). `.invalid` can never receive verification mail, so that author can never be a Vercel account/team member — the check fails on every ledger push, forever.
- Why the tuhdoo repo's own Vercel project never showed this: its Root Directory is `site/`, and Vercel's monorepo change-detection skips the push at the **webhook** stage (before deployment creation, so before the author check). The orphan data branch contains no `site/`, so every ledger push is "no relevant changes." Root-directory projects are naturally immune; root-of-repo projects are not.

**Verified fix** (live on the second repo, 2026-08-20 — blocked-deploy warnings stopped): `vercel.json` on the **default branch** with

```json
{ "git": { "deploymentEnabled": { "tuhdoo": false } } }
```

This stops deployment creation at the webhook stage. Empirically it works even though the pushed orphan branch carries no `vercel.json` (there were conflicting community reports on which branch the file is read from; the live test settles it for this shape).

## The ask

Two separable pieces; triage may split them:

1. **Docs (mechanical, ready once triaged):** update `docs/joining.md` (~lines 136–154) so the hosted-preview-builder section leads with `vercel.json` `git.deploymentEnabled` as the primary Vercel mitigation (repo-side, verified), demotes the Ignored Build Step to a footnote with an honest note that the commit-author check fires before it on team/private-repo setups, and mentions that monorepo projects with a Root Directory are naturally immune. Check whether `tuhdoo init` output mentions the hazard and needs the same correction. Netlify/Cloudflare Pages guidance: verify whether they have analogous author checks before claiming the doc's advice suffices there.
2. **Design question (needs grilling, not a quick patch — D7/identity territory):** should the daemon's commit identity be configurable (e.g. default to the repo's `git config user.email` instead of `daemon@tuhdoo.invalid`)? The plumbing exists (`daemon.Options.Ident`) but no CLI/config path wires it. This would make host author-checks pass generally, beyond Vercel. Touches the identity model — route through a /grill-me cycle before deciding.

## Acceptance (for the docs half)

- `docs/joining.md` reflects the verified mechanism and fix above, with the pipeline-ordering explanation (author check before Ignored Build Step) stated plainly.
- No claim in the section is unverified: anything not tested live is labeled as such.
- `make test lint` green; one PR per the repo convention.

## Pointers

- `docs/joining.md:125-155` — current hosted-builder section
- `internal/daemon/daemon.go:66-74` — hardcoded ident + unwired `Options.Ident`
- `internal-docs/design/001-core-design.md:54` — D1 re-affirmation note: revisit on real adopter reports captured fresh with evidence (this task is that capture); prior verified facts on cancelled task tuh-01KZPW8CZPKF2KTWMA5EAVHQ7F
- Vercel refs: https://vercel.com/docs/project-configuration/git-configuration (git.deploymentEnabled), https://vercel.com/docs/deployments/troubleshoot-project-collaboration (author check)

## Constraints

- Piece 2 must not be implemented from this capture; it needs a design-doc revision first (grilling convention).
- Never commit anything to the data branch by hand.

## History

_No activity yet._
