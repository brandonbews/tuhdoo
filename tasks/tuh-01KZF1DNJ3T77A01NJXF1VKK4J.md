# Docs swap: root docs/ becomes the published content root; working docs move to internal-docs/

`tuh-01KZF1DNJ3T77A01NJXF1VKK4J`

- **Status:** done
- **Priority:** 2
- **Labels:** `docs` `onboarding`
- **Created:** 2026-08-07 21:17 UTC by `brandon/claude-code-1`

## Description

Child of the launch epic (tuh-01KZEPBEE8HFDQVK96AV6RN0ER). Immediately claimable — no dependency on the rest of the strategy grill.

Context: settled at the 2026-08-07 strategy-grill session (agenda item 5 of tuh-01KZEPBEE8HFDQVK96AQNCQF4G): root docs/ becomes the published doc content root, and the directory is the publish boundary — everything in it is public content, everything outside is internal. The full representation contract is recorded on tuh-01KZ9YD2Q9V7BSZJBDGVNYM6QY: GFM + YAML frontmatter restricted to title + description; cross-doc links are relative paths to real .md files; GitHub rendering is the semantic baseline.

The ask:
1. git mv docs internal-docs (design/, plan/, the tombstones, and the folder README stay inside it).
2. Create the new root docs/ and MOVE (never copy) the three public docs into it: joining.md, uninstall.md, agent-protocol.md.
3. Add title + description YAML frontmatter to those three (the first conforming files of the contract). No other keys.
4. Fix references: cmd/tuhdoo/uninstall_doc_test.go:24 (const "../../docs/uninstall.md"); CLAUDE.md (reading order + doc pointers); root README.md links; internal-docs/README.md's layout inventory (split its entries; give the new docs/ a minimal README.md index — the user-docs task tuh-01KZ9YD2Q9V7BSZJBDGVNYM6QY expands it later); Go comments citing docs/agent-protocol.md (internal/core/state.go, internal/daemon/mcp.go, harness/collision/main.go).
5. Ledger references (update_task, description edits only): tuh-01KZANB3J4YYH09F0Z6FSZQ5CD cites docs/agent-protocol.md; t-01KYRMFV10W1N28TCN5SH4QM7A cites docs/plan/roadmap.md. Leave stale paths in closed/cancelled tasks alone.

Acceptance: make test lint green — the uninstall doc-execution test proves the moved doc still runs verbatim from its new path; a repo-wide grep for the old docs/ paths (docs/design, docs/plan, docs/agent-protocol, docs/joining, docs/uninstall) finds no live reference (PR body lists any deliberately-left historical hits with why); the three public docs carry title+description frontmatter and their relative links click through correctly on GitHub; the two live ledger descriptions updated; one PR per repo conventions.

Constraints: moves, not copies — no doc content forked anywhere; .github/workflows/ untouched; the generated data-branch README's "design lives in docs/" sentence (internal/views/views.go:141) is OUT of scope — it has its own inbox capture from the same grill session.

## History

### 2026-08-07 21:47 UTC — edit by `brandon/claude-code-1`

priority 1→2

### 2026-08-07 21:55 UTC — run by `brandon/claude-code-1` — done

- Branch: `tuh-kk4j/docs-swap`
- PR: <https://github.com/brandonbews/tuhdoo/pull/52>
- Merged as: `4b18038a398bcb7b764d071bc66f23bbd3cd1529`

Landed on main as 4b18038 (PR #52, squash). docs/ → internal-docs/ via git mv; joining.md, uninstall.md, agent-protocol.md moved back into a new root docs/ (net: their paths unchanged in git, so the uninstall test's const needed no edit), each with title+description frontmatter — the first conforming files of the representation contract — plus a minimal docs/README.md index (the user-docs task tuh-01KZ9YD2Q9V7BSZJBDGVNYM6QY expands it later). CLAUDE.md, root README, and internal-docs/README updated; both live ledger descriptions (tuh-01KZANB3J4YYH09F0Z6FSZQ5CD, t-01KYRMFV10W1N28TCN5SH4QM7A) updated via update_task. Deliberately-left old-path hits (tombstone git-log commands, roadmap historical annotation, golden event bytes per T3, views.go:141 out-of-scope sentence) are listed with reasons in the PR body. make test lint green; CI green; no binary change, no daemon restart needed.
