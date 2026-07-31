# tuh-01KYX1D49M9M0EB69HNVBZT906 — Trunk-based PR flow: squash-only merges, enforced by rulesets, loop rewired in CLAUDE.md

- Status: open — ready
- Priority: 0
- Labels: `process`, `docs`
- Created: 2026-07-31 21:31 UTC by `brandon`

## Description

Context: Decided in the 2026-07-31 grill cycle (this task's original capture assumed a git-flow dev/main split; that was rejected — releases are already tag-driven and branch-independent, see .github/workflows/release.yml). Goal: a mature git history where a ticket becomes a PR, the PR squash-merges to main, and the work for any task is reviewable in isolation long after merge. Review remains optional; the record is the point. This is THIS REPO's process only — docs/agent-protocol.md is product surface and stays workflow-agnostic (its step 3 already says tuhdoo never touches your code workflow); do not edit it.

The ask, in three parts:

1. Repo settings (needs GitHub Administration permission — Brandon will temporarily lift the PAT's permissions on request; raise a blocking escalation when ready to apply, listing the exact gh api calls, and wait for his go):
   - Merge methods: squash ONLY (disable merge commits and rebase merges); default squash message = PR title + description.
   - Enable auto-merge; enable auto-delete of merged branches.
   - Ruleset on main: require a pull request before merging (0 approvals), require the "test" status check to pass, no bypass actors.
   - Ruleset on the tuhdoo data branch: block force pushes and deletion (mechanizes the founding no-force-push law).
2. CLAUDE.md "Building tasks" rewrite — the new loop: claim → branch tuh-<short-id>/<slug> off fresh main → work, commit freely (squash makes intermediates free), make test lint green locally → open PR (title = task title, so it becomes the squash-commit subject; body opens with the tuhdoo task ID and an honest summary) → gh pr merge --auto --squash → wait for the merge to land → finish_run(done) ONLY after merge (if CI fails: fix, or finish_run(blocked); work sitting unmerged on a branch is not done). Keep the deploy-after-landing daemon-restart guidance intact.
3. Folded from tuh-01KYX2JXT30YBT4ZQNEZP3Z7XM (cancelled into this task): verify the full suite gates every merge — test.yml already runs make test lint on pull_request; the required-check ruleset makes it binding.

Acceptance:
- Repo settings and both rulesets live and verified (gh api output in the run summary): a direct push to main is rejected; a PR cannot merge with a red test check; squash is the only merge method; force-push to tuhdoo is rejected (verify with a dry-run/--force-with-lease probe against a throwaway ref or by reading the ruleset, NOT by force-pushing the data branch).
- CLAUDE.md describes the new loop; no edits to docs/agent-protocol.md or .github/workflows/ (test.yml already runs on PRs — if a workflow edit turns out to be needed, stop: project law requires Brandon's eyes-on review, call it out separately).
- The first task landed after this one flows through the new loop end-to-end as proof.
- make test lint green from the repo root.

Pointers: CLAUDE.md (Building tasks section), .github/workflows/test.yml (the "test" check name), .github/workflows/release.yml (tag-only — confirm untouched), gh api docs for rulesets (POST /repos/{owner}/{repo}/rulesets).

Constraints: no dev branch — main stays the only long-lived code branch. Releases stay tag-driven. The tuhdoo data branch is never force-pushed, including to test the rule. Settings changes wait for Brandon's explicit permission lift (escalate first).

## History

_No activity yet._
