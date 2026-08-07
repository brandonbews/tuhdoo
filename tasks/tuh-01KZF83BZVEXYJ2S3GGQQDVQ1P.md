# Root .gitignore: ignore OS junk (.DS_Store and friends)

`tuh-01KZF83BZVEXYJ2S3GGQQDVQ1P`

- **Status:** done
- **Priority:** 2
- **Labels:** `repo-hygiene`
- **Created:** 2026-08-07 23:14 UTC by `brandon/claude-code-1`

## Description

Context: Brandon hit .DS_Store noise accessing the repo from another Mac — the root .gitignore only contains /bin/, so Finder metadata shows up as untracked clutter and risks being committed by a broad git add. Nothing is tracked yet on main or the data branch (verified 2026-08-07).

The ask: add standard OS metadata entries to the root .gitignore (.DS_Store, AppleDouble ._*, Thumbs.db for future Windows peers). Keep it minimal and boring; site/ tooling ignores stay in site/.gitignore.

Acceptance: git status stays clean when .DS_Store files exist anywhere in the tree; make test lint green; one PR per repo conventions.

Constraints: no workflow files; do not restructure existing ignores.

## History

### 2026-08-07 23:16 UTC — run by `brandon/claude-code-1` — done

- Branch: `tuh-vq1p/gitignore-os-junk`
- PR: <https://github.com/brandonbews/tuhdoo/pull/57>
- Merged as: `17c373c573d419a46857e851eabd356c61d3f349`

Landed on main as 17c373c (PR 57, squash). Root .gitignore gained the OS-metadata trio (.DS_Store, AppleDouble ._*, Thumbs.db) alongside the existing /bin/. Slash-less patterns match at every depth — verified by planting .DS_Store files at root, docs/, site/, and internal-docs/design/: git status stayed clean and check-ignore attributed all to the new rule. Nothing was actually tracked on main or the data branch, so no history cleanup was needed; Brandon’s other machine just needs a pull. No binary change, no daemon restart needed.
