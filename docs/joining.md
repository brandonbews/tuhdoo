---
title: Joining an existing tuhdoo repo
description: How a new machine joins a repository that already uses tuhdoo — clone, install, tuhdoo init, verify — plus the branch-protection and CI settings the repo admin sets once.
---

# Joining an existing tuhdoo repo

How a new machine — a teammate's laptop, your second workstation — joins a
repository that already uses tuhdoo. There is no server to register with
and no account to create: the coordination ledger is an orphan git branch
(`tuhdoo` by default) inside the repo you are about to clone, and joining
is four steps — clone, install the binary, `tuhdoo init`, verify. This page
is self-contained.

(The reverse move — removing tuhdoo from a machine or a repo — is
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
  branch's tip tree only — truncated history on your code branches never
  matters to it.
- **Do not run tuhdoo from a fork.** The daemon syncs with `origin` and
  nothing else. Cloned from a fork, it will faithfully sync the *fork's*
  data branch — silently maintaining a divergent copy of the team's ledger.
  Clone the repository the team actually shares.
- **Bare and `--mirror` clones cannot run tuhdoo.** They have no worktree;
  tuhdoo runs inside a working repository.

## 2. Install the binary

Any one of these; all produce the same single static binary.

Via npm (recommended for TS/JS projects — pins the version in your
lockfile):

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
idempotent — safe to run again anytime — and joining is automatic: when the
remote already carries a `tuhdoo` branch, the daemon adopts it as its local
copy instead of minting a fresh one. Nothing to configure, no flags.

Offline at the time? Still fine: init works fully locally, and the first
sync after the remote becomes reachable merges histories automatically —
that convergence is tuhdoo's normal operating mode, not a repair.

## 4. Verify

```sh
tuhdoo status
```

should show the data branch, `syncing with "origin"`, and a running daemon.
Then:

```sh
tuhdoo backlog
```

should list the team's existing tasks — the ledger you just joined, not an
empty table. From here, bare `tuhdoo` opens the interactive TUI, and the
init output includes the MCP snippet that connects an agent harness (agents
then follow [`agent-protocol.md`](agent-protocol.md)).

## 5. Set your work identity (if needed)

Everything you write to the ledger is attributed to a principal derived
from your git identity: the local part of `user.email` (so
`sarah@example.com` acts as `sarah`). When that derivation is wrong — a
host noreply address like `4099114+sarah@users.noreply.github.com`, or a
work identity that differs from your commit email — override it once per
clone:

```sh
git config tuhdoo.principal sarah
```

(`--global` works too, with ordinary git config precedence.)

## For the repo admin: branch protection and CI

Two one-time settings on the shared repository, both also printed by
`tuhdoo init`:

- **Exempt the data branch from pull-request and review requirements.**
  If your host enforces rules on all branches (e.g. a GitHub ruleset
  targeting "all branches" that requires PRs), the daemon's pushes to
  `tuhdoo` are rejected and every machine's ledger silently stops
  publishing while continuing to work locally. Exclude the `tuhdoo` branch
  from any rule requiring pull requests, reviews, or status checks. This is
  safe by construction: the branch is written only by tuhdoo daemons, moves
  fast-forward only, and is never force-pushed — a rule *blocking* force
  pushes to it is harmless.
- **Exclude the data branch from CI triggers**, so ledger syncs don't burn
  CI runs — e.g. for GitHub Actions:
  `on: { push: { branches-ignore: ["tuhdoo"] } }`.
