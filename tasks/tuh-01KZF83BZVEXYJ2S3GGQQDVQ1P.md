# Root .gitignore: ignore OS junk (.DS_Store and friends)

`tuh-01KZF83BZVEXYJ2S3GGQQDVQ1P`

- **Status:** open — in progress, claimed by `brandon/claude-code-1`
- **Priority:** 2
- **Labels:** `repo-hygiene`
- **Created:** 2026-08-07 23:14 UTC by `brandon/claude-code-1`

## Description

Context: Brandon hit .DS_Store noise accessing the repo from another Mac — the root .gitignore only contains /bin/, so Finder metadata shows up as untracked clutter and risks being committed by a broad git add. Nothing is tracked yet on main or the data branch (verified 2026-08-07).

The ask: add standard OS metadata entries to the root .gitignore (.DS_Store, AppleDouble ._*, Thumbs.db for future Windows peers). Keep it minimal and boring; site/ tooling ignores stay in site/.gitignore.

Acceptance: git status stays clean when .DS_Store files exist anywhere in the tree; make test lint green; one PR per repo conventions.

Constraints: no workflow files; do not restructure existing ignores.

## History

_No activity yet._
