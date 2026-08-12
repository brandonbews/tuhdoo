# Repo-hosting edge cases: shallow clones, --single-branch, forks, mirrors

`tuh-01KZA0VT234XJYVZWT8VGFG3NX`

- **Status:** cancelled
- **Priority:** 0
- **Labels:** `design` `gitx`
- **Created:** 2026-08-05 22:31 UTC by `brandon/claude-code-1`

## Description

Cancelled as subsumed, 2026-08-06 triage grill (Brandon). The surviving work — a clone-shapes paragraph in the teammate joining doc plus two pinning tests (adopt from --single-branch and --depth=1 clones) — was folded into the onboarding remainder task tuh-01KZA0VT234XJYVZWT8S09PK06 before this cancellation. No mechanism gets built; every clone shape resolved to a settled fact, a documented warning, or an accepted limitation.

Fact-check that dissolved the capture (2026-08-06, against live code):

- Shallow clones: replay reads only the data-branch tip tree — store.LoadReplayInput is one ls-tree plus cat-file, no history walk anywhere in non-test code; 001 D9.4 already declares history forensic, not load-bearing. So shallow history cannot break state. git clone --depth implies --single-branch, so the common shallow clone lacks the data branch entirely and adoption fetches it fresh and un-depth-limited. Residual exotic case (--depth --no-single-branch, giving a shallow data-branch tip): merge-base --is-ancestor can misjudge ancestry across the shallow boundary and produce a spurious app-level merge commit — still convergent under the union merge; cosmetic, not corruption. Examined and accepted; no shallow detection built.

- --single-branch: works by construction since clone-join (PR #38). Adoption fetches refs/heads/tuhdoo by explicit command-line refspec into refs/tuhdoo/remote (internal/syncer/adopt.go; TrackingRef in syncer.go), a ref tuhdoo owns precisely so no remote-tracking config is assumed; the configured fetch refspec is irrelevant. Settled by design after this capture was written; nothing to build.

- Forks: the syncer remote is origin with no knob (daemon never sets Options.Remote), so a fork's daemon silently maintains the fork's own divergent copy of the ledger. But fork-based work is the non-committer model D4's trust boundary already excludes — that pair (read-only sharing, public intake bridge) is parked in the v2+ pointer task tuh-01KZA0VT234XJYVZWT98B7NXEH. The served-today answer is a doc line — clone, don't fork — now owned by the surviving task.

- Mirrors/bare clones: no worktree, so the CLI fails at repo discovery with a generic "not a git repository". Nobody plans work inside a --mirror; examined and accepted, no bare-repo diagnosis built.

- The capture's "degrade loudly, not mysteriously" ask is already served where it matters: sync failures surface as status Mode="error" with LastError (rendered by tuhdoo status and the TUI), incomprehensible events trip fail-safe read-only (T3), non-blob tree entries hard-fail, and the confirmation gate refuses loudly (503) when the remote is unreachable. No new loudness mechanism was justified by any clone shape.

## History

### 2026-08-06 22:40 UTC — edit by `brandon/claude-code-1`

description edited · status inbox→cancelled
