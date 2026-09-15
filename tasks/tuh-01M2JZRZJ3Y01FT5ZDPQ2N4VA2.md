# Release: re-enable the npm publish job and ship v0.6.0 to npm

`tuh-01M2JZRZJ3Y01FT5ZDPQ2N4VA2`

- **Status:** open — in progress, claimed by `brandon/claude-code-3`
- **Priority:** 1
- **Labels:** `release` `ci`
- **Created:** 2026-09-15 16:53 UTC by `brandon/claude-code-3`

## Description

Context: tuhdoo went private on 2026-09-12 (PR #113 parked the `npm` job in release.yml with `if: false`; the five npm names hold 0.0.1 placeholder stubs; 0.1.0–0.5.0 were unpublished and are retired). The repo and site are public again as of 2026-09-15.

The ask: switch the npm tier back on and republish. Remove `if: false` from the `npm` job in `.github/workflows/release.yml`; replace the PARKED comment with a short history note (0.1.0–0.5.0 retired 2026-09-12; first republished version is 0.6.0). Then tag v0.6.0 on the merged main and confirm the npm job publishes all five packages.

Acceptance:
- `npm` job in release.yml runs on tag pushes again; comment records the retired-versions history.
- `make test lint` green; PR merged.
- v0.6.0 tag pushed; GitHub Release created; `npm view tuhdoo` shows 0.6.0 as latest, all four @tuhdoo/* platform packages at 0.6.0.

Constraints: workflow change — call it out separately for Brandon's eyes-on diff review. No other files in the PR.

## History

_No activity yet._
