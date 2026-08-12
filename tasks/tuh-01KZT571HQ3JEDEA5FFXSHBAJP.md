# One vocabulary: "tools" replaces "verbs" everywhere, "verb-time" becomes "call-time" (prose only, zero behavior)

`tuh-01KZT571HQ3JEDEA5FFXSHBAJP`

- **Status:** done
- **Priority:** 1
- **Labels:** `docs` `polish` `go`
- **Created:** 2026-08-12 04:55 UTC by `brandon/claude-code-1`

## Description

Context: Brandon settled "tools" over "verbs" for published surfaces at the copy-tightening escalation (01KZSKPJ9JHFBKQEJK89JQFHWA); PR #71 aligned docs/ and site/. Grilled 2026-08-12: Brandon chose FULL DEPTH — one vocabulary everywhere, citing the 2026-08-01 status-vocabulary precedent (a dual vocabulary costs more than the softer word earns). Confirmed on the merits too: these are literally MCP tools (tools/call); "verb" was house shorthand, and the reads (get_task, get_backlog) never fit it anyway.

The ask: replace "verb(s)" with "tool(s)" as the term for the twelve MCP tools, across:
- The two agent-visible strings in internal/daemon/mcp.go: the get_backlog tool description (~line 423, "which claim verbs never serve") and the warning-field jsonschema string (~line 390, "losers learn at verb-time").
- CLAUDE.md's T5 law line ("Fewer, better MCP verbs").
- internal-docs: T5's heading "MCP verb surface" and all body uses; 001 D6 and every other occurrence — WITH revision notes per convention, dated 2026-08-12.
- All code comments and test names: ops.go, top.go, mcp.go, api.go, harness/collision, *_test.go (e.g. TestMCPConfirmClaimVerb, TestVerbTimeStandDownNotices).

The D6 coinage "verb-time" is renamed "call-time" (Brandon, 2026-08-12): same concept — no push channel to a working agent; a race loser learns it lost at its next tool call — same rhetorical role, defined where D6 defines it today.

Do NOT touch (correct English, not the house term): textinput.go's verb parameter (the submit word); event.go's "noun.verb" event-type naming; every "verbatim"; the TUI mockup fixtures under internal-docs/design/mockups/ (frozen historical artifacts, some titled with "verb").

Constraints: prose/identifier-comments only — zero behavior diffs; renaming a test function is fine, changing what it asserts is not. make test lint green; one PR. Land BEFORE the Go sweep (tuh-01KZ9YBF1N06FQ37XV65940SSG) starts — this task is priority 1 for exactly that reason; the sweep's readers should meet the settled vocabulary.

## History

### 2026-08-12 20:52 UTC — edit by `brandon/claude-code-1`

retitled · description edited · status inbox→open · priority 0→1 · labels +go

### 2026-08-12 21:46 UTC — run by `brandon/claude-code-1` — done

- Branch: `tuh-shbajp/verbs-to-tools`
- PR: <https://github.com/brandonbews/tuhdoo/pull/76>
- Merged as: `a8cbcdcbc84c5095e1f64b17dbe411dba2e0ef48`

Vocabulary sweep landed via PR #76 (squash a8cbcdc): "tool(s)" replaces "verb(s)" for the twelve MCP tools across the two sanctioned agent-visible strings in internal/daemon/mcp.go, CLAUDE.md's T5 law, internal-docs (002 T5 heading now "MCP tool surface"; 001 D6 amended with dated revision notes per convention), all code comments, and test names (TestMCPConfirmClaimVerb→TestMCPConfirmClaimTool, TestVerbTimeStandDownNotices→TestCallTimeStandDownNotices; harness const verbRetries→toolRetries). "verb-time" → "call-time" everywhere, defined in D6 clause 3. Untouched by design: textinput.go's submit-word param, noun.verb event naming, "verbatim", frozen TUI mockups, CLI-subcommand senses ("verb-less TUI", TestHelpDocumentsWriteVerbs), and 7 harness/smoke diagnostic string literals (string changes beyond the two sanctioned were out of scope; no test asserts their text — a follow-up could sanction them if full unification is wanted). 73 insertions/73 deletions, zero behavior diffs, make test lint green. The Go sweep (tuh-01KZ9YBF1N06FQ37XV65940SSG) can now start against settled vocabulary.
