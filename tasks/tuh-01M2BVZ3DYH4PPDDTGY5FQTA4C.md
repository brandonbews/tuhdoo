# Release: park the npm publish job while tuhdoo is pre-launch

`tuh-01M2BVZ3DYH4PPDDTGY5FQTA4C`

- **Status:** open — ready
- **Priority:** 1
- **Labels:** `release` `ci`
- **Created:** 2026-09-12 22:31 UTC by `brandon/claude-code-1`

## Description

Context: Brandon is pulling tuhdoo off npm until it's polished (repo is private, site is behind Vercel auth). The five package names (tuhdoo, @tuhdoo/{darwin-arm64,darwin-x64,linux-arm64,linux-x64}) are being held with 0.0.1 placeholder stubs and the real 0.1.0–0.5.0 versions unpublished — that registry work is done by hand from Brandon's npm login, not by CI.

The ask: stop `.github/workflows/release.yml` from republishing real packages on the next v* tag, without deleting the npm tier (npm/prepare.js, npm/smoke.sh, the job itself) so it can be switched back on at launch.

Acceptance:
- The `npm` job in release.yml is skipped on tag pushes (e.g. `if: false`) with a comment saying why and what to revisit on re-enable: first real version must be > 0.5.0 (0.1.0–0.5.0 are retired), and `--provenance` fails from a private repo.
- The GitHub Release job is unchanged.
- `make test lint` green; PR merged.

Constraints: workflow change — call it out separately for Brandon's eyes-on diff review. No other files.

## History

_No activity yet._
