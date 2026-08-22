# Release npm job: switch to OIDC trusted publishing (drop NPM_TOKEN)

`tuh-01KYWKT8NQ980F0NF4MJ9W33H5`

- **Status:** done
- **Priority:** none
- **Labels:** `distribution` `ci` `npm`
- **Created:** 2026-07-31 17:33 UTC by `brandon/claude-code-1`

## Description

Context: the original two warts are half-resolved — idempotent re-runs landed in commit 610d611 (publish() wrapper in release.yml, exercised successfully on the v0.1.0 rollout when a token-scope failure left 4 of 5 packages published and the re-cut tag recovered cleanly). Provenance is superseded by this task: npm trusted publishing (OIDC) gives provenance automatically and removes the long-lived token entirely.

Gating (Brandon, browser-side, may already be done by the time you read this — verify): (1) on npmjs.com, add a trusted publisher to each of the five packages (tuhdoo, @tuhdoo/darwin-arm64, @tuhdoo/darwin-x64, @tuhdoo/linux-arm64, @tuhdoo/linux-x64): GitHub user brandonbews, repo tuhdoo, workflow release.yml, no environment; (2) delete the bootstrap npm token and the NPM_TOKEN repo secret.

The ask: rework the npm job in .github/workflows/release.yml for OIDC — add id-token: write to the job permissions; ensure npm >= 11.5.1 on the runner (preinstalled npm is older; `npm install -g npm@11` or setup-node with a node that bundles it); remove the .npmrc/NODE_AUTH_TOKEN wiring entirely. Keep the publish() idempotency wrapper and the platform-before-launcher order. npm CLI auto-detects OIDC; no token config remains anywhere.

Acceptance: next v* tag publishes all five packages with no NPM_TOKEN secret in the repo; packages show provenance attestations on npmjs.com; a re-run of a partially-failed npm job still skips already-published versions; make test lint green.

Pointers: .github/workflows/release.yml npm job; https://docs.npmjs.com/trusted-publishers (self-hosted runners unsupported; one publisher per package; requires npm >= 11.5.1 and node >= 22.14).

Constraints: PROJECT LAW — workflow change: isolated commit, explicit call-out for Brandon's eyes-on diff review. Byte-identity stays: download release assets, never rebuild.

## History

### 2026-07-31 18:42 UTC — edit by `brandon/claude-code-1`

retitled · description edited

### 2026-07-31 19:16 UTC — run by `brandon/claude-code-1` — done

- Branch: `main`
- Commits: `a771a65`

OIDC trusted publishing is live and verified end-to-end: commit a771a65 (eyes-on reviewed by Brandon in-session) added id-token: write, removed all NPM_TOKEN/.npmrc wiring, and upgraded runner npm to 11 in the open; Brandon configured trusted publishers on all five packages (repo brandonbews/tuhdoo, workflow release.yml, publish-only) and deleted the bootstrap token + secret. Tag v0.1.1 then published all five packages with no credential in existence — the auth half of acceptance is proven live. The publish() idempotency wrapper is unchanged. One acceptance item deferred: provenance attestations were NOT generated because the repo was private (npm silently skips provenance for private repos — it goes to Sigstore's public transparency log). Brandon decided in-session to make the repo public; once flipped, provenance turns on automatically at the next v* tag with zero workflow changes — whoever cuts the next release should confirm attestations appear (npm view <pkg> dist.attestations).
