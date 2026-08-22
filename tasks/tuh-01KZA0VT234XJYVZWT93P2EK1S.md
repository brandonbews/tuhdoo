# Uninstall doc + test: prove a team can walk away clean

`tuh-01KZA0VT234XJYVZWT93P2EK1S`

- **Status:** done
- **Priority:** none
- **Labels:** `docs` `cli` `onboarding`
- **Created:** 2026-08-05 22:31 UTC by `brandon/claude-code-1`

## Description

Context: promoted from the migrated open-question "uninstall story" at the 2026-08-06 grill (Brandon). The footprint inventory (verified against the live repo) makes the capture's claim essentially true, and nothing tuhdoo does contaminates code history — the ledger is an orphan branch, there are no git hooks, no worktree writes beyond the adopter's own MCP config. Full per-machine footprint: three refs (refs/heads/tuhdoo, refs/remotes/origin/tuhdoo, refs/tuhdoo/remote — the last is the syncer's tracking ref, internal/syncer/syncer.go TrackingRef), the .git/tuhdoo/ runtime dir (daemon.json/lock/log/sock, machine-id), git config tuhdoo.principal (may be repo-local or --global — check both), the harness MCP entry, and the binary. Team-level footprint: the tuhdoo branch on the remote — the whole ledger, shared by every peer. Decided at the grill: docs + a test that proves the claim; no `tuhdoo uninstall` CLI command (leaving via five ordinary git commands IS the pitch; a command can come later if an adopter asks, and the destructive team half must never be automated).

The ask: (1) an uninstall doc — docs/uninstall.md or wherever the user-facing docs task later homes it — in two layers: per-machine walk-away (stop the daemon first: TERM the pid in .git/tuhdoo/daemon.json and wait for exit; then remove .git/tuhdoo/, delete the three refs, unset tuhdoo.principal local and global, remove the MCP entry) and team walk-away (deleting the remote tuhdoo branch: irreversible, breaks every peer at once, and usually unnecessary — a dormant orphan branch costs nothing and never touches code history; recommend abandoning or archiving over deleting, and note host branch-protection on the data branch may need lifting first). (2) A test that executes the documented per-machine steps against a temp repo with a running daemon and asserts zero trace: no tuhdoo refs, clean git status, config unset, process gone, .git/tuhdoo/ absent — so the doc and reality cannot drift silently.

Acceptance: the doc exists with both layers and the destructive-step framing above; the test runs the documented commands (not a parallel reimplementation of them — extract them from the doc or keep a single source) and passes; make test lint green.

Pointers: smoke.sh and its release-workflow gate (#41) for the end-to-end-shell precedent; internal/daemon tests for temp-repo daemon spin-up; cmd/tuhdoo/commands.go init output for the tone the doc should match.

Constraints: no new CLI or MCP surface; the doc must not assume a host (T2) — branch-protection notes phrased generically; if the test lands as a workflow change, remember workflow files need Brandon's eyes-on review (should not be needed — prefer a make target the existing test job already runs).

## History

### 2026-08-06 21:17 UTC — edit by `brandon/claude-code-1`

retitled · description edited · status inbox→open · labels +cli +onboarding −design

### 2026-08-06 23:52 UTC — run by `brandon/claude-code-1` — done

- Branch: `tuh-ek1s/uninstall-doc`
- PR: <https://github.com/brandonbews/tuhdoo/pull/48>
- Commits: `63f0da5`
- Merged as: `69781fa6da5b98d0534aeef5c4405e8afdc85ac4`

docs/uninstall.md landed with both layers (per-machine walk-away with runnable steps + zero-trace verify; team-level retirement framed irreversible/usually-unnecessary with abandon/tag-archive recommended, peer-daemon-republish precondition, host-protection note, recovery note). TestUninstallDocStepsLeaveZeroTrace (cmd/tuhdoo/uninstall_doc_test.go) executes the doc blocks marked <!-- uninstall-test: run --> verbatim against a temp repo with a live daemon — doc is single source, no reimplementation; global-config unset sandboxed via GIT_CONFIG_GLOBAL. joining.md companion link + docs/README.md + README.md pointers updated. Squash-merged via PR #48. Docs+test only — no binary change, no deploy needed.
