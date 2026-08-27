# tuhdoo

A coordination fabric for agent fleets, steered by humans.

tuhdoo gives your coding agents a shared backlog, work queue, and activity
ledger — the durable record of every claim, note, and outcome — stored on
a git branch inside the repo it plans. It syncs over the git remote you
already have — no server, no vendor, no accounts.

This package is a thin launcher: the real tuhdoo is a single static Go
binary, and your package manager installs the build for your platform
(macOS or Linux) through one of the `@tuhdoo/*` optional dependencies.
Installing tuhdoo as a devDependency pins the binary version per project
through your lockfile.

## Install

```sh
npm i -D tuhdoo      # or: pnpm add -D tuhdoo
                     # or: yarn add -D tuhdoo
npx tuhdoo init      # inside the repository you want to plan
npx tuhdoo status
```

## Connect an agent

Agents connect through the Model Context Protocol (MCP) shim, spoken over
standard input/output (stdio). Add this to your agent harness's MCP
config:

```json
{ "mcpServers": { "tuhdoo": { "command": "npx", "args": ["tuhdoo", "mcp"] } } }
```

## Learn more

Docs, design record, and source: <https://github.com/brandonbews/tuhdoo>
