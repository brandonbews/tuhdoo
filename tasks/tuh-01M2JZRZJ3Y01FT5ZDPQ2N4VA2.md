# Release: re-enable the npm publish job and ship v0.6.0 to npm

`tuh-01M2JZRZJ3Y01FT5ZDPQ2N4VA2`

- **Status:** done
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

### 2026-09-15 17:02 UTC — run by `brandon/claude-code-3` — done

- Branch: `tuh-01M2JZRZ/reenable-npm-publish`
- PR: <https://github.com/brandonbews/tuhdoo/pull/114>
- Merged as: `adfb0cd2e2eb02bdb12f813dbb9a513d5e1bf29d`

release.yml's npm job runs again (`if: false` removed; PARKED comment replaced with a history note: 0.1.0–0.5.0 retired 2026-09-12, first republished version 0.6.0, --provenance needs a public repo). Tagged v0.6.0 on the merge commit (same code as v0.5.0 plus the workflow fix). Release run 34998019709: both jobs green; GitHub Release v0.6.0 has the four tarballs + checksums; npm job published @tuhdoo/{darwin-arm64,darwin-x64,linux-arm64,linux-x64}@0.6.0 and tuhdoo@0.6.0 via OIDC trusted publishing, each with a sigstore provenance statement (trusted-publisher config survived the hand-published stubs). tuhdoo `latest` is 0.6.0 on the registry; platform packages were still replicating minutes after publish. Still by hand from Brandon's npm login (optional): `npm deprecate <pkg>@0.0.1 "placeholder"` on the five names.
