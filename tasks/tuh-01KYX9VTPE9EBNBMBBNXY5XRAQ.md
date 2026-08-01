# Init flavor picker: multiple-choice workflow setup in `tuhdoo init` that drops recipe files

`tuh-01KYX9VTPE9EBNBMBBNXY5XRAQ`

- **Status:** archived
- **Priority:** 0
- **Labels:** `cli` `docs` `product`
- **Created:** 2026-07-31 23:58 UTC by `brandon/claude-code-1`

## Description

Gated: shares the gate of the workflow-recipes task (tuh-01KYX3KJYP7Y178GH5DJ6JPMRQ) — unpark for the same grill cycle once the v0 dogfood week has held AND this repo's trunk-based PR flow has been exercised on real tasks. Do NOT scope or build before that grill; this is a capture (2026-07-31), not a plan.

Captured idea (Brandon): after a successful `tuhdoo init`, ask a quick multiple-choice question offering a few flavors of working with tuhdoo — e.g. Brandon's trunk-based ticket→PR→squash flow, at least one other pattern, and always a "let me set it up on my own" exit. A selected flavor drops the couple of files needed to work that way into the host repo (e.g. an AGENTS.md/CLAUDE.md block, a vendorable skill file for the drain-the-backlog loop).

Relationship to recipes: this is a stronger delivery mechanism on the same axis. Floor = init emits links to recipe docs after success (precedent: init already emits CI guidance and a "Next:" line). Ceiling = this picker, which scaffolds files. The grill decides how far up the axis to go; the two tasks unpark together and this one may collapse into the recipes task or be deferred.

Tensions the grill must resolve:
- Dropping host-/harness-specific files (e.g. .claude/ artifacts) from a workflow-agnostic tool — recipes are suggestions, never baked into the protocol (agent-protocol step 3). File-dropping is still opt-in suggestion, but it's the tool writing opinionated files; where's the line?
- `tuhdoo init` is currently idempotent and non-interactive; a prompt needs a non-interactive bypass (flag, or default to link-emitting only when not a TTY) and must not break re-runs.
- No marketplace: flavors are files shipped in the tuhdoo binary/docs, copy-pasteable and vendorable — no server, no accounts, no curation platform (founding no-server/no-vendor principles).

## History

_No activity yet._
