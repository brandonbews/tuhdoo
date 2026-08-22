# LsTree silently accepts symlink tree entries; its fail-don-t-skip arm only fires on gitlinks

`tuh-01KZW1WKW1MKWKJQ360Y5AR40S`

- **Status:** inbox — untriaged capture
- **Priority:** none
- **Labels:** `audit-finding`
- **Created:** 2026-08-12 22:36 UTC by `brandon/claude-code-bg`

## Description

Go-sweep audit finding (tuh-01KZ9YBF1N06FQ37XV65940SSG, PR 2). internal/gitx/cli.go ~270-273 rejects non-blob entries with a comment claiming coverage of submodule and symlink — but git ls-tree -r types 120000 symlink entries as blob (verified empirically), so the fields[1] != "blob" arm never fires on symlinks; only 160000 gitlink entries trip it (now pinned by TestLsTreeRejectsNonBlobEntries). A data-branch tree containing a symlink is silently accepted and its target path read as content. Decide: reject mode 120000 explicitly (behavior change — out of the zero-behavior sweep) or accept and fix the comment.

## History

_No activity yet._
