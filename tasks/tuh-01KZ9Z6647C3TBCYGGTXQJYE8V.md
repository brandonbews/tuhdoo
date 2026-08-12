# Codebase map + private Go reading companion: architecture doc in internal-docs, learning collateral as an artifact

`tuh-01KZ9Z6647C3TBCYGGTXQJYE8V`

- **Status:** open — ready
- **Priority:** 0
- **Labels:** `docs` `go`
- **Depends on:** [`tuh-0ssg`](tuh-01KZ9YBF1N06FQ37XV65940SSG.md) (done)
- **Created:** 2026-08-05 22:02 UTC by `brandon`

## Description

Context: Grilled with Brandon 2026-08-12; decisions are his — do not re-litigate. Brandon (a TS dev who has not yet read the Go in this repo) wants to use his understanding of tuhdoo's internals to accelerate learning Go itself. The deliverable splits in two: a public-safe codebase map, and a personal reading companion that stays out of the public repo. The depends_on edge on the Go sweep (tuh-01KZ9YBF1N06FQ37XV65940SSG) is deliberate: write against the cleaned codebase, not the pre-sweep one.

GROUND TRUTH RULE (Brandon, explicitly): both deliverables are written from a FRESH READ of the codebase as it stands when this task is claimed — after the vocabulary rename and the Go sweep have landed. Do the read yourself or fan out sub-agent readers per package (CLAUDE.md pattern); either way, every claim in the map and every idiom example in the companion must come from the current tree, never from the design docs' descriptions of the code, this description, or prior-session knowledge. The design docs govern WHY; only the tree says WHAT.

Deliverable 1 — the map: internal-docs/architecture.md (in-repo, one PR). Neutral professional register matching the rest of internal-docs; no learner framing anywhere. Three parts, in order:
1. One system mermaid diagram: surfaces (MCP shim, CLI, TUI) → daemon socket → ops layer → core/store/gitx/syncer → data branch + generated views. LAYERED AND LINEAR-ISH — subgraph per layer, edges flow one direction between adjacent layers, no spaghetti; a relationship that cannot be drawn without crossing layers goes in the trace prose, not the diagram. GFM + mermaid (GitHub renders natively).
2. One end-to-end trace: a single claim_next followed from MCP tool call → ops → event append → commit on the data branch → sync push → view regeneration.
3. Per-package sections, bounded (~15 lines each): role in one sentence, the 2-3 files that matter, key types, invariants owned, and a pointer to the governing design decision (core→T1, gitx→T2, event→T3, ...). Not exhaustive file inventories — depth lives in the design docs.

Header carries "snapshot of <date>; regenerate, don't patch" — a dated map, not a maintained contract; no future task should burn on keeping it current.

Deliverable 2 — the companion (NOT in the repo; published as a private claude.ai artifact, link recorded in the closing run summary). A guided reading course for this codebase, not a Go tutorial:
- Short on-ramp first: package/import mechanics, the go.mod dependency roster narrated (what each import buys and why it's mostly stdlib — a deliberate T1 posture worth a sentence), and the fundamentals the first reading stop needs, each with a TS mapping (structs vs objects, pointers vs references, error returns vs exceptions, zero values). Fundamentals earn inclusion by being needed at a stop, not for completeness.
- A reading order through the packages, sequenced so each stop only needs concepts already introduced — roughly event → core → store/gitx → syncer → daemon → TUI (Bubble Tea's Elm-ish loop lands last; it will feel familiar from React).
- Per stop: the package's job in one line (echoing the map); the 2-3 Go idioms met there for the first time, one sentence each with a TS contrast; and one "read this specific function and notice X" prompt to keep the reading active.
- Deliberately absent: setup/tooling chapters, syntax reference, anything this codebase doesn't exercise ("you won't meet channels here — that's T1, not an accident"). Sized for two or three evenings alongside the code.

Acceptance: the map lands as one PR (make test lint green — doc-only, but the gate is the gate); the companion is published as a default-private artifact and linked in the closing run summary; the closing summary tells Brandon where both live. Brandon's review of both is the real bar, post-merge — expect follow-up edits as he actually reads against them.

## History

### 2026-08-12 21:18 UTC — edit by `brandon/claude-code-1`

retitled · description edited · status inbox→open · priority 0→0 · labels +docs +go · depends_on +tuh-0ssg
