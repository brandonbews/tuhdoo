# tuhdoo

A coordination fabric for agent fleets, steered by humans: a shared backlog,
work queue, and activity ledger living in a git orphan branch inside the repo
it plans. Synced through an ordinary git remote — no server, no vendor, no
accounts.

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
# example: Apple Silicon macOS, release v0.1.0
curl -LO https://github.com/brandonbews/tuhdoo/releases/download/v0.1.0/tuhdoo_v0.1.0_darwin_arm64.tar.gz
curl -LO https://github.com/brandonbews/tuhdoo/releases/download/v0.1.0/checksums.txt
shasum -a 256 --check --ignore-missing checksums.txt
tar -xzf tuhdoo_v0.1.0_darwin_arm64.tar.gz
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
for the interactive TUI. Agents connect through the stdio MCP shim
(`command: tuhdoo, args: [mcp]`); see `docs/agent-protocol.md`.

The design record lives in `docs/` — start with
`docs/design/001-core-design.md`.
