# t-01KYRVCBE83KT62BAE1502VV29 — npm devDependency distribution (esbuild-pattern wrapper packages)

- Status: open — blocked on dependencies
- Priority: 1
- Labels: `distribution`, `npm`
- Depends on: [t-01KYRVCBE83KT62BAE11W3TAM8](t-01KYRVCBE83KT62BAE11W3TAM8.md) (open)
- Created: 2026-07-30 06:28 UTC by `brandon/impl-2`

## Description

Context: TS-project users should get tuhdoo with `npm i -D tuhdoo` + `npx tuhdoo init`, pinning the binary per project in their lockfile — which is exactly the per-repo version pinning T4 wants. The established pattern for shipping native binaries on npm is esbuild/turbo/biome: per-platform binary packages selected by npm itself. Depends on the release pipeline task, which produces the binaries this wraps.

The ask: (1) per-platform npm packages (e.g. @tuhdoo/darwin-arm64, @tuhdoo/darwin-x64, @tuhdoo/linux-arm64, @tuhdoo/linux-x64), each containing only the prebuilt binary plus a package.json with matching os/cpu fields; (2) a main `tuhdoo` package listing them as optionalDependencies, whose bin shim (small, dependency-free JS) resolves the installed platform package and spawns the real binary, forwarding argv, stdio, and exit code faithfully — the MCP stdio shim runs through this, so no buffering, no output of its own, signals passed through; (3) CI publish step keyed to the same git tag as the GitHub Release, npm version locked to the tag. Do NOT use a postinstall download (breaks --ignore-scripts, offline mirrors, and vendored registries).

Acceptance: on a machine with only node+git, `npm i -D tuhdoo && npx tuhdoo init` works in a fresh repo and `npx tuhdoo status` reports local-only; an .mcp.json of {command: "npx", args: ["tuhdoo", "mcp"]} (and the node_modules/.bin/tuhdoo direct path) serves the ten verbs to a harness; the wrapper adds nothing to stdout/stderr of the shim path; package versions match the git tag; docs updated (README install section; agent-protocol.md Connecting mentions the npx form works unchanged).

Pointers: the release pipeline task's artifacts; cmd/tuhdoo/mcp_cmd.go (stdio shim — the thing the JS shim must not garble); esbuild's npm packaging as prior art.

Constraints: same workflow-file law as the release task — .github/workflows/ changes get called out separately for Brandon's eyes-on review. The Go binary stays the single source of behavior; the JS layer is a dumb launcher, never a reimplementation. Publishing credentials/npm scope setup is Brandon's to provision — escalate for the token rather than inventing a workaround.

## History

_No activity yet._
