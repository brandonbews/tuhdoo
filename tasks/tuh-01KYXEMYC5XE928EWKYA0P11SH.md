# tuh-01KYXEMYC5XE928EWKYA0P11SH — npm provenance: trusted publishing promised attestations, the registry has none

- Status: open — ready
- Priority: 1
- Labels: `distribution`, `ci`, `npm`, `investigation`
- Created: 2026-08-01 01:22 UTC by `brandon`

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

_No activity yet._
