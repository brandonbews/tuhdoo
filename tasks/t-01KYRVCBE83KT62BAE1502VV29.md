# t-01KYRVCBE83KT62BAE1502VV29 — npm devDependency distribution (esbuild-pattern wrapper packages)

- Status: open — waiting on an escalation answer
- Priority: 1
- Labels: `distribution`, `npm`
- Depends on: [t-01KYRVCBE83KT62BAE11W3TAM8](t-01KYRVCBE83KT62BAE11W3TAM8.md) (done)
- Created: 2026-07-30 06:28 UTC by `brandon/impl-2`

## Description

Context: TS-project users should get tuhdoo with `npm i -D tuhdoo` + `npx tuhdoo init`, pinning the binary per project in their lockfile — which is exactly the per-repo version pinning T4 wants. The established pattern for shipping native binaries on npm is esbuild/turbo/biome: per-platform binary packages selected by npm itself. Depends on the release pipeline task, which produces the binaries this wraps.

The ask: (1) per-platform npm packages (e.g. @tuhdoo/darwin-arm64, @tuhdoo/darwin-x64, @tuhdoo/linux-arm64, @tuhdoo/linux-x64), each containing only the prebuilt binary plus a package.json with matching os/cpu fields; (2) a main `tuhdoo` package listing them as optionalDependencies, whose bin shim (small, dependency-free JS) resolves the installed platform package and spawns the real binary, forwarding argv, stdio, and exit code faithfully — the MCP stdio shim runs through this, so no buffering, no output of its own, signals passed through; (3) CI publish step keyed to the same git tag as the GitHub Release, npm version locked to the tag. Do NOT use a postinstall download (breaks --ignore-scripts, offline mirrors, and vendored registries).

Acceptance: on a machine with only node+git, `npm i -D tuhdoo && npx tuhdoo init` works in a fresh repo and `npx tuhdoo status` reports local-only; an .mcp.json of {command: "npx", args: ["tuhdoo", "mcp"]} (and the node_modules/.bin/tuhdoo direct path) serves the ten verbs to a harness; the wrapper adds nothing to stdout/stderr of the shim path; package versions match the git tag; docs updated (README install section; agent-protocol.md Connecting mentions the npx form works unchanged).

Pointers: the release pipeline task's artifacts; cmd/tuhdoo/mcp_cmd.go (stdio shim — the thing the JS shim must not garble); esbuild's npm packaging as prior art.

Constraints: same workflow-file law as the release task — .github/workflows/ changes get called out separately for Brandon's eyes-on review. The Go binary stays the single source of behavior; the JS layer is a dumb launcher, never a reimplementation. Publishing credentials/npm scope setup is Brandon's to provision — escalate for the token rather than inventing a workaround.

## History

### 2026-07-31 08:13 UTC — escalation from `brandon/claude-code-11` (blocking)

**Q:** npm publishing needs credentials only you can provision. Everything else is landed and smoke-tested; the first `v*` tag will publish to npm automatically once these exist. Needed: (1) an npmjs.com account/org owning the `@tuhdoo` scope (create org "tuhdoo" — the scope and the `tuhdoo` package name were both unclaimed as of 2026-07-31); (2) a granular npm access token with read/write publish rights on the `tuhdoo` package and `@tuhdoo` scope (first publish creates them, so the token needs "create packages" rights on the scope); (3) that token added as the repo Actions secret NPM_TOKEN. Also: the repo has no LICENSE file and the npm packages currently ship without a license field — npm warns but publishes; tell me (or a successor) the intended license and we'll add it to the repo + packages. One workflow change awaits your eyes-on review per project law: commit 17ea914 adds an `npm` job to .github/workflows/release.yml (downloads release assets, assembles packages with npm/prepare.js, publishes with NPM_TOKEN via ${NODE_AUTH_TOKEN} in .npmrc; no new third-party actions). Recommendation: review 17ea914, provision the org+token, then push the first tag (e.g. v0.1.0) — that single tag exercises both the release pipeline and the npm tier end-to-end.

_Unanswered._

### 2026-07-31 08:13 UTC — note from `brandon/claude-code-11`

Resume state: all code/docs landed on main (2c57b6e npm packaging + docs, 17ea914 workflow job). Local validation: npm/smoke.sh passes end-to-end on darwin/arm64 (fresh-repo install from tarballs, launcher output byte-identical to the binary, init + status local-only, all eleven MCP verbs served through the launcher, SIGTERM forwarded to the binary). What the escalation answer unblocks: nothing to build — after NPM_TOKEN + @tuhdoo org exist and Brandon pushes the first v* tag, CI publishes; the successor's only job is to verify the published packages against the task's acceptance (npm i -D tuhdoo in a clean repo, npx tuhdoo init/status, .mcp.json npx form) and fold in the license decision if one was made. Gotcha for local testing: run npm/smoke.sh from a short path — the daemon's unix socket dies loudly past 103 bytes, which is why the script pins its workdir under /tmp.

### 2026-07-31 08:13 UTC — run by `brandon/claude-code-11` — blocked

All buildable work landed on main: commit 2c57b6e (npm/ packaging: dependency-free launcher, prepare.js assembler, smoke.sh end-to-end test, README + agent-protocol docs, /bin gitignore fix) and commit 17ea914 (release.yml npm publish job — WORKFLOW CHANGE, isolated for eyes-on review). Smoke test passes locally on darwin/arm64. Blocked on escalation 01KYVKR3D1RCVGFEWFPQXFB4AY: npm org/scope + NPM_TOKEN provisioning and the license decision are Brandon's; after that, the first v* tag publishes automatically and a successor verifies published-registry acceptance.
