# Adoption docs and `tuhdoo init` miss hosted preview builders (Vercel, Netlify, Cloudflare Pages)

`tuh-01KZPW8CZPKF2KTWMA5B8QYVN0`

- **Status:** open — ready
- **Priority:** 0
- **Labels:** `docs` `go` `adoption`
- **Created:** 2026-08-10 22:21 UTC by `brandon/claude-code-2`

## Description

Context: Vercel was building failing preview deployments off this repo's `tuhdoo` data branch (live hit, 2026-08-10). Hosted preview builders — Vercel, Netlify, Cloudflare Pages — deploy every branch by default and are configured in their own dashboards, not from a file in the repo. The daemon pushes on every ledger event, so an adopter running agents gets a failing deployment every few minutes, with emails, in their first hour — a first-run-experience hazard, not a papercut. Existing adopter guidance (`docs/joining.md:117-133` "For the repo admin", the block `tuhdoo init` prints at `cmd/tuhdoo/commands.go:93`, pointers from `docs/adopting.md:32` and `docs/recipes/trunk-based-pr-flow.md:108`) covers only repo-side config: ruleset exemption and Actions `branches-ignore`. Nothing warns about dashboard-side builders.

The former open questions were settled at the 2026-08-11 grill (Brandon) — do not re-open them:
1. **Shape: general principle + one verified worked example.** State the rule plainly — the data branch is a real branch with all the quirks of a real branch; tuhdoo pushes it often after init, so any host that auto-deploys every branch needs a dashboard-side exclusion for it. Then give the Vercel walkthrough as the single worked example. Do NOT document Netlify/Cloudflare steps — they are unverified and vendor UIs drift; the principle covers them.
2. **Placement: both.** The docs (the existing repo-admin section in `docs/joining.md` is the natural home, with the getting-started/adopting path pointing at it plainly) AND one short line in the `init` output block — e.g. "tuhdoo pushes this branch often — make sure nothing auto-deploys it" — not a wall of vendor steps.
3. **Branch name: interpolate.** Use `branchName()` the way the existing init lines do; the data branch name is configurable.

The verified Vercel fix (checked against Vercel docs 2026-08-10): Project → Settings → Git → Ignored Build Step → Custom:

    if [ "$VERCEL_GIT_COMMIT_REF" = "tuhdoo" ]; then exit 0; else exit 1; fi

Note the inverted exit codes: 0 skips the build, 1 proceeds. Requires "Automatically expose System Environment Variables" (on by default) for `VERCEL_GIT_COMMIT_REF`. Deliberately NOT the fix: `vercel.json` `git.deploymentEnabled` — Vercel's docs don't state which branch's `vercel.json` decides, and the likely answer is the pushed commit's, which on an orphan data branch has no `vercel.json` at all. Present the example with the adopter's actual branch name where the doc context allows.

Acceptance:
- `docs/joining.md` repo-admin section covers hosted preview builders: the general principle, then the verified Vercel example (with the inverted-exit-codes gotcha called out); framing stays host-agnostic, GFM, relative links, frontmatter title+description only.
- `tuhdoo init` output gains one short interpolated line about auto-deploys; `cmd/tuhdoo/cli_test.go:219` asserts on init output — update the assertion to match.
- No Netlify/Cloudflare step-by-steps anywhere.
- `make test lint` green; one PR.

Pointers/background: the structural alternative (ledger outside refs/heads/) was grilled and declined 2026-08-11 — see cancelled tuh-01KZPW8CZPKF2KTWMA5EAVHQ7F for the verified webhook facts and the tripwire; D4 consequence 3 in `internal-docs/design/001-core-design.md` carries the re-affirmation note. Related wording cleanup: tuh-01KZF1DNJ3T77A01NJXHW4QGAW (generated data-branch README sentence).

## History

### 2026-08-11 22:00 UTC — edit by `brandon/claude-code-1`

description edited · status inbox→open
