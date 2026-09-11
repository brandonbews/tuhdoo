# Priority is clearable: task.updated v4, clear on the API, MCP, and CLI

`tuh-01M28YZVWS1PJEMG9J7QN11DRX`

- **Status:** open — ready
- **Priority:** 1
- **Labels:** `go` `event` `core` `daemon` `mcp` `cli`
- **Created:** 2026-09-11 19:26 UTC by `brandon/claude-code-1`

## Description

Context: a set priority cannot be cleared anywhere, by anyone, through any surface — and the MCP, HTTP, and CLI help all say so. The cause is the wire shape: task.updated (v3) uses null on every field to mean "unchanged", so "clear" and "leave alone" are the same bytes. Brandon put an accidental p2 on tuh-01KZVZT7F8CVJYX1P00BRPGMTX and could not undo it; that is the bug this fixes. Decided 2026-09-11 (keyboard/priority grill): priority becomes clearable across the whole stack — the steering-parity rule (002 T7) means the TUI, CLI, and MCP all get it, and this task is the non-TUI half. The TUI picker lands in a later task that depends on this one.

The ask:
1. Event schema, additive-first (T3): bump task.updated to v4 in internal/event/catalog.go Versions. In v4 the priority key is tri-state: absent = unchanged, explicit null = clear, integer = set. Every other field keeps v3 semantics (null = unchanged). Register a v3→v4 upcaster in internal/core/upcast.go beside zeroPriorityToNull that deletes a null priority key from a v3 payload in memory (v3 null meant unchanged), leaving every other byte alone. Stored bytes are never rewritten. The Go struct must encode v4 faithfully: absent must stay absent on the wire (a plain *int with json:"priority" emits null — choose the boring fix: a small tri-state value type with MarshalJSON/UnmarshalJSON, or route priority through the raw map the way Unknown already does). Canonical re-encoding (internal/event/canonical.go) must stay byte-identical for v3 and v4 events.
2. Core replay (internal/core/replay.go, the task.updated arm): a v4 explicit null sets t.Priority = nil and records the field change as "priority pN→none" via priorityLabel. Pure, table-tested.
3. Daemon: updateTaskReq (internal/daemon/ops.go) gains `clear_priority bool` (json clear_priority, omitempty). Sending both priority and clear_priority is a 400. opUpdateTask emits a v4 event with an explicit null when clear_priority is set. The HTTP PATCH /v0/tasks/{id} handler needs no change beyond the struct. The daemon writes v4 for every task.updated from now on.
4. MCP update_task (internal/daemon/mcp.go): add the optional boolean input `clear_priority` (a field, not a tool — the T5 count stays twelve, the same precedent as the status field). Rewrite the priority field's description: drop "A set priority cannot be cleared back to unprioritized" and say clear_priority clears it.
5. CLI (cmd/tuhdoo/write_cmds.go): `tuhdoo update <id> --priority none` sends clear_priority. Keep --priority <n> as the set form. Rewrite the usage text to say so; delete the "cannot be cleared" clause. `tuhdoo create` is unchanged (omit = none already).
6. Docs: docs/agent-protocol.md and docs/steering.md wherever they state or imply priority cannot be cleared — search for "clear" and "unprioritized"; fix only actual statements, add no new rules.

Acceptance (tests must exist and pass):
- internal/event: a v4 task.updated golden fixture (testdata) round-trips byte-identically through decode → canonical encode for all three priority states (absent, null, integer); a v3 fixture still round-trips unchanged.
- internal/core table test: v3 payload with "priority": null replays as unchanged; v4 absent replays as unchanged; v4 null clears a set priority and the Fields entry reads "priority p2→none"; v4 integer sets. An event at version 5 fails replay with ErrCannotReplay (existing fail-safe test pattern).
- internal/daemon: PATCH with clear_priority:true clears; with both priority and clear_priority returns 400; the emitted event is v4 with an explicit null priority key; an update that does not touch priority emits a v4 event with no priority key.
- MCP: update_task with clear_priority true clears (existing mcp test harness); the tool count remains twelve (existing test).
- cmd/tuhdoo: `update <id> --priority none` sends clear_priority (CLI golden/request test); `--priority 3` still sends priority: 3; usage text golden updated.
- Manual, in the PR body: after deploy, run `tuhdoo update tuh-01KZVZT7F8CVJYX1P00BRPGMTX --priority none` and paste the `tuhdoo backlog` row showing PRI `-`. That is the accidental p2 this task exists for.
- `make test lint` green; PR title = task title; body opens with this task ID.

Pointers: internal/event/catalog.go (Versions, TaskUpdated, the v3 comment above Versions), internal/event/canonical.go, internal/event/golden_test.go and testdata/, internal/core/upcast.go (registerCatalogUpcasters, zeroPriorityToNull, upcast), internal/core/replay.go (the task.updated arm around line 264, priorityLabel), internal/daemon/ops.go (updateTaskReq, empty(), opUpdateTask), internal/daemon/mcp.go (the update_task input struct near line 407 and the call near 622), internal/daemon/api.go (handleUpdateTask), cmd/tuhdoo/write_cmds.go (runUpdate: the set map and usage const), 002 T3 (additive-first, upcasters, fail-safe) and T5 (fields, not tools).

Constraints: stored event bytes never rewritten (T3); no new MCP tool (T5); boring Go — no reflection tricks for the tri-state; replay stays pure; the TUI is untouched here (its picker is tuh task "TUI keymap rework", which depends on this). Deploy after landing per CLAUDE.md; the restart kills live MCP sessions. Add a revision note to 002 T3's v3 paragraph recording v4 and its reason.

## History

_No activity yet._
