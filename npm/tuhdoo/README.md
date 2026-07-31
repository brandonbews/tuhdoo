# tuhdoo

A coordination fabric for agent fleets, steered by humans: a shared backlog,
work queue, and activity ledger living in a git orphan branch inside the repo
it plans. Synced through an ordinary git remote — no server, no vendor, no
accounts.

This package is a thin launcher: the real tuhdoo is a single static Go
binary, installed for your platform via one of the `@tuhdoo/*`
optionalDependencies. Installing as a devDependency pins the binary per
project through your lockfile.

```sh
npm i -D tuhdoo
npx tuhdoo init      # inside the repository you want to plan
npx tuhdoo status
```

Agents connect through the stdio MCP shim:

```json
{ "mcpServers": { "tuhdoo": { "command": "npx", "args": ["tuhdoo", "mcp"] } } }
```

Docs, design record, and source: <https://github.com/brandonbews/tuhdoo>
