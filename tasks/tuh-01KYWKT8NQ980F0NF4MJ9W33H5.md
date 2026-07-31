# tuh-01KYWKT8NQ980F0NF4MJ9W33H5 — Release npm job hardening: idempotent re-runs and provenance attestations

- Status: open — ready
- Priority: 0
- Labels: `distribution`, `ci`, `npm`
- Created: 2026-07-31 17:33 UTC by `brandon/claude-code-1`

## Description

Context: the release.yml npm job (commit 17ea914, approved by Brandon 2026-07-31 with these two warts explicitly accepted as follow-ups) publishes five packages in sequence with `set -e`. Two accepted gaps: (1) if publish dies midway (e.g. after @tuhdoo/darwin-arm64), a re-run hits "version already exists" on the first package and the job fails — recovery today is bumping to the next patch tag; (2) packages publish without npm provenance attestations.

The ask: (1) make the publish loop idempotent — before each `npm publish`, check whether that exact name@version is already on the registry (e.g. `npm view <name>@<version> version`) and skip if so, so a re-run after a partial failure completes the remainder; (2) add `--provenance` to the publish commands plus `id-token: write` to the npm job's permissions, giving the packages verifiable build attestations tied to the workflow run.

Acceptance: a re-run of the npm job after a simulated partial publish (some packages already at the version, some not) publishes only the missing ones and exits green; published packages show provenance on npmjs.com; the job's permissions gain only id-token: write; make test lint stays green.

Pointers: .github/workflows/release.yml npm job (lines ~68-115); npm/prepare.js; npm docs on `npm publish --provenance` (requires the registry to trust the GitHub OIDC issuer — works out of the box for public repos on npmjs.com).

Constraints: PROJECT LAW — this is a .github/workflows/ change; isolate it in its own commit and call it out explicitly for Brandon's eyes-on diff review. Do not restructure the job otherwise; the download-not-rebuild byte-identity property and platform-before-launcher publish order stay.

## History

_No activity yet._
