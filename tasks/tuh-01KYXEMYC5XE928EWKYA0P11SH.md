# tuh-01KYXEMYC5XE928EWKYA0P11SH — Release npm job hardening: partial-publish re-run guard, provenance attestations

- Status: open — ready
- Priority: 1
- Labels: `distribution`, `ci`, `npm`
- Created: 2026-08-01 01:22 UTC by `brandon`

## Description

## Context

Follow-up ordered in Brandon's answer to escalation 01KYVKR3D1RCVGFEWFPQXFB4AY (npm publishing credentials, 2026-07-31): workflow commit 17ea914 was approved as-is with two accepted warts to fix later. This task is that fix.

## The ask

Harden the `npm` job in .github/workflows/release.yml:

1. **Partial-publish re-run guard**: if a tag's publish run dies after publishing some of the five packages, re-running the job must not fail on the already-published ones — skip versions that already exist on the registry (e.g. check `npm view <pkg>@<version>` before publishing) so a re-run converges instead of erroring.
2. **Provenance attestations**: publish with `--provenance` (requires `id-token: write` permission on the job). Note tuh-01KYWKT8NQ980F0NF4MJ9W33H5 already moved publishing to OIDC trusted publishing — verify how provenance composes with that setup rather than assuming the NPM_TOKEN-era shape.

## Acceptance

- A re-run of the npm job on a tag whose packages are partially (or fully) published succeeds without manual registry cleanup.
- Published packages carry provenance attestations (visible on npmjs.com).
- Workflow-file law: this change is under .github/workflows/ — it must be called out explicitly and separately in the session summary for Brandon's eyes-on diff review, never folded into a larger commit.

## Pointers

- .github/workflows/release.yml (npm job from commit 17ea914, since amended by tuh-01KYWKT8NQ980F0NF4MJ9W33H5's OIDC switch), npm/prepare.js.

## History

_No activity yet._
