---
title: Uninstalling tuhdoo
description: How a machine walks away from tuhdoo with zero trace using ordinary git commands, and how a team retires its shared ledger. The reverse of joining.
---

# Uninstalling tuhdoo

This page shows how a machine, or a whole team, walks away from tuhdoo. It
is the companion to [`joining.md`](joining.md): joining is four steps, and
leaving is a handful of ordinary git commands, because nothing tuhdoo does
touches your code history. The ledger is an orphan branch, there are no git
hooks, and your worktree is never written to. This page is self-contained.

The steps come in two layers, deliberately separate:

- **Per machine**: safe and reversible. This removes every trace of tuhdoo
  from one clone; as long as the team's ledger still exists on the remote,
  `tuhdoo init` rejoins with full history at any time.
- **For the team**: irreversible, and usually unnecessary. This retires the
  shared ledger itself.

There is intentionally no `tuhdoo uninstall` command. Leaving via ordinary
git commands means you can read exactly what each step removes, and the
one destructive team-level step must never be automated.

## What tuhdoo leaves on a machine

The complete footprint, verified against the code:

- **Three refs.** `refs/heads/tuhdoo`, the local copy of the data branch;
  `refs/remotes/origin/tuhdoo`, an ordinary remote-tracking ref (full
  clones only); and `refs/tuhdoo/remote`, the daemon's own tracking ref,
  where its fetches of the remote data branch land.
- **The runtime directory** `.git/tuhdoo/`, containing `daemon.json`,
  `daemon.lock`, `daemon.log`, `daemon.sock`, and `machine-id`.
- **One git config key**, `tuhdoo.principal`, repo-local or `--global`
  if you set it there. It is only present if you overrode the default
  identity.
- **The MCP entry** in your agent harness's config (the snippet `tuhdoo
  init` printed), if you added one.
- **The binary.**

Nothing else is left: no hooks, no writes to your worktree, no commits on
your code branches. After the steps below, the repository looks exactly as
if tuhdoo had never run.

## Per machine: walk away clean

Run everything below from the repository root. (In a linked worktree, the
runtime dir lives under `$(git rev-parse --git-dir)/tuhdoo` rather than
`.git/tuhdoo`; substitute accordingly.)

### 1. Stop the daemon

The daemon's pid is in `.git/tuhdoo/daemon.json`. Send it SIGTERM and wait
for it to actually exit: shutdown flushes and publishes any last events,
so killing harder than TERM can strand your final writes locally.

<!-- uninstall-test: run -->
```sh
if [ -f .git/tuhdoo/daemon.json ]; then
  pid="$(sed -n 's/.*"pid":[[:space:]]*\([0-9][0-9]*\).*/\1/p' .git/tuhdoo/daemon.json)"
  kill -TERM "$pid"
  for _ in $(seq 1 50); do kill -0 "$pid" 2>/dev/null || break; sleep 0.1; done
fi
```

No `daemon.json` means no daemon is running; the block skips itself.

### 2. Remove the runtime directory

A clean shutdown already removed the socket and `daemon.json`; this clears
the lock, log, and machine-id too:

<!-- uninstall-test: run -->
```sh
rm -rf .git/tuhdoo
```

### 3. Delete the three refs

Depending on clone shape and history, any of the three may be absent (a
`--single-branch` clone has no remote-tracking ref; a repo that never
synced has no `refs/tuhdoo/remote`). This deletes exactly the ones that
exist, silently skipping the rest:

<!-- uninstall-test: run -->
```sh
git for-each-ref --format='delete %(refname)' \
  refs/heads/tuhdoo refs/remotes/origin/tuhdoo refs/tuhdoo/ \
  | git update-ref --stdin
```

This touches only your machine's refs; the team's ledger on the remote is
unaffected, and other machines never notice.

### 4. Unset your principal

<!-- uninstall-test: run -->
```sh
git config --unset tuhdoo.principal || true
git config --global --unset tuhdoo.principal || true
```

(The `|| true` is there because most machines never set the key, and
`git config --unset` exits non-zero when there is nothing to unset.)

### 5. Remove the harness MCP entry and the binary

Both are environment-specific. Delete the `"tuhdoo"` entry from your agent
harness's MCP config (the snippet `tuhdoo init` printed, under
`"mcpServers"`), then remove the binary however it arrived:

```sh
npm uninstall -D tuhdoo      # if installed via npm
rm "$(command -v tuhdoo)"    # if installed from a release archive or `go install`
```

### Verify: zero trace

Every check below succeeds only when nothing is left; the last line prints
only if all of them pass:

<!-- uninstall-test: run -->
```sh
test -z "$(git for-each-ref refs/heads/tuhdoo refs/remotes/origin/tuhdoo refs/tuhdoo/)" \
  && ! git config --get tuhdoo.principal \
  && ! test -e .git/tuhdoo \
  && echo "clean: no trace of tuhdoo on this machine"
```

And `git status` looks exactly as it did before you started: tuhdoo never
writes to your worktree, so there is nothing there to clean up.

*Maintainers: the fenced blocks above marked with `<!-- uninstall-test:
run -->` comments are executed verbatim by
`TestUninstallDocStepsLeaveZeroTrace` (`cmd/tuhdoo/uninstall_doc_test.go`)
against a temp repo with a running daemon. The doc is the single source of
truth: edit a step here and the test re-proves the walk-away claim.*

## For the team: retiring the ledger

You probably don't need to. A dormant `tuhdoo` branch costs nothing: it is
a small stretch of orphan history that never touches your code branches,
never appears in `--single-branch` clones, and (configured per
`joining.md`) never triggers CI. It is also the team's decision record:
every task, note, escalation, and answer. The recommended way to stop
using tuhdoo is simply to stop: each machine runs the per-machine steps
above, and the branch sits untouched on the remote, rejoinable later.

If you want the branch name gone but the history kept, archive it first,
from any machine that still has the local branch:

```sh
git push origin refs/heads/tuhdoo:refs/tags/tuhdoo-archive
```

then delete the branch as below; the history stays reachable through the
tag.

### Deleting the remote branch

This is the one irreversible step in this document. It destroys the whole ledger
for every peer at once, with no undo beyond a surviving local copy on some
machine. Two things must happen first:

1. **Every machine stops (or fully uninstalls) its daemon.** A live
   daemon on any peer will faithfully republish the branch on its next
   sync; from its point of view the remote merely lost history that it
   still has. Coordinate: run the per-machine steps everywhere, then
   delete.
2. **Lift any host protection on the data branch.** If your host restricts
   branch deletion, release the `tuhdoo` branch from that rule first.

Then, typed by a human, on purpose:

```sh
git push origin --delete tuhdoo
```

If you regret it, recovery is still possible: any machine that has not yet
run the per-machine cleanup still holds the full ledger in
`refs/heads/tuhdoo`, and a plain `git push origin tuhdoo` from there
restores everything.
