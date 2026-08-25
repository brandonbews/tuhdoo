---
title: Joining an existing tuhdoo repo
description: How a new machine joins a repository that already uses tuhdoo in four steps, plus the branch-protection and CI settings the repo admin sets once.
---

# Joining an existing tuhdoo repo

This page shows how a new machine, whether a teammate's laptop or your
second workstation, joins a repository that already uses tuhdoo. There is
no server to register with and no account to create; the coordination
ledger is an orphan git branch (`tuhdoo` by default) inside the repo you
are about to clone. Joining is four steps: clone, install the binary, run
`tuhdoo init`, and verify. This page is self-contained.

(The reverse move, removing tuhdoo from a machine or a repo, is
[`uninstall.md`](uninstall.md).)

## 1. Clone the repository

```sh
git clone <remote-url> my-repo
cd my-repo
```

A plain, full clone (as above) is the recommended shape. Other shapes:

- **`--single-branch` works.** The data branch never needs to be part of
  your clone: tuhdoo fetches `refs/heads/tuhdoo` from the remote by an
  explicit refspec (into its own `refs/tuhdoo/remote` tracking ref), so no
  remote-tracking configuration is assumed.
- **Shallow clones (`--depth=…`) are fine.** The data branch is fetched
  fresh from the remote as above, and tuhdoo replays state from that
  branch's tip tree only; truncated history on your code branches never
  matters to it.
- **Do not run tuhdoo from a fork.** The daemon syncs with `origin` and
  nothing else. Cloned from a fork, it will faithfully sync the *fork's*
  data branch, silently maintaining a divergent copy of the team's ledger.
  Clone the repository the team actually shares.
- **Bare and `--mirror` clones cannot run tuhdoo.** They have no worktree;
  tuhdoo runs inside a working repository.

## 2. Install the binary

Any one of these works; all produce the same single static binary.
tuhdoo runs on macOS and Linux; on Windows, use WSL.

Via npm (recommended for TS/JS projects, because it pins the version in
your lockfile):

```sh
npm i -D tuhdoo
```

then invoke it as `npx tuhdoo` wherever this page says `tuhdoo`.

Via a release archive: download the archive for your platform and
`checksums.txt` from <https://github.com/brandonbews/tuhdoo/releases>,
verify with `shasum -a 256 --check --ignore-missing checksums.txt`, extract,
and put `tuhdoo` on your `PATH` (e.g. `install -m 755 tuhdoo
/usr/local/bin/tuhdoo`).

Via the Go toolchain:

```sh
go install github.com/brandonbews/tuhdoo/cmd/tuhdoo@latest
```

## 3. Run `tuhdoo init`

From anywhere inside the clone:

```sh
tuhdoo init
```

This starts the per-repo daemon and confirms the data branch exists. It is
idempotent, so running it again anytime is safe, and joining is automatic:
when the remote already carries a `tuhdoo` branch, the daemon adopts it as
its local copy instead of minting a fresh one. There is nothing to
configure and no flags to pass.

Running it offline is fine: init works fully locally, and the first sync
after the remote becomes reachable merges histories automatically. That
convergence is tuhdoo's normal operating mode, not a repair.

## 4. Verify

```sh
tuhdoo status
```

should show the data branch, `syncing with "origin"`, and a running daemon.
Then:

```sh
tuhdoo backlog
```

should list the team's existing tasks: the ledger you just joined, not an
empty table. From here, bare `tuhdoo` opens the interactive TUI, and the
init output includes the MCP snippet that connects an agent harness (agents
then follow [`agent-protocol.md`](agent-protocol.md)).

## 5. Set your work identity (if needed)

Everything you write to the ledger is attributed to a principal derived
from your git identity: the local part of `user.email` (so
`sarah@example.com` acts as `sarah`). When that derivation is wrong, say a
host noreply address like `4099114+sarah@users.noreply.github.com` or a
work identity that differs from your commit email, override it once per
clone:

```sh
git config tuhdoo.principal sarah
```

(`--global` works too, with ordinary git config precedence.)

## For the repo admin: branch protection and CI

Three one-time settings on the shared repository, all also printed by
`tuhdoo init`:

- **Exempt the data branch from pull-request and review requirements.**
  If your host enforces rules on all branches (e.g. a GitHub ruleset
  targeting "all branches" that requires PRs), the daemon's pushes to
  `tuhdoo` are rejected and every machine's ledger silently stops
  publishing while continuing to work locally. Exclude the `tuhdoo` branch
  from any rule requiring pull requests, reviews, or status checks. This is
  safe by construction: the branch is written by tuhdoo daemons (plus at
  most a rare sanctioned hand commit — see the auto-deploy bullet below),
  moves fast-forward only, and is never force-pushed, so a rule *blocking*
  force pushes to it is harmless.
- **Exclude the data branch from CI triggers**, so ledger syncs don't burn
  CI runs. For GitHub Actions:
  `on: { push: { branches-ignore: ["tuhdoo"] } }`.
- **Silence the data branch in hosts that autodeploy it.** The data
  branch is a real branch, and tuhdoo pushes it often once init has run.
  Whether that triggers anything — and what actually stops it — differs by
  host:

  - **Vercel** deploys automatically on every branch push, and on private
    repos it first checks that the commit author is an authorized member
    of the Vercel team ([Vercel's
    docs](https://vercel.com/docs/deployments/troubleshoot-project-collaboration)).
    The ledger's commits are authored by `tuhdoo daemon
    <daemon@tuhdoo.invalid>`, which can never be a team member, so every
    ledger push produces a blocked deployment and a warning. The author
    check runs **before** the dashboard's Ignored Build Step (verified
    live, 2026-08-21), so an ignore rule alone cannot silence it wherever
    that check applies; and a `vercel.json` on your default branch does
    nothing here, because Vercel reads configuration from the pushed
    commit (also verified live). The fix that works is a `vercel.json`
    committed onto the data branch itself — the one sanctioned hand
    commit on an otherwise daemon-only branch (one ordinary commit, never
    a force-push). Walkthrough: [`recipes/vercel.md`](recipes/vercel.md).
    Projects that set a Root Directory (monorepos deploying a
    subdirectory) are immune without any of this: pushes that don't touch
    that directory are skipped up front (verified live).
  - **Netlify** builds only the production branch unless branch deploys
    were explicitly enabled ([Netlify's
    docs](https://docs.netlify.com/deploy/deploy-types/branch-deploys/)),
    so a default-configured site never builds the data branch — usually
    there is nothing to do. If your site does deploy branches beyond
    production, scope that setting (individual branches, or a prefix
    pattern) so it misses the data branch. Not tested live. Per Netlify's
    docs, private-repo builds triggered by authors who aren't team
    members are held as "Pending approval" deploy requests — so a site
    deploying all branches of a private repo would collect one held
    request per ledger sync.
  - **Cloudflare Pages** builds every non-production branch by default
    ([Cloudflare's
    docs](https://developers.cloudflare.com/pages/configuration/branch-build-controls/)).
    Exclude the data branch in the project's branch-control settings
    (custom branches, with an exclude rule). Its docs describe no
    commit-author check like Vercel's. Not tested live.
