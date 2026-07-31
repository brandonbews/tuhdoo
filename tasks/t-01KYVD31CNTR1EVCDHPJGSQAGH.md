# t-01KYVD31CNTR1EVCDHPJGSQAGH — Align MCP tool descriptions with the revised notes doctrine

- Status: done
- Priority: 1
- Labels: `protocol`, `docs`, `mcp`
- Created: 2026-07-31 06:16 UTC by `brandon/claude-code-2`

## Description

Context: the 2026-07-30 protocol revision (docs/agent-protocol.md; 002 T5 note) reframed add_note as optional garnish — continuity across agents is carried by the typed transition events (claim, finish_run, release_claim, escalate, synthesized interrupted run). Five live strings still teach the old 'letters to the next agent/incarnation' doctrine, and tool descriptions are what agents actually read, so the live surface now lags the doc: internal/daemon/mcp.go (~line 30 server instructions; ~344 add_note input schema description; ~439 release_claim description; ~505-507 add_note tool description) and internal/event/catalog.go (~134 NoteAdded doc comment).

The ask: revise those strings to match the revised doctrine — notes optional, written when they would save a successor real work; transitions carry the record. Keep the imperative voice and keep descriptions short (they ride every tools/list response).

Acceptance: no 'letter(s) to the next' phrasing remains in live tool text or the catalog comment; mcp_cli_test string assertions updated where they pin old text; make test lint green.

Constraints: text only — no schema, verb, or behavior changes; stored events untouched.

## History

### 2026-07-31 08:21 UTC — run by `brandon/claude-fable` — done

Commit 5f417d0 on main. All five stale strings revised to the 2026-07-30 doctrine: mcpInstructions (transitions are the record, add_note optional), add_note input schema description, add_note tool description, release_claim description (the reason is the handoff), and the NoteAdded doc comment in internal/event/catalog.go. grep confirms no letter-to-the-next phrasing remains in live code; mcp_cli_test pins only tool names, so no test edits were needed; make test lint green. Left alone deliberately: 002-technology.md (quotes the old framing inside its revision note) and open-questions.md historical entry. The new text ships in tools/list at the next daemon rebuild+restart, done immediately after this run closed.

### 2026-07-31 08:22 UTC — run by `brandon/claude-fable` — done

Commit 5f417d0 on main. All five stale strings revised to the 2026-07-30 doctrine: mcpInstructions (transitions are the record, add_note optional), add_note input schema description, add_note tool description, release_claim description (the reason is the handoff), and the NoteAdded doc comment in internal/event/catalog.go. grep confirms no letter-to-the-next phrasing remains in live code; mcp_cli_test pins only tool names, so no test edits were needed; make test lint green. Left alone deliberately: 002-technology.md (quotes the old framing inside its revision note) and open-questions.md line 9 (historical entry — arguably stale now that the protocol doc exists, but curating open-questions is human work). NOTE: this text ships in tools/list, so it goes live at the next daemon rebuild+restart — happening right after this finish_run.

### 2026-07-31 08:24 UTC — note from `brandon/claude-fable`

Ledger correction: run 01KYVM8KY10HP90YNBXMQP8ZCV is an accidental duplicate of 01KYVM7700YAJX8HP2JCSJG5CW — repro traffic mistakenly sent to the live daemon while investigating a shim crash. Its one useful effect: it exposed that the daemon accepts finish_run from a principal holding no claim (note the empty claim ref on the duplicate). Filed as its own task.
