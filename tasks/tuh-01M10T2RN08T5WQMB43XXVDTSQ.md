# User-facing copy rewrite: ISO 24495-1 backbone + Google dev-doc style (README, docs, npm, site)

`tuh-01M10T2RN08T5WQMB43XXVDTSQ`

- **Status:** open — in progress, claimed by `brandon/claude-code-2`
- **Priority:** 2
- **Labels:** `docs` `site` `copy`
- **Depends on:** [`tuh-364k`](tuh-01KZWX46MBVN8BHVMB7537364K.md) (done)
- **Created:** 2026-08-27 05:11 UTC by `brandon`

## Description

Context: Grill decision 2026-08-26 (Brandon). All user-facing prose gets rewritten — not audited into a findings list — to completely conform to a two-layer clear-writing standard. Layer 1 (structure) is the four principles of ISO 24495-1 (plain language), distilled into the rubric below because the standard's text is paywalled — apply the rubric, don't try to fetch the ISO document. Layer 2 (sentence style) is the Google developer documentation style guide (https://developers.google.com/style), which is public — consult it directly for anything the summary below doesn't settle. No linting/CI enforcement in this task; operationalizing conformance is deferred (Brandon decides later).

Scope — in: root README.md; everything under docs/ (recipes and agent-protocol.md included); npm/tuhdoo/README.md; site copy (site/src/app/page.tsx, not-found.tsx, opengraph-image.alt.txt, metadata strings in layout.tsx). Out: CLI help/usage/error strings, TUI strings, MCP tool descriptions (in-code microcopy — a possible follow-up task), and internal-docs/ (not user-facing).

Voice line: the homepage (page.tsx), root README, and npm README are pitch surfaces — brand voice is allowed there, but every clarity rule below still binds (no obscurity, jargon defined at first use, active voice). Everything under docs/ is a reference surface: strict conformance, no flavor exceptions.

The rubric — Layer 1, applied per document:
1. Relevance: the document opens by stating who it is for and what the reader will accomplish. Content the stated audience doesn't need is cut or moved.
2. Findability: information is ordered by reader need (task order, not implementation order); headings say what the section does for the reader; someone scanning headings alone can locate any topic the document covers.
3. Understandability: each sentence carries one idea; every project term is defined at first use or linked to its definition; no undefined jargon.
4. Usability: instructions are imperative steps in execution order; each step's success is observable; prerequisites appear before the steps that need them.

Layer 2 — the load-bearing Google style rules: second person ("you"); present tense; active voice (passive only when the actor is genuinely irrelevant); sentence-case headings; numbered lists for sequences, bullets for unordered sets; code font for commands, paths, and literals; abbreviations spelled out at first use; contractions are fine; no "please". The published guide is the referee for anything else.

The ask:
1. Rewrite every in-scope file to full conformance with both layers, respecting the voice line.
2. Keep terminology consistent across all surfaces: one set of project terms (task, claim, lease, escalation, data branch, ledger, view, principal) used identically everywhere.
3. This task depends on tuh-01KZWX46MBVN8BHVMB7537364K (pnpm/yarn install lines in docs) — rewrite the text that task lands too.

Hard constraints:
- docs/agent-protocol.md: protocol semantics are preserved exactly. Every normative statement — each must-shaped instruction, state name, tool name, and transition rule — survives with meaning unchanged; wording may improve, behavior may not. This file is embedded byte-for-byte in the binary (embed.go → `tuhdoo protocol`) and is the instruction text live agent harnesses load, so a semantic slip here changes fleet behavior on the next deploy. A test in cmd/tuhdoo pins the embed to the file; no lockstep work needed beyond editing the one file.
- docs/uninstall.md: the fenced shell blocks marked `<!-- uninstall-test: run -->` are executed verbatim by cmd/tuhdoo/uninstall_doc_test.go. Keep the blocks runnable; the prose around them is fair game. Test edits never force app changes (house rule) — if a block seems wrong, capture a task instead of editing it.
- Root docs/ conventions hold: GFM, frontmatter restricted to title + description, relative links, GitHub rendering as the semantic baseline.
- If headings or anchors change, verify cross-links between docs and the site nav (site/src/lib/nav.ts, prev/next ordering) still resolve.

Acceptance:
- Every in-scope file rewritten. The PR body carries a per-document conformance checklist against rubric items 1–4 plus the style layer — that checklist is the audit record.
- The agent-protocol.md diff is accompanied by a PR-body list of its normative statements confirming each is semantically unchanged.
- In agent-protocol.md, hard requirements and illustrative examples are unmistakably distinguishable after the rewrite: anything host- or workflow-specific (PRs, merge styles, branch names) appears only as a clearly marked example of one way to work, never phrased so it could be read as something tuhdoo requires. The dividing line the doc itself draws — protocol governs ledger writes, the code workflow is the adopter's — must survive sharpened, not blurred.
- `make test lint` green from the repo root (the uninstall doc test runs there); `cd site && npm run build` succeeds if site files changed.
- One or two PRs, agent's call: split into a strict docs/ pass and a pitch-surfaces pass if the diff warrants; each PR self-contained with its own checklist.

Pointers: README.md; docs/README.md, adopting.md, agent-protocol.md, joining.md, steering.md, uninstall.md; docs/recipes/*.md; npm/tuhdoo/README.md; site/src/app/page.tsx, not-found.tsx, opengraph-image.alt.txt, layout.tsx; embed.go; cmd/tuhdoo/uninstall_doc_test.go; https://developers.google.com/style.

## History

### 2026-08-27 06:39 UTC — edit by `brandon`

description edited
