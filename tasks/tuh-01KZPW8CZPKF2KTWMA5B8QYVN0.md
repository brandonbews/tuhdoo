# Adoption docs and `tuhdoo init` miss hosted preview builders (Vercel, Netlify, Cloudflare Pages)

`tuh-01KZPW8CZPKF2KTWMA5B8QYVN0`

- **Status:** inbox — untriaged capture
- **Priority:** 0
- **Labels:** `docs` `go` `adoption`
- **Created:** 2026-08-10 22:21 UTC by `brandon/claude-code-2`

## Description

Captured 2026-08-10 from a live hit: Vercel was building failing preview deployments off the `tuhdoo` data branch of this repo.

Context: the existing adopter guidance assumes CI config lives in the repo. `docs/joining.md:117-133` ("For the repo admin: branch protection and CI") and the block `tuhdoo init` prints (`cmd/tuhdoo/commands.go:93`) both cover exactly two things: exempt the data branch from PR/review rulesets, and `on: { push: { branches-ignore: ["tuhdoo"] } }` for GitHub Actions. `docs/adopting.md:32` and `docs/recipes/trunk-based-pr-flow.md:108` point at that same section.

The gap: hosted preview builders — Vercel, Netlify, Cloudflare Pages — deploy *every* branch by default and are configured in their own dashboard, not from a file in the repo. Nothing in tuhdoo's docs or init output warns about them.

Why this matters more than it looks: the daemon pushes on every ledger event (claim, note, finish), so an adopter running agents gets a failing preview deployment every few minutes, with emails, in their first hour. It is a first-run-experience hazard, not a papercut. It also burns free-tier deployment quota.

Verified against Vercel's docs 2026-08-10, the working fix for Vercel is Project → Settings → Git → Ignored Build Step → Custom:

    if [ "$VERCEL_GIT_COMMIT_REF" = "tuhdoo" ]; then exit 0; else exit 1; fi

Note the inverted exit codes: 0 skips the build, 1 proceeds. Requires "Automatically expose System Environment Variables" (on by default) for `VERCEL_GIT_COMMIT_REF`.

Deliberately NOT the fix: `vercel.json`'s `git.deploymentEnabled: { "tuhdoo": false }`. Vercel's docs do not state which branch's `vercel.json` is read for that decision, and the likely answer is the pushed commit's — which on an orphan data branch has no `vercel.json` at all. The Ignored Build Step is a project-level setting on Vercel's side, so it applies regardless of branch contents. Whoever claims this should confirm rather than inherit that reasoning.

At promotion, decide (these are the real open questions, not implementation detail):
- Netlify and Cloudflare Pages equivalents — both need checking; neither has been verified. Does the doc name three platforms with exact steps, or state the general shape ("any host that previews every branch needs a dashboard-side exclusion") plus one worked example? Naming platforms means a maintenance burden as their UIs drift.
- Does this go in the printed `init` block, the docs, or both? The init block is already long, and a wall of per-vendor instructions for platforms the adopter may not use is its own kind of noise.
- Does the branch name need interpolating (`branchName()`) the way the existing lines do, since the data branch name is configurable?

Constraints: `docs/` is published content — GFM, frontmatter limited to title + description, relative links, GitHub rendering as the semantic baseline. Host-agnosticism (T2) is about the binary never calling a host API; printing vendor-specific *guidance* does not violate it, but keep the framing host-agnostic rather than GitHub/Vercel-centric. If `init` output changes, `cmd/tuhdoo/cli_test.go:219` asserts on it.

Related: tuh-01KZF1DNJ3T77A01NJXHW4QGAW (generated data-branch README carries a wrong sentence for adopters — adjacent adopter-facing-wording cleanup, different mechanism). The deeper structural alternative is the sibling capture on moving the ledger out of `refs/heads/`, which if it ever lands would make most of this doc guidance unnecessary.

## History

_No activity yet._
