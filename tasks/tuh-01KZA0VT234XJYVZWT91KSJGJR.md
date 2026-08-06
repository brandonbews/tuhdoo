# Multi-repo story: does a plan ever span repos, or is that explicitly out of scope?

`tuh-01KZA0VT234XJYVZWT91KSJGJR`

- **Status:** cancelled
- **Priority:** 0
- **Labels:** `design`
- **Created:** 2026-08-05 22:31 UTC by `brandon/claude-code-1`

## Description

Migrated from open-questions.md (onboarding thread), 2026-08-05 sweep. Cross-repo identity and edges would be format-touching; per-machine supervisor (v2+ parked) is the adjacent read-side idea.

Cancelled at the 2026-08-06 grill (Brandon): cross-repo plans are explicitly out of scope as mechanism. Reasoning: (a) mechanical edges across ledgers conflict with D4's repo-write trust boundary and the founding definition's repo scope — an edge into another repo's ledger dangles for anyone without that repo and spans two committer sets; (b) the read-side want (see many fabrics at once) is already parked in the v2+ pointer as supervisor/cross-project dashboard; (c) the practical need — one team, several repos — is served today by prose: a task in one repo's ledger may describe work in another repo via durable string pointers, exactly like PR links under T2; the fabric's home and the work's location are already decoupled. Which repo hosts the fabric is the monorepo-grain question (tuh-01KZA0VT234XJYVZWT8YFV8XE2), which keeps its own thread. Write-side edges now have a named shelf in the v2+ pointer (tuh-01KZA0VT234XJYVZWT98B7NXEH) should adopter demand ever appear.

## History

_No activity yet._
