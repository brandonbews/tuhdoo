# tuhdoo

A coordination fabric for agent fleets, steered by humans: a shared backlog,
work queue, and activity ledger living in a git orphan branch (`tuhdoo`)
inside the repo it plans. Synced through an ordinary git remote — no server,
no vendor, no accounts.

## Install

tuhdoo is a single static binary; pick whichever path suits you.

### npm (recommended for TS/JS projects)

```sh
npm i -D tuhdoo
npx tuhdoo init
```

The `tuhdoo` package is a thin launcher; the real binary ships in a
per-platform `@tuhdoo/*` package that npm selects via `os`/`cpu` fields
(the esbuild pattern — no postinstall downloads). Installing as a
devDependency pins the binary version per project through your lockfile.
The MCP stdio shim runs through it unchanged:

```json
{ "mcpServers": { "tuhdoo": { "command": "npx", "args": ["tuhdoo", "mcp"] } } }
```

### Release binaries

Every tagged release publishes checksummed archives for
`darwin/arm64`, `darwin/amd64`, `linux/arm64`, and `linux/amd64` at
<https://github.com/brandonbews/tuhdoo/releases>.

```sh
# example: Apple Silicon macOS — set TAG to the latest tag from
# https://github.com/brandonbews/tuhdoo/releases/latest
TAG=v0.x.y
curl -LO "https://github.com/brandonbews/tuhdoo/releases/download/$TAG/tuhdoo_${TAG}_darwin_arm64.tar.gz"
curl -LO "https://github.com/brandonbews/tuhdoo/releases/download/$TAG/checksums.txt"
shasum -a 256 --check --ignore-missing checksums.txt
tar -xzf "tuhdoo_${TAG}_darwin_arm64.tar.gz"
install -m 755 tuhdoo /usr/local/bin/tuhdoo
```

`tuhdoo version` on an extracted binary prints the release tag.

### go install

```sh
go install github.com/brandonbews/tuhdoo/cmd/tuhdoo@latest
```

Note: binaries built this way print `tuhdoo dev` — the version is stamped
by the release pipeline, not by the Go toolchain.

### From source

```sh
git clone https://github.com/brandonbews/tuhdoo
cd tuhdoo
make build   # → bin/tuhdoo
```

## Start here

Run `tuhdoo init` inside the repository you want to plan, then bare `tuhdoo`
for the interactive TUI (`tuhdoo --watch` is the same screen read-only —
safe to leave open in a pane). One-shot commands cover reads (`status`,
`backlog`, `task <id>`, `escalations`) and session-free writes (`create`,
`update`, `answer`). Agents connect through the stdio MCP shim:

```json
{ "mcpServers": { "tuhdoo": { "command": "tuhdoo", "args": ["mcp"] } } }
```

See `docs/agent-protocol.md` for the agent loop.

Joining a repository that already uses tuhdoo — a teammate's project, a
second machine? `docs/joining.md` is the end-to-end walkthrough, including
clone shapes and the branch-protection settings the host needs. Leaving is
just as clean: `docs/uninstall.md` walks a machine — or a whole team —
away with zero trace.

The design record lives in `internal-docs/` — start with
`internal-docs/design/001-core-design.md`.
