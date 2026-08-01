# Inbox and held: capture without scoping pressure, pause without pretending

`t-01KYVJ5NABEKX72KNE006MBBMC`

- **Status:** done
- **Priority:** 1
- **Labels:** `core` `mcp` `cli` `tui` `design`
- **Depends on:** [`t-q5ev`](t-01KYVD31CNTR1EVCDHPGZFQ5EV.md) (done)
- **Created:** 2026-07-31 07:45 UTC by `brandon/claude-code-8`

## Description

Context: grill cycle, 2026-07-31 (Brandon + session). The queue prices every entry at commission rates — prompt-quality description, priority, scope — which pressures idea capture into premature quantification. Decisions, all confirmed: two new task statuses joining open/done/cancelled. `inbox` = never triaged, carries inherent review debt, the chuck-it-in tier. `held` = passed triage, workable, deliberately paused (absorbs and kills the parked-p0-label convention). `open` remains the only status claim_next serves. Transitions are mechanically permissive — no rules in the deterministic core (T3: no rejected-event edge cases) — with semantics carried by the protocol doc: promote inbox→open by supplying a prompt-quality description; pause/resume open↔held; anyone (human or agent) may promote, the quality bar is documentation not schema. Inbox/held items are ordinary shared state on the data branch. Capture minimum is title-only; fragment descriptions are legitimate FOR INBOX ITEMS ONLY; labels and edges allowed at capture (deps on inbox/held tasks block naturally — they are not done); priority stored but inert until open.

The ask:
1. Core: StatusInbox and StatusHeld in internal/core; classify/readiness treats only open as claimable; replay stays pure with table-driven tests.
2. Events (T3): additive evolution; decide whether new status values in existing task.created/task.updated payloads need a schema version bump so OLD binaries fail safe (read-only mode) rather than mis-bucketing unknown statuses — verify what an old binary actually does with an unknown status string and close that hole; stored bytes never rewritten.
3. MCP: create_task items gain an optional status field accepting inbox (and held?) — decide whether creating directly into held is allowed (recommend yes, permissive); update_task status accepts the new values. Verb count unchanged (T5) — fields, not verbs. claim_next and claim_task must never serve inbox/held (test this explicitly).
4. get_backlog: separate inbox and held arrays alongside ready/etc so agents orient without claiming.
5. One-shot + TUI: Held and Inbox render as dim bottom sections (Held above Inbox); rows enter detail normally; archive (c) works on both; blocked-task waiting: reasons annotate deps that are inbox/held via the existing taskRef status annotation. NO TUI promotion — promotion is an agent/CLI conversation.
6. TUI quick-capture: a key (suggest `i`) opens a single-line input that creates an inbox item (title only) as the steering actor; armed mode only — fully absent in --watch; y/n-free (capture is reversible via archive); model-driven tests per top_test.go patterns.
7. Docs, dated revision notes in place: status model where it is defined (001/002 as appropriate), T5 note for the get_backlog shape and create_task field, agent-protocol.md guidance: capture cheap, promote deliberate, the tasks-are-prompts bar applies at promotion not capture; promoting to sneak past scoping = same failure as writing a bad task.
8. Migration (after deploy): demote the two parked tasks — t-01KYRMFV10W1N28TCN62RR3A4D (unix-only daemon portability) and t-01KYT63MB28Z535SMJCBC7SY1P (tree/parent-grouped TUI rendering) — to held via update_task, prepending their gate to the description (e.g. 'Gated: unpark when a Windows user or long-path repo materializes'); remove the parked label; the parked convention is dead.

Acceptance: claim_next/claim_task never return inbox/held tasks (tested); capture via MCP with title-only works and syncs; promote/pause/resume round-trips work via update_task; TUI shows both sections dim at bottom with quick-capture creating a real inbox task end to end; one-shot sections updated with exact-format tests adjusted deliberately; old-binary fail-safe behavior for new statuses verified and documented; docs revised with dated notes; make test lint green.

Constraints: boring Go; deterministic core stays pure; no new MCP verbs; stored event bytes untouched; no transition enforcement in replay.

## History

### 2026-07-31 15:48 UTC — run by `brandon/claude-code-1` — done

- Branch: `main`
- Commits: `238ce7e`

Landed in commit 238ce7e; step 8 (demoting the two parked tasks to held) executes post-deploy at this drain session's wrap-up. Core: StatusInbox/StatusHeld constants; only open is claimable; replay validates vocabulary via knownStatus, never transitions; NewReplayer now auto-registers catalog upcasters so no caller can forget them. EVENT DECISION: task.created gained a status field (empty = open, the v1 reading) and task.created/task.updated bumped to v2 with identity upcasters. Verified empirically: a v1-only binary decoding status:"inbox" drops the unknown field and mints the task OPEN AND CLAIMABLE (silent mis-bucketing), so "additive fields never bump" doesn't apply — a field whose absence implied a default its presence contradicts is breaking in additive clothing. At v2, old binaries hit the upcast gate: "schema version above this binary's — upgrade tuhdoo", fail-safe read-only (proved live with a pristine-tree binary against v2 bytes: DEGRADED status, 0 ready, writes rejected 503). Accepted consequence documented in T3: once any peer writes v2, un-upgraded peers go read-only. MCP: create_task items take optional status open/inbox/held (create-into-held permitted — permissive by design; born-done/cancelled rejected); update_task accepts all five; get_backlog returns inbox and held arrays (creation order); claim_next/claim_task never serve shelved tasks (tested at HTTP and MCP layers). One-shot + views + TUI: Held above Inbox as dim bottom sections; rows enter detail; c archives both; blocked reasons annotate shelved deps; views FormatVersion 3. TUI quick-capture: `i` in armed panes opens single-line input creating a title-only inbox task as the steering actor; absent in --watch; y/n-free. Docs: dated notes in 001 D5, 002 T3/T5/T7, agent-protocol.md (capture cheap, promote deliberate; the prompts bar applies at promotion). E2E: scratch daemon verified capture→sync→promote→claim round-trips and the old-binary fail-safe; scratch cleaned up; live daemon untouched. make test lint green. FLAGGED FOR BRANDON: (1) the mixed-fleet consequence — first v2 write puts un-upgraded peers read-only; conditional versioning was judged too clever. (2) replay accepts born-done/cancelled in task.created payloads (write surfaces reject them) — consistent with no-rejected-event-edge-cases. NOTE: live daemon still runs pre-inbox code until wrap-up deploy; do not create inbox/held tasks via this session's MCP before then (it would be rejected).
