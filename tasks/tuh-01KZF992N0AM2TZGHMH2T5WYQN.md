# Site toolchain: current create-next-app baseline — Tailwind v4, Biome, full CI gate

`tuh-01KZF992N0AM2TZGHMH2T5WYQN`

- **Status:** open — in progress, claimed by `brandon/claude-code-1`
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

### 2026-08-10 22:28 UTC — escalation from `brandon/claude-code-1` (blocking)

> Review the .github/workflows/test.yml diff in PR #63 (https://github.com/brandonbews/tuhdoo/pull/63) and squash-merge it — or answer "merge it" here and the next claimant merges. Also confirm you want "site" added as a required status check afterward.

The toolchain work is complete and fully green on the PR: Tailwind v4 substrate (appearance-identical — rule-by-rule CSS diff and byte-identical prerendered routes; Vercel preview on the PR for a visual spot-check), Biome 2.5.7 + tsc with real scripts, site CI job, and root make test/lint now covering the site. It sits unmerged for one reason: the PR changes .github/workflows/test.yml (adds the site job; the existing test job's one line becomes "make test-go lint-go" to keep jobs disjoint), and workflow changes are the repo-law exception needing your eyes-on diff review — the harness's permission layer independently declined my autonomous merge of it, which I honored rather than worked around. Two follow-ups after merge, for me-or-successor unless you'd rather do them: (1) make the site check required — exact flip: ruleset 20155938 ("main: changes land via PR with green test") → required_status_checks → add context "site" alongside "test" (doable via: gh api -X PUT repos/brandonbews/tuhdoo/rulesets/20155938 with the rules array re-sent, or the GitHub UI ruleset editor); do this only AFTER #63 is on main, or every open PR blocks. (2) Refresh PR #62's branch (merge main, run npx biome format --write in site/) — the Biome format pass reformatted page.tsx, which #62's copy rewrite also touches, so an un-refreshed #62 will conflict. Recommendation: review the test.yml diff (it's 19 lines), squash-merge #63, and answer here whether the required-check flip is approved.

**Answer** (`brandon`, relayed by `brandon/claude-code-1`): merge it and go ahead with the required-check flip

### 2026-08-10 22:28 UTC — note from `brandon/claude-code-1`

Resume state: branch tuh-wyqn/site-toolchain, PR #63 open and green (test + site jobs + Vercel preview), no auto-merge — blocked on escalation 01KZPWNK94JP4J1FAE48A5MV7J (workflow-file review is Brandon's). Next claimant after the answer: (1) confirm_claim, merge #63, record squash commit in merged_as; (2) if approved, add "site" to required_status_checks on ruleset 20155938 (only after merge lands); (3) refresh PR #62 branch tuh-jhqx/docs-landing-copy: merge main, run `npx biome format --write` in site/, push — prevents the page.tsx format/copy conflict. No Go changes — no daemon restart needed for this task.

### 2026-08-10 22:29 UTC — run by `brandon/claude-code-1` — blocked

- Branch: `tuh-wyqn/site-toolchain`
- PR: <https://github.com/brandonbews/tuhdoo/pull/63>
- Commits: `7d7bb3a`

Toolchain complete and green on PR #63; blocked on Brandon's workflow-diff review — see escalation 01KZPWNK94JP4J1FAE48A5MV7J.

### 2026-08-10 23:22 UTC — escalation from `brandon/claude-code-1` (blocking)

> One step left, yours alone: add "site" as a required status check. Either GitHub UI (Settings → Rules → Rulesets → "main: changes land via PR with green test" → Require status checks to pass → add "site" alongside "test"), or grant the session PAT "Administration: write" and answer "retry" here. Answer once done so the task unblocks.

You approved this flip (answer relayed 22:58: "merge it and go ahead with the required-check flip") and everything else on the task is complete: PR #63 squash-merged to main as 9e7dbe2, site CI job live and green on both open-PR merge refs, root make test lint covers the site, PR #62's branch refreshed to prevent the format conflict. The session token cannot perform the flip itself — gh api PUT on ruleset 20155938 returns 403 "Resource not accessible by personal access token" (the fine-grained PAT lacks repo Administration write, consistent with your least-privilege posture) — and 20 minutes of polling after the merge still shows only "test" required. This is the final acceptance criterion; when the ruleset shows both "test" and "site", whoever holds the task finishes it done with merged_as 9e7dbe2.

_Unanswered._

### 2026-08-10 23:22 UTC — note from `brandon/claude-code-1`

Resume state: all work merged (PR #63 → main as 9e7dbe2; PR #62 branch refreshed separately under its own task). Sole remaining criterion: "site" required check — blocked on escalation 01KZPZQCXFDEDZPRAMSKVYSPQK. Verify with: gh api repos/brandonbews/tuhdoo/rulesets/20155938 --jq '[.rules[] | select(.type=="required_status_checks") | .parameters.required_status_checks[].context]' — when it returns ["test","site"], finish_run done with merged_as 9e7dbe2. If Brandon instead grants the PAT Administration:write, the ready-to-PUT payload approach: GET the ruleset, add {"context":"site"} to required_status_checks, PUT back. No Go changes anywhere in this task — no daemon restart needed.
