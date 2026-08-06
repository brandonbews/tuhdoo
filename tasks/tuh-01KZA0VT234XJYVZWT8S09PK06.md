# Onboarding remainder: teammate joining doc and branch-protection guidance (doc + init line)

`tuh-01KZA0VT234XJYVZWT8S09PK06`

- **Status:** open — ready
- **Priority:** 0
- **Labels:** `cli` `docs` `onboarding`
- **Created:** 2026-08-05 22:31 UTC by `brandon/claude-code-1`

## Description

Context: promoted from the migrated open-question "init UX remainder" at the 2026-08-06 grill (Brandon). The mechanics landed in clone-join (PR #38) and init hardening (#42); what's missing is the prose: no joining flow is documented anywhere (README and docs grepped clean), and branch-protection guidance exists nowhere — a host org whose ruleset requires PRs on all branches silently breaks the daemon's direct pushes to the data branch. This task is milestone evidence: v1 clause 3 ("Brandon's 5-person work team could be onboarded with tuhdoo init + docs alone") is unjudgeable until these docs exist, so the v1 milestone container now depends on this task.

The ask, one PR, two surfaces: (1) a "joining an existing tuhdoo repo" doc section (README or docs/ — fit it to where the uninstall doc from tuh-01KZA0VT234XJYVZWT93P2EK1S lands, they are the two ends of the same lifecycle): clone the repo, install the binary, run tuhdoo init (idempotent — it adopts the existing remote data branch), verify with tuhdoo status / tuhdoo backlog, and set git config tuhdoo.principal when the git identity isn't the right work identity. (2) Branch-protection guidance: in that doc, and one line in init output next to the existing CI-guidance paragraph (#42's MCP snippet is the precedent for init carrying adopter guidance): exempt the data branch from PR-required/review rules — the daemon pushes it directly, fast-forward only, never force. Phrase host-generically (T2): rulesets/branch protection as a concept, github-actions-style example at most, matching the CI paragraph's tone.

Acceptance: the joining section exists and a newcomer could follow it end-to-end without other docs; the branch-protection guidance appears in both the doc and init output; init's golden/CLI tests updated for the new line; make test lint green.

Pointers: cmd/tuhdoo/commands.go runInit (the printf block with the CI paragraph); cmd/tuhdoo/cli_test.go and oneshot_golden_test.go for init output coverage; PR #38 (clone-join) for what init actually does on a fresh clone; the uninstall task tuh-01KZA0VT234XJYVZWT93P2EK1S for doc placement coherence.

Constraints: init stays flag-free and loud on unknown args (#42); no new commands or MCP surface; host-agnostic wording (T2 — GitHub may appear only as an example, as in the CI paragraph); no workflow-file changes.

## History

_No activity yet._
