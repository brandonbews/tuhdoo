# Onboarding remainder: teammate joining doc and branch-protection guidance (doc + init line)

`tuh-01KZA0VT234XJYVZWT8S09PK06`

- **Status:** done
- **Priority:** none
- **Labels:** `cli` `docs` `onboarding`
- **Created:** 2026-08-05 22:31 UTC by `brandon/claude-code-1`

## Description

Context: promoted from the migrated open-question "init UX remainder" at the 2026-08-06 grill (Brandon). The mechanics landed in clone-join (PR #38) and init hardening (#42); what's missing is the prose: no joining flow is documented anywhere (README and docs grepped clean), and branch-protection guidance exists nowhere — a host org whose ruleset requires PRs on all branches silently breaks the daemon's direct pushes to the data branch. This task is milestone evidence: v1 clause 3 ("Brandon's 5-person work team could be onboarded with tuhdoo init + docs alone") is unjudgeable until these docs exist, so the v1 milestone container now depends on this task. Absorbs (2026-08-06 triage grill) the repo-hosting edge-cases capture tuh-01KZA0VT234XJYVZWT8VGFG3NX — clone shapes turned out to be settled facts to document and pin with a test, not mechanism to build; full fact-check reasoning lives in that task's cancellation record.

The ask, one PR, three doc surfaces plus tests: (1) a "joining an existing tuhdoo repo" doc section (README or docs/ — fit it to where the uninstall doc from tuh-01KZA0VT234XJYVZWT93P2EK1S lands, they are the two ends of the same lifecycle): clone the repo, install the binary, run tuhdoo init (idempotent — it adopts the existing remote data branch), verify with tuhdoo status / tuhdoo backlog, and set git config tuhdoo.principal when the git identity isn't the right work identity. (2) Branch-protection guidance: in that doc, and one line in init output next to the existing CI-guidance paragraph (#42's MCP snippet is the precedent for init carrying adopter guidance): exempt the data branch from PR-required/review rules — the daemon pushes it directly, fast-forward only, never force. Phrase host-generically (T2): rulesets/branch protection as a concept, github-actions-style example at most, matching the CI paragraph's tone. (3) A clone-shapes paragraph in the joining doc (absorbed from tuh-01KZA0VT234XJYVZWT8VGFG3NX): recommend a plain full clone; --single-branch works by construction (adoption fetches refs/heads/tuhdoo by explicit refspec into refs/tuhdoo/remote — no remote-tracking config assumed); shallow clones are fine (replay reads only the data-branch tip tree, and clone --depth implies --single-branch, so adoption fetches the data branch fresh); do NOT run from a fork — the daemon syncs origin only, so a fork's daemon silently maintains a divergent copy of the ledger (fork/non-committer access is the v2+ parked pair, see tuh-01KZA0VT234XJYVZWT98B7NXEH); bare/--mirror clones have no worktree and cannot run tuhdoo. (4) Two adoption tests pinning that paragraph's claims: daemon adopt from a --single-branch clone and from a --depth=1 clone, asserting the remote data branch is adopted (one root, seeded state visible) — so the doc and reality cannot drift, same docs-plus-proving-test shape as the uninstall task.

Acceptance: the joining section exists and a newcomer could follow it end-to-end without other docs; the branch-protection guidance appears in both the doc and init output; the clone-shapes paragraph exists including the fork warning; init's golden/CLI tests updated for the new line; the two clone-shape adopt tests exist and pass; make test lint green.

Pointers: cmd/tuhdoo/commands.go runInit (the printf block with the CI paragraph); cmd/tuhdoo/cli_test.go and oneshot_golden_test.go for init output coverage; PR #38 (clone-join) for what init actually does on a fresh clone; internal/daemon/adopt_test.go (the plain-clone adopt test — template for the two new clone-shape tests) and internal/syncer/adopt_test.go; internal/syncer/adopt.go and syncer.go TrackingRef for the explicit-refspec mechanics the paragraph describes; the uninstall task tuh-01KZA0VT234XJYVZWT93P2EK1S for doc placement coherence.

Constraints: init stays flag-free and loud on unknown args (#42); no new commands or MCP surface; clone shapes get docs and tests only — no shallow/bare/fork detection code (examined and accepted at the 2026-08-06 grill); host-agnostic wording (T2 — GitHub may appear only as an example, as in the CI paragraph); no workflow-file changes.

## History

### 2026-08-06 21:27 UTC — edit by `brandon/claude-code-1`

retitled · description edited · status inbox→open · labels +onboarding

### 2026-08-06 22:40 UTC — edit by `brandon/claude-code-1`

description edited

### 2026-08-06 23:40 UTC — run by `brandon/claude-code-1` — done

- Branch: `tuh-pk06/joining-doc`
- PR: <https://github.com/brandonbews/tuhdoo/pull/47>
- Commits: `31c087b`
- Merged as: `01f90190cab537d1cba43616a438b73366a1e715`

Joining doc landed as docs/joining.md (clone/install/init/verify/principal-override, clone-shapes incl. fork warning, admin branch-protection + CI section), pointers added in README.md and docs/README.md, one branch-protection paragraph in init output (pinned by TestInitRemoteless), and two clone-shape adoption tests in internal/daemon/adopt_test.go (single-branch and shallow, each pre-asserting genuine clone shape; shallow clones via file:// since git ignores --depth on local paths). Both tests live in the daemon suite — the syncer suite never performs real clones. Task pointer correction for the record: init output is not goldened; cli_test.go TestInitRemoteless is the only init-output coverage. Squash-merged via PR #47. Binary changed (init output) — deploy/restart needed.
