# Go codebase sweep: duplicate mechanisms, comment fluff, and a test-suite spec audit (zero behavior diffs)

`tuh-01KZ9YBF1N06FQ37XV65940SSG`

- **Status:** open — in progress, claimed by `brandon/claude-code-bg`
- **Priority:** 0
- **Labels:** `polish` `go`
- **Created:** 2026-08-05 21:47 UTC by `brandon`

## Description

Context: Grilled with Brandon 2026-08-12; every rule below is his decision — do not re-litigate. The codebase is ~14.4k lines of non-test Go and ~15.9k lines of test Go across internal/*, cmd/tuhdoo, and harness/collision, built fast by agent fleets; this is the first dedicated hygiene pass. Scope is all Go code (test and non-test) and nothing else — docs/site prose was swept by tuh-01KZSBDXFZCRNEDY7DMD4XGP75 (PR #71); internal-docs and workflows are out of scope. SPIRIT: an audit with cleanup authority, not a cleanup quota. "It's clean" is a legitimate outcome; do not manufacture diffs to justify the claim. Target bar: a Go expert walks in and feels at home. Learner accommodations are explicitly NOT a goal — Brandon's Go-learning collateral is a separate parked capture (tuh-01KZ9Z6647C3TBCYGGTXQJYE8V).

SEQUENCING: the vocabulary rename (tuh-01KZT571HQ3JEDEA5FFXSHBAJP, priority 1) lands FIRST — it edits comments in the same files. Do not start this sweep while that task is open or in flight.

The ask — one lens ("does every line earn its place?"), two jobs:
1. Fluff sweep over all Go code, test and non-test: reimplemented mechanisms, dead code, superfluous comments, filler.
2. Test-spec audit: for each test, judge — does it pin behavior we DESIGNED (001/002 decisions, protocol contracts, task acceptance criteria), or does it merely mirror whatever the code produced when it was written? Tests are the executable spec; a test that mirrors the code protects nothing.

PRIME LAW — zero behavior diffs, categorically. App code may be edited only in provably behavior-neutral ways (dedup, dead-code deletion, comment pruning). Every test edit must go green against untouched app code, first try; a test edit that requires any app change is out of bounds by definition. The disaster mode this forbids: rewriting a test to match your reading of the design, watching it go red, and "fixing" the app — that is a regression smuggled in through the suite.

Rules:
- Comments: WHY stays (constraints, invariants, design-decision pointers, warnings); WHAT goes (line narration, signature restatement, review-artifact commentary, commented-out code). Doc comments on exported identifiers stay — Go idiom. If deletion makes a passage genuinely harder to audit, the fix is clearer code, not a comment.
- Dedup: dedup KNOWLEDGE, never COINCIDENCE. Extract at 3+ occurrences, or 2 where divergence would be a bug — into a plain named function where it naturally lives. Never a util package, never generics for their own sake, never an interface invented to unify two call sites, never a bool-flag merger. Idiomatic Go duplication is fine. In tests: dedup setup freely; never DRY assertions past the point a failure tells you what broke.
- Discrepancies are findings, not fixes. Test contradicts design, or app behavior looks wrong: capture a new tuhdoo task with the evidence (test name, design clause, observed behavior) and move on. Whether app or docs are right is a human call, outside this task.
- Gaps: local gap-fills in scope (new table rows, one new test function pinning a nearby invariant — green first try); structural gaps ("lease expiry has no coverage") become new task captures.
- No new tooling. Run any analyzer locally to find candidates; nothing added to repo, Makefile, or CI. If findings suggest golangci-lint is warranted, that's a new capture with evidence.

Execution (recommended, per CLAUDE.md fan-out pattern): findings-first fan-out — one reader per package cluster, pointed at CLAUDE.md + relevant design sections + this description, mandate "report findings, change nothing." Each reports dedup candidates WITH a one-line mechanism summary (cross-package reimplementation is spotted at the orchestrator by comparing summaries), comment kill-lists, dead code, and per-test spec-or-mirror verdicts. Orchestrator synthesizes, applies edits, owns the PRs, and is sole holder of the prime law.

Acceptance:
- PR 1: non-test sweep. PR 2: test sweep + audit. Captures filed as found, not batched. If a diff is huge, split by package; if the audit is mostly green, don't manufacture PRs.
- make test lint green; per-PR zero-behavior-diff evidence in the closing run summary (suite green against untouched app code before test edits, and after).
- Closing summary states honestly, per package: what was cut, what was judged clean, and every capture filed.

## History

### 2026-08-11 21:21 UTC — edit by `brandon`

retitled

### 2026-08-12 20:37 UTC — edit by `brandon/claude-code-1`

retitled · description edited · status inbox→open · priority 0→0 · labels +polish +go

### 2026-08-12 20:53 UTC — edit by `brandon/claude-code-1`

description edited

### 2026-08-12 21:49 UTC — note from `brandon/claude-code-bg`

Findings-first fan-out in flight: six read-only readers by package cluster (internal/daemon; cmd/tuhdoo TUI half; cmd/tuhdoo CLI half; internal/core+event; internal/store+gitx+syncer+views; harness/collision), mandate report-only. Orchestrator synthesizes and applies. Intended branches: tuh-40ssg/go-sweep-app (PR 1, non-test) and tuh-40ssg/go-sweep-tests (PR 2, test sweep + audit), both off main at/after a8cbcdc (the verbs->tools rename this sweep was sequenced behind — already landed). No edits exist yet; if interrupted before branches push, restart from the fan-out.
