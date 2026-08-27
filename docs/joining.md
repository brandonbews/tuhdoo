---
title: Joining an existing tuhdoo repo
description: How a new machine joins a repository that already uses tuhdoo in four steps, plus the branch-protection and CI settings the repo admin sets once.
---

# Joining an existing tuhdoo repo

This page is for anyone setting up a new machine — a teammate's laptop or your second workstation — against a repository that already uses tuhdoo. By the end, the machine sees the team's backlog and can connect an agent. A final section covers the settings the repo admin sets once for the whole team.

There is no server to register with and no account to create. The shared state is the **data branch**: a git orphan branch (`tuhdoo` by default) inside the repo you are about to clone. Joining is four steps: clone, install the binary, run `tuhdoo init`, and verify. This page is self-contained.

The reverse move — removing tuhdoo from a machine or a repo — is [`uninstall.md`](uninstall.md).

## 1. Clone the repository

```sh
git clone <remote-url> my-repo
cd my-repo
```

A plain, full clone (as above) is the recommended shape. Other shapes:

- **`--single-branch` works.** The data branch never needs to be part of your clone. tuhdoo fetches `refs/heads/tuhdoo` from the remote by an explicit refspec, into its own `refs/tuhdoo/remote` tracking ref, so it assumes no remote-tracking configuration.
- **Shallow clones (`--depth=…`) work.** The data branch is fetched fresh from the remote as above, and tuhdoo replays state from that branch's tip tree only. Truncated history on your code branches never matters to it.
- **Don't run tuhdoo from a fork.** The daemon syncs with `origin` and nothing else. Cloned from a fork, it faithfully syncs the *fork's* data branch, silently maintaining a divergent copy of the team's ledger. Clone the repository the team actually shares.
- **Bare and `--mirror` clones can't run tuhdoo.** They have no worktree, and tuhdoo runs inside a working repository.

## 2. Install the binary

Any one of these works; all produce the same single static binary. tuhdoo runs on macOS and Linux. On Windows, use the Windows Subsystem for Linux (WSL).

**Via npm, pnpm, or yarn.** Recommended for TypeScript and JavaScript projects, because it pins the version in your lockfile:

```sh
npm i -D tuhdoo      # or: pnpm add -D tuhdoo
                     # or: yarn add -D tuhdoo
```

Then invoke it as `npx tuhdoo` wherever this page says `tuhdoo`. The real binary ships in a per-platform package gated by `os`/`cpu` fields; pnpm and yarn resolve those platform-specific optional dependencies correctly, same as npm.

**Via a release archive.** Download the archive for your platform and `checksums.txt` from <https://github.com/brandonbews/tuhdoo/releases>, verify with `shasum -a 256 --check --ignore-missing checksums.txt`, extract, and put `tuhdoo` on your `PATH` (for example `install -m 755 tuhdoo /usr/local/bin/tuhdoo`).

**Via the Go toolchain:**

```sh
go install github.com/brandonbews/tuhdoo/cmd/tuhdoo@latest
```

## 3. Run `tuhdoo init`

From anywhere inside the clone, run:

```sh
tuhdoo init
```

The command starts the per-repo daemon — the local process that owns all writes to the data branch and syncs it with the remote — and confirms the data branch exists. Joining is automatic: when the remote already carries a `tuhdoo` branch, the daemon adopts it as its local copy instead of minting a fresh one. The command is idempotent, so running it again at any time is safe. There is nothing to configure and no flags to pass.

Running it offline is fine. `tuhdoo init` works fully locally, and the first sync after the remote becomes reachable merges histories automatically. That convergence is tuhdoo's normal operating mode, not a repair.

## 4. Verify

Run:

```sh
tuhdoo status
```

It shows the data branch, `syncing with "origin"`, and a running daemon. Then run:

```sh
tuhdoo backlog
```

It lists the team's existing tasks, not an empty table. This is the **ledger** you just joined: tuhdoo's append-only record of tasks, claims, notes, outcomes, and questions.

From here, bare `tuhdoo` opens the interactive terminal user interface (TUI), and the `tuhdoo init` output includes the Model Context Protocol (MCP) snippet that connects an agent harness. Connected agents follow [`agent-protocol.md`](agent-protocol.md).

## 5. Set your work identity (if needed)

Everything you write to the ledger is attributed to a **principal** derived from your git identity: the local part of `user.email`, so `sarah@example.com` acts as `sarah`. When that derivation is wrong — say, a host noreply address like `4099114+sarah@users.noreply.github.com`, or a work identity that differs from your commit email — override it once per clone:

```sh
git config tuhdoo.principal sarah
```

`--global` works too, with ordinary git config precedence.

## For the repo admin: branch protection and CI

Three one-time settings on the shared repository, all also printed by `tuhdoo init`:

- **Exempt the data branch from pull-request and review requirements.** If your host enforces rules on all branches — for example, a GitHub ruleset targeting "all branches" that requires pull requests — it rejects the daemon's pushes to `tuhdoo`, and every machine's ledger silently stops publishing while continuing to work locally. Exclude the `tuhdoo` branch from any rule requiring pull requests, reviews, or status checks. This is safe by construction: tuhdoo daemons write the branch (plus at most a rare sanctioned hand commit — see the autodeploy bullet below), it moves fast-forward only, and it is never force-pushed, so a rule *blocking* force pushes to it is harmless.
- **Exclude the data branch from continuous integration (CI) triggers**, so ledger syncs don't burn CI runs. For GitHub Actions: `on: { push: { branches-ignore: ["tuhdoo"] } }`.
- **Silence the data branch in hosts that autodeploy it.** The data branch is a real branch, and tuhdoo pushes it often once init has run. Whether that triggers anything — and what actually stops it — differs by host:

  - **Vercel** deploys automatically on every branch push. On private repos it first checks that the commit author is an authorized member of the Vercel team ([Vercel's docs](https://vercel.com/docs/deployments/troubleshoot-project-collaboration)). The ledger's commits are authored by `tuhdoo daemon <daemon@tuhdoo.invalid>`, which can never be a team member, so every ledger push produces a blocked deployment and a warning. The author check runs **before** the dashboard's Ignored Build Step (verified live, 2026-08-21), so an ignore rule alone can't silence it wherever that check applies. A `vercel.json` on your default branch does nothing here either, because Vercel reads configuration from the pushed commit (also verified live). The fix that works is a `vercel.json` committed onto the data branch itself: the one sanctioned hand commit on an otherwise daemon-only branch (one ordinary commit, never a force-push). Walkthrough: [`recipes/vercel.md`](recipes/vercel.md). Projects that set a Root Directory (monorepos deploying a subdirectory) are immune without any of this: Vercel skips pushes that don't touch that directory up front (verified live).
  - **Netlify** builds only the production branch unless branch deploys were explicitly enabled ([Netlify's docs](https://docs.netlify.com/deploy/deploy-types/branch-deploys/)), so a default-configured site never builds the data branch — usually there is nothing to do. If your site does deploy branches beyond production, scope that setting (individual branches, or a prefix pattern) so it misses the data branch. Not tested live. Per Netlify's docs, private-repo builds triggered by authors who aren't team members are held as "Pending approval" deploy requests — so a site deploying all branches of a private repo would collect one held request per ledger sync.
  - **Cloudflare Pages** builds every non-production branch by default ([Cloudflare's docs](https://developers.cloudflare.com/pages/configuration/branch-build-controls/)). Exclude the data branch in the project's branch-control settings (custom branches, with an exclude rule). Its docs describe no commit-author check like Vercel's. Not tested live.
