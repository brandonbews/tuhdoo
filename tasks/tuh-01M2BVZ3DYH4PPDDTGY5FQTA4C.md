# Release: park the npm publish job while tuhdoo is pre-launch

`tuh-01M2BVZ3DYH4PPDDTGY5FQTA4C`

- **Status:** done
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

### 2026-09-13 00:27 UTC — run by `brandon/claude-code-1` — done

- Branch: `tuh-01M2BVZ3/park-npm-publish`
- PR: <https://github.com/brandonbews/tuhdoo/pull/113>
- Merged as: `1f2a9a176202a7e0117865d0fe06d227cfdf73a3`

release.yml's npm job now has `if: false` plus a PARKED comment (re-enable notes: first real version must be > 0.5.0; --provenance fails while the repo is private). GitHub Release job, npm/prepare.js, npm/smoke.sh untouched. Registry side done by Brandon by hand and verified: all five names (tuhdoo, @tuhdoo/{darwin-arm64,darwin-x64,linux-arm64,linux-x64}) hold only a 0.0.1 "Reserved" placeholder tagged latest; 0.1.0–0.5.0 unpublished. Gotcha for re-launch: npm 2FA-bypass granular tokens can publish/dist-tag but cannot unpublish (403, since 2026-07-31). README/docs/site still say `npm i -D tuhdoo` — left as-is, correct again at launch.
