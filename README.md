# tuhdoo

A coordination fabric for agent fleets, steered by humans.

tuhdoo gives your coding agents a shared backlog, work queue, and activity
ledger — the durable record of every claim, note, and outcome. All of it
lives on the **data branch**: an orphan git branch named `tuhdoo` (a branch
with its own history, carrying coordination data instead of code) inside
the repo it plans. It syncs over the git remote you already have — no
server, no vendor, no accounts.

## Install

tuhdoo is a single static binary for macOS and Linux; pick whichever
install path suits you. Windows is not supported — on a Windows machine,
run tuhdoo inside WSL (Windows Subsystem for Linux), where it is an
ordinary Linux program.

### npm, pnpm, or yarn (recommended for TypeScript and JavaScript projects)

```sh
npm i -D tuhdoo      # or: pnpm add -D tuhdoo
                     # or: yarn add -D tuhdoo
npx tuhdoo init
```

The `tuhdoo` package is a thin launcher: the real binary ships in a
per-platform `@tuhdoo/*` package that your package manager selects through
the packages' `os` and `cpu` fields — the esbuild pattern, with no
postinstall downloads. npm, pnpm, and yarn all resolve those
platform-specific optional dependencies correctly. Installing tuhdoo as a
devDependency pins the binary version per project through your lockfile.

The Model Context Protocol (MCP) shim — the command agents connect
through, spoken over standard input/output (stdio) — runs via `npx`
unchanged:

```json
{ "mcpServers": { "tuhdoo": { "command": "npx", "args": ["tuhdoo", "mcp"] } } }
```

### Release binaries

Every tagged release publishes checksummed archives for `darwin/arm64`,
`darwin/amd64`, `linux/arm64`, and `linux/amd64` at
<https://github.com/brandonbews/tuhdoo/releases>. Download the archive for
your platform, verify it, and install the binary:

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

To confirm the install, run `tuhdoo version`: it prints the release tag.

### `go install`

```sh
go install github.com/brandonbews/tuhdoo/cmd/tuhdoo@latest
```

Binaries built this way print `tuhdoo dev` instead of a release tag — the
release pipeline stamps the version, not the Go toolchain.

### From source

```sh
git clone https://github.com/brandonbews/tuhdoo
cd tuhdoo
make build   # → bin/tuhdoo
```

## Get started

1. Run `tuhdoo init` from anywhere inside the repository you want to plan.
   It starts the per-repo daemon, creates the data branch — or adopts the
   one your remote already carries — and prints the MCP snippet for
   connecting agents.
2. Run `tuhdoo` to open the interactive TUI (terminal user interface).
   `tuhdoo --watch` is the same screen read-only — safe to leave open in a
   pane.
3. Connect your agent harness — the tool that runs your agents — through
   the MCP shim:

   ```json
   { "mcpServers": { "tuhdoo": { "command": "tuhdoo", "args": ["mcp"] } } }
   ```

   Agents then follow the loop in
   [`docs/agent-protocol.md`](docs/agent-protocol.md); `tuhdoo protocol`
   prints that same text straight from the binary, ready to wire into a
   host repo's agent instructions.

For quick commands outside the TUI: `tuhdoo status`, `tuhdoo backlog`,
`tuhdoo task <id>`, and `tuhdoo escalations` read the ledger; `tuhdoo
create`, `tuhdoo update`, and `tuhdoo answer` write to it, no agent
session required.

## Join a repo, or leave one

Joining a repository that already uses tuhdoo — a teammate's project, a
second machine — takes a clone and one `init`.
[`docs/joining.md`](docs/joining.md) is the end-to-end walkthrough,
including how to clone and the branch-protection settings the repo admin
needs. Leaving is just as clean: [`docs/uninstall.md`](docs/uninstall.md)
walks a machine — or a whole team — away with zero trace.

## Learn more

The full documentation lives in [`docs/`](docs/README.md).

## Private notes

The maintainer keeps the design decisions, the plan and the rules agents
follow for this project in a private repo, `brandonbews/tuhdoo-internal`,
cloned into this checkout as `internal-docs/`. Its `tuhdoo` branch is also
the ledger this repo plans its work with; [`.tuhdoo`](.tuhdoo) names it.
This repo ignores `/internal-docs`, `/CLAUDE.local.md` and
`/.claude/commands/drain-backlog.md`, and everything builds and tests the
same without them. To set up a computer with it:

1. Clone this repo, then the private one inside it, as `internal-docs`
   (not `internal`, which the Go packages own):
   `git clone https://github.com/brandonbews/tuhdoo-internal.git internal-docs`.
2. Run `sh internal-docs/scripts/install-hooks.sh`, and again after moving
   either checkout. It's safe to rerun, and it refuses to replace a file or
   a hook it didn't write.
   - It links `CLAUDE.local.md` here to `internal-docs/agent-rules.md`, so
     Claude Code loads those rules in this checkout and in worktrees under
     `.claude/worktrees/`, and links `.claude/commands/drain-backlog.md` to
     the private copy of the `/drain-backlog` command.
   - It installs a `post-checkout` hook that links `internal-docs` and the
     command into every new worktree (and `CLAUDE.local.md`, for a worktree
     outside this checkout).
