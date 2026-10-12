# tuhdoo

A coordination fabric for agent fleets, steered by humans: a shared backlog, work queue, and activity ledger on a git orphan branch, synced through an ordinary git remote. One Go binary (`cmd/tuhdoo`) carries the daemon, the MCP surface, the CLI and the TUI; `site/` is the website.

## Building and testing

- `make build` writes `bin/tuhdoo`.
- `make test lint` runs the Go tests and `go vet`, and builds, lints and typechecks the site. It must be green before a commit. In a fresh worktree, run `npm ci` in `site/` first.
- The site in `site/` has its own commands and rules: `site/AGENTS.md`.

## Docs

`docs/` is published content: the site renders it, and the binary embeds `docs/agent-protocol.md` (`tuhdoo protocol`). Everything in it is public. It is GitHub-flavored Markdown with relative links, and its frontmatter holds only `title` and `description`.

## Maintainer notes

`internal-docs/` is a gitignored clone of the private `tuhdoo-internal` repo; its README covers setup. It may be missing, and that's fine.
