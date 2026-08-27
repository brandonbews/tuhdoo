# LsTree: reject symlink (120000) tree entries — blobs-only means regular blobs

`tuh-01KZW1WKW1MKWKJQ360Y5AR40S`

- **Status:** done
- **Priority:** none
- **Labels:** `go` `storage` `audit-finding`
- **Created:** 2026-08-12 22:36 UTC by `brandon/claude-code-bg`

## Description

Context: Go-sweep audit finding, re-verified 2026-08-27. internal/gitx/cli.go:269-273 rejects non-blob entries with a comment claiming submodule AND symlink coverage — but `git ls-tree -r` types mode-120000 symlink entries as "blob", so the check fires only on 160000 gitlinks; the mode field (fields[0]) is parsed and discarded (TreeEntry carries no mode, cli.go:274), so no downstream consumer can reject one either. A data-branch tree containing a symlink is silently accepted and its target path string read as content. The 2026-08-12 test sweep documented the gap in TestLsTreeRejectsNonBlobEntries's comment (gitx_test.go:167-169) without fixing it, so the test comment and the code comment now contradict each other. Direction decided by the code's own stated posture ("The data branch holds blobs only; anything else means the tree is not ours — fail, don't skip"): reject.

The ask: check the mode field — accept only regular blob modes (100644, 100755); reject 120000 and anything else unexpected with the same fail-don't-skip error shape, naming the offending path and mode. Update the cli.go comment to match reality; extend TestLsTreeRejectsNonBlobEntries with a real symlink entry.

Acceptance: a test commits a symlink into a tree and asserts LsTree fails loudly; the gitlink case stays pinned; cli.go and gitx_test.go comments agree with the code; make test lint green.

Constraints: T3 fail-safe — reject, never skip-and-continue.

## History

### 2026-08-27 07:06 UTC — edit by `brandon/claude-code-1`

retitled · description edited · status inbox→open · labels +go +storage

### 2026-08-27 09:16 UTC — run by `brandon/claude-code-2` — done

- Branch: `tuh-r40s/lstree-symlink`
- PR: <https://github.com/brandonbews/tuhdoo/pull/101>
- Merged as: `541b56d`

Landed via PR #101 (squash 541b56d). LsTree now checks the mode field after the type check — regular blob modes 100644/100755 only — so symlink (120000) entries, which git ls-tree types as blob, reject loudly naming path and mode instead of being read as content. cli.go and test comments agree with the code; TestLsTreeRejectsNonBlobEntries is a two-row table (gitlink type pin kept, real symlink entry via raw mktree). make test lint green. Binary changed: rebuilt and daemon restarted post-finish.
