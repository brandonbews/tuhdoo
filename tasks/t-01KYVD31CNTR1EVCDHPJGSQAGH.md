# t-01KYVD31CNTR1EVCDHPJGSQAGH — Align MCP tool descriptions with the revised notes doctrine

- Status: open — ready
- Priority: 1
- Labels: `protocol`, `docs`, `mcp`
- Created: 2026-07-31 06:16 UTC by `brandon/claude-code-2`

## Description

Context: the 2026-07-30 protocol revision (docs/agent-protocol.md; 002 T5 note) reframed add_note as optional garnish — continuity across agents is carried by the typed transition events (claim, finish_run, release_claim, escalate, synthesized interrupted run). Five live strings still teach the old 'letters to the next agent/incarnation' doctrine, and tool descriptions are what agents actually read, so the live surface now lags the doc: internal/daemon/mcp.go (~line 30 server instructions; ~344 add_note input schema description; ~439 release_claim description; ~505-507 add_note tool description) and internal/event/catalog.go (~134 NoteAdded doc comment).

The ask: revise those strings to match the revised doctrine — notes optional, written when they would save a successor real work; transitions carry the record. Keep the imperative voice and keep descriptions short (they ride every tools/list response).

Acceptance: no 'letter(s) to the next' phrasing remains in live tool text or the catalog comment; mcp_cli_test string assertions updated where they pin old text; make test lint green.

Constraints: text only — no schema, verb, or behavior changes; stored events untouched.

## History

_No activity yet._
