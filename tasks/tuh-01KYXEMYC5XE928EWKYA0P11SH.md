# npm provenance: trusted publishing promised attestations, the registry has none

`tuh-01KYXEMYC5XE928EWKYA0P11SH`

- **Status:** open — in progress, claimed by `brandon/claude-code-1`
- **Priority:** 1
- **Labels:** `distribution` `ci` `npm` `investigation`
- **Created:** 2026-08-01 01:22 UTC by `brandon`

## Description

## Context

Brandon's answer to escalation 01KYVKR3D1RCVGFEWFPQXFB4AY (2026-07-31) ordered a follow-up for two accepted warts in the npm release job. Investigation on 2026-07-31 found one is already fixed and the other is real but different than assumed:

- **Partial-publish re-run guard: DONE.** release.yml now skips already-published versions ("already published, skipping", ~lines 116-124); it's what let the v0.1.0 re-run succeed after two failed attempts.
- **Provenance: claimed but absent.** The workflow switched to OIDC trusted publishing (tuh-01KYWKT8NQ980F0NF4MJ9W33H5) and its comment at line 76 says "provenance attestations come free." The registry disagrees: as of v0.1.1, `npm view <pkg> --json` shows no `dist.attestations` on any of the five packages (tuhdoo + 4 @tuhdoo platform packages) — only the registry signature. prepare.js does set a `repository` field, so the classic repo-mismatch cause may not be it.

## The ask

Find out why trusted publishing didn't attach provenance and fix it. Suspects to rule out: npm CLI version on the runner (auto-provenance via trusted publishing has a minimum version), the publish invocation needing an explicit `--provenance` flag in this setup, a silent skip logged in the run output (check the v0.1.1 run logs, job "npm"), or a repository-URL normalization mismatch between prepare.js's REPOSITORY and the OIDC claim. Correct the misleading comment if the fix contradicts it.

## Acceptance

- The next tag's packages carry provenance attestations: `npm view <pkg> --json` shows `dist.attestations` for all five, and npmjs.com shows the provenance badge.
- Whatever the cause was is stated plainly in the PR body.
- Workflow-file law: any release.yml change is called out explicitly and separately in the session summary for Brandon's eyes-on diff review — never folded into a larger commit.

## Pointers

- .github/workflows/release.yml (npm job), npm/prepare.js (REPOSITORY at ~line 70), run logs for v0.1.1 (gh run view 30657767961).

## History

### 2026-08-01 03:50 UTC — escalation from `brandon/claude-code-1` (blocking)

> PR #13 (https://github.com/brandonbews/tuhdoo/pull/13) changes .github/workflows/release.yml and per the workflow-file law needs your eyes-on diff review before merge — auto-merge is deliberately NOT enabled. The diff is two hunks: (1) comment correction, (2) `npm publish --access public --provenance` (one added flag). Root cause in the PR body: npm auto-enable of provenance under trusted publishing fails silently (verbose-only logging, npm/cli oidc.js); explicit flag makes future failures loud. Options: (a) review and merge PR #13 yourself, or (b) reply approving it and the next claimant merges and finishes. Recommendation: (a) — one-glance diff. Registry verification (dist.attestations on all five packages + npmjs badge) is deferred to the next v* tag by nature.

_Unanswered._

### 2026-08-01 03:50 UTC — note from `brandon/claude-code-1`

Resume state: investigation complete, fix committed on branch tuh-11sh/npm-provenance, PR #13 open with full root-cause writeup; only the merge (blocked on the workflow-law review escalation) and post-merge finish remain. No deploy needed (workflow file only, binary unchanged). After merge: pull main, finish_run(done). Next v* tag verifies the registry acceptance.
