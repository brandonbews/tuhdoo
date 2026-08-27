---
title: "Recipe: Vercel and the data branch"
description: "How to stop Vercel from attempting a deployment on every push of tuhdoo's data branch: one vercel.json committed onto the data branch itself, and why that is the placement that works."
---

# Recipe: Vercel and the data branch

This recipe is for anyone running tuhdoo in a repo that is also connected to a Vercel project. Every daemon push of the data branch — the git orphan branch that carries tuhdoo's ledger (its append-only record of tasks and activity) — gives Vercel something to deploy, and on a private repo every one of those pushes produces a blocked deployment and a warning, forever. The fix is one small file committed once, in a place you might not guess: onto the data branch itself. This page is the walkthrough, plus enough of the mechanism to trust it. It is a recommendation, not protocol (see the [recipes overview](README.md) for the boundary); other hosts are covered in the repo-admin section of [`joining.md`](../joining.md#for-the-repo-admin-branch-protection-and-ci).

Everything here was verified live against a real adopter repo on 2026-08-21, except where a claim is labeled as resting on Vercel's docs.

## What happens without it

Vercel deploys automatically on every branch push ([Vercel's docs](https://vercel.com/docs/git)), and the daemon pushes the data branch on every sync that carries news — after a burst of agent activity, that is often. On a private repo, Vercel first checks that the commit author is authorized: the owner for a Hobby team, a team member for a Pro team ([Vercel's docs](https://vercel.com/docs/deployments/troubleshoot-project-collaboration); public repos are exempt from this check). The ledger's commits are authored by `tuhdoo daemon <daemon@tuhdoo.invalid>` — deliberately not a person — so the check fails on every push, and each failure surfaces as a blocked deployment with a warning.

## Two fixes that do not work

- **An Ignored Build Step.** The dashboard rule that skips builds for a named branch never gets its chance here: the commit-author check runs *before* the Ignored Build Step, so for the daemon's pushes the rule simply never executes (verified live: the warnings continued with the rule in place). On a public repo, where no author check applies, an ignore rule does work — but the fix below stops the deployments from being triggered at all, and works in both cases.
- **`vercel.json` on your default branch.** Vercel takes project configuration from the commit being deployed, and the data branch is an orphan branch carrying only the ledger — your default branch's files do not apply to its pushes. Tried live: a `git.deploymentEnabled` rule on the default branch changed nothing.

## The fix that works

Commit a `vercel.json` at the root of the data branch, disabling deployments for that branch:

```json
{
  "$schema": "https://openapi.vercel.sh/vercel.json",
  "git": { "deploymentEnabled": { "tuhdoo": false } }
}
```

`git.deploymentEnabled` is Vercel's own switch for branches that should not trigger a deployment on commit ([Vercel's docs](https://vercel.com/docs/project-configuration/git-configuration#git.deploymentenabled)). If your data branch is not named `tuhdoo`, key the object by its actual name. Committed onto the data branch, the configuration travels with every push of that branch, so Vercel sees it every time. Verified live: the blocked deployments stopped as soon as this commit landed, and stayed stopped.

### Committing it

The data branch is never checked out, and your local copy of it belongs to the daemon. So make the commit against the *remote's* tip in a throwaway worktree, and push it straight back — the remote's fast-forward check is the arbiter if a daemon pushes at the same moment:

```sh
git fetch origin tuhdoo
git worktree add --detach ../data-branch FETCH_HEAD
cd ../data-branch
cat > vercel.json <<'EOF'
{
  "$schema": "https://openapi.vercel.sh/vercel.json",
  "git": { "deploymentEnabled": { "tuhdoo": false } }
}
EOF
git add vercel.json
git commit -m "vercel: never deploy the data branch"
git push origin HEAD:refs/heads/tuhdoo
cd -
git worktree remove ../data-branch
```

If the push is rejected because a daemon got there first, nothing is lost: fetch again and repeat — **never force-push**. There is nothing to restart afterwards; every machine's daemon absorbs the new commit through its ordinary sync merge. Any route that produces one ordinary commit on the branch is equivalent — on GitHub, "Add file" on the data branch through the web UI does the same job.

## Isn't the data branch daemon-only?

Yes — and this is the sanctioned exception. The rule that humans never write the data branch exists to protect the ledger's invariants: events are never rewritten, history moves fast-forward only, nothing is ever force-pushed. A hand commit that *adds a host-hygiene file at the root* — one ordinary commit, touching nothing under `events/` or `leases/`, never forced — breaks none of them. That is the entire licence: add a file like this one, and leave everything else on the branch to the daemons.

## Why the file survives

Daemon writes are additive by construction. An append takes the current tip's tree and overlays only the paths it is writing; a sync merge unions the two sides' trees, with special handling only for the areas tuhdoo owns (`events/`, `leases/`, and its generated views — the browsable markdown pages it renders from the ledger). A root file the daemons know nothing about rides along untouched through both. And since the branch is never force-pushed or rewritten — a founding rule of the tool — nothing ever rebuilds the tree out from under it.

## If your project sets a Root Directory

Monorepo projects that deploy a subdirectory (a Root Directory in Vercel's project settings) never see any of this: Vercel skips pushes that do not touch that directory up front, and the data branch never touches it. Verified live: tuhdoo's own repository deploys its site this way, and its ledger pushes produce no deployment attempts.
