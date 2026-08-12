# One vocabulary: "tools" replaces "verbs" everywhere, "verb-time" becomes "call-time" (prose only, zero behavior)

`tuh-01KZT571HQ3JEDEA5FFXSHBAJP`

- **Status:** open — ready
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
