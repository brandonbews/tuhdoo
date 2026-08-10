---
title: "Recipe: trunk-based PR flow"
description: One task, one branch, one squash-merged pull request into a protected default branch — the recommended code workflow for an agent fleet working a tuhdoo backlog.
---

# Recipe: trunk-based PR flow

One task, one branch, one squash-merged pull request into a protected
default branch. This is the workflow tuhdoo's own repository is built with —
dozens of agent-authored PRs have landed through it — and the recipe to
start from if your team has no strong prior. It is a recommendation, not
protocol: tuhdoo works identically with any git workflow (see the
[recipes overview](README.md) for the boundary).

It assumes a repo with a single long-lived default branch (`main` below), a
CI check that runs on pull requests, and a host that supports squash merges
and branch protection — GitHub, GitLab, and their peers all qualify.

## Why this shape

Trunk-based flow and agent fleets fit unusually well:

- **One PR per task** gives every ledger entry a single durable artifact.
  The task says what was asked; the PR says what was done; the squash
  commit on the default branch carries both forward. Nothing about the work
  lives only in an agent's session.
- **Squash merges make intermediate commits cost-free.** Agents can commit
  as often as they like — checkpoints, dead ends, reverts — without anyone
  curating history. The default branch records one commit per task, titled
  after the task.
- **Branch protection turns convention into enforcement.** An agent cannot
  be talked out of a rule the host refuses to bypass. Direct pushes
  blocked, a green check required, squash the only merge method: the
  workflow holds even when an individual session goes off-script.

## The flow

Steps 1 and 6–7 are tuhdoo protocol (linked); steps 2–5 are this recipe.

1. **Claim the task.** Agents take work through tuhdoo's `claim_next` (or
   `claim_task` for a specific task), never by just starting. A claim is a
   time-boxed lease the daemon renews automatically while the agent's
   session is connected — it is what stops two agents from building the
   same thing. Protocol: see
   [the loop](../agent-protocol.md#the-loop) in the agent protocol.

2. **Branch off fresh `main`.** Name the branch after the task:

   ```
   tuh-<short-id>/<slug>
   ```

   e.g. `tuh-d83w/retry-on-stale-lease` for task `tuh-…d83w`. Every task ID
   ends in a four-character tail (the *short ID*, shown throughout tuhdoo's
   CLI and TUI); putting it in the branch name lets anyone — human or
   successor agent — trace a branch to its task at a glance, and find an
   interrupted agent's work by searching branches for the short ID. Branch
   from freshly-pulled `main`, not from another task branch: stacked
   branches die badly under squash merges.

3. **Work and commit freely.** Ordinary commits on the task branch, as many
   as useful. Run the repo's tests locally before opening the PR — CI is
   the gate, not the first line of defense.

4. **Open the PR as the durable record.**

   - **Title = task title.** The squash merge reuses the PR title as the
     commit subject, so the default branch's history reads as a list of
     completed tasks.
   - **Body opens with the tuhdoo task ID**, then an honest summary of what
     was done — including what was *not* done, and anything surprising. The
     PR body is what a human reviews and what a future reader finds from
     `git log`; write it for them, not for the merge button.

5. **Enable auto-merge (squash).** Once CI is green the PR lands without a
   human round-trip — e.g. `gh pr merge --auto --squash` on GitHub. If your
   team wants human review before merge, require a review in branch
   protection instead of skipping auto-merge: keep the mechanics automatic
   and put the human gate in the host's rules.

6. **Confirm before the merge lands.** A claim is provisional — on a
   multi-machine team, an earlier claim elsewhere can void it. tuhdoo's
   `confirm_claim` settles ownership irrevocably; agents call it before
   merging and merge only on a confirmed verdict. Protocol:
   [confirm before you merge](../agent-protocol.md#the-loop).

7. **Finish only after the merge lands.** `finish_run(done)` means the
   acceptance criteria hold *on the default branch* — so agents wait for
   the merge, then report the squash commit in `merged_as` (under squash
   merges the branch commits never reach `main`; the squash commit carries
   the work). If CI goes red: fix it, or finish honestly as
   `blocked`/`failed`. Work sitting unmerged on a branch is not done.
   Protocol: [finish honestly](../agent-protocol.md#the-loop).

## Repo settings that enforce it

Set once by the repo admin, on the host:

- **Block direct pushes to the default branch** — changes arrive by pull
  request only.
- **Require the CI check to pass** before merging.
- **Allow squash merge only** — with one method available, agents cannot
  pick the wrong one.
- **Auto-delete merged branches** — task branches are disposable by design;
  deleting them on merge keeps the branch list meaning "work in flight".
- **Exempt the tuhdoo data branch** (`tuhdoo` by default — the branch the
  coordination ledger lives on) from all of the above, and exclude it from
  CI triggers. The daemon pushes it directly, fast-forward only. Details in
  [joining.md](../joining.md#for-the-repo-admin-branch-protection-and-ci).

With these in place, the recipe is self-enforcing: an agent that skips the
PR, the green check, or the squash merge simply cannot land its work.

## Agent instructions block

Paste into your repo's agent-instructions file (`CLAUDE.md`, `AGENTS.md`,
or whatever your harness reads) and adapt the bracketed parts. It assumes
the harness is already connected to tuhdoo and follows the
[agent protocol](../agent-protocol.md).

```markdown
## Git workflow (trunk-based PR flow)

- Claim a task through tuhdoo before working; never start unclaimed work.
- One PR per task. Branch `tuh-<short-id>/<slug>` off freshly-pulled
  [main]; commit freely on the branch — squash-merge makes intermediate
  commits cost-free.
- Run [test command] locally; open the PR only when it is green.
- PR title = task title (it becomes the squash-commit subject). Open the
  body with the tuhdoo task ID and an honest summary — the PR is the
  durable record of the work.
- Enable auto-merge with squash (e.g. `gh pr merge --auto --squash`).
- Call `confirm_claim` before the merge can land; merge only on a
  confirmed verdict.
- `finish_run(done)` only after the merge lands, reporting the squash
  commit in `merged_as`. If CI goes red, fix it or finish as `blocked` —
  work sitting unmerged on a branch is not done.
- Direct pushes to [main] are blocked by branch protection. That is
  enforcement, not convention: don't fight it, don't ask for a bypass.
```

## Adapting it

The load-bearing parts are: one PR per task, the task ID in the branch
name, the PR title/body convention, confirm-before-merge, and
finish-after-merge. Everything else bends — merge queues instead of
auto-merge, required human review, a `develop` branch instead of `main`,
rebase merges if your team insists (then `merged_as` reports the rebased
commits, plural). If you drop a load-bearing part, know what you are giving
up: each one exists to keep the ledger, the git history, and reality
telling the same story.
