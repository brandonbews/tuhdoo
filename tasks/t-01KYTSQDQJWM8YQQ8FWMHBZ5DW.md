# Short IDs are the human contract: display everywhere, accept as input, annotate edges

`t-01KYTSQDQJWM8YQQ8FWMHBZ5DW`

- **Status:** done
- **Priority:** 1
- **Labels:** `cli` `tui` `ux` `design`
- **Created:** 2026-07-31 00:38 UTC by `4099114+brandonbews`

## Description

Context (steering, 2026-07-30): the TUI list shows short IDs (t-d83w = t- prefix + lowercase last-4 of the ULID tail) but the detail screen's depends_on/parents fields still show full ULIDs — two ID dialects in one UI. Worse, edges can point at done/cancelled tasks, which render no rows anywhere, so a full-ULID dep reference looks like dangling/corrupt data (Brandon hit this live: ready follow-ups depending on the completed Cycle-4 task t-01KYT63MB28Z535SMJC9B0D83W, visible nowhere). Decision: canonical IDs stay ULIDs — sequential IDs need a central counter the no-server design forbids, and ULID sort order is load-bearing (creation order, ID-derived timestamps). Instead, adopt the git model: the long ID is plumbing, the SHORT form is the human contract, display and input, consistently.

The ask:
1. Display: every human-facing task reference in the TUI uses the short form — detail screen depends_on/parents/task-title line, waiting: reasons, escalation task refs (list rows already do). The detail screen may show the full ID once (e.g. dimmed on its own line) as the copyable canonical form.
2. Annotate edges: dep/parent references resolvable from the snapshot render with status and title, e.g. `depends on t-d83w (done — One TUI: bare tuhdoo…)`; unresolvable IDs render bare — never invent status. This kills the dangling-edge illusion for hidden (done/cancelled) tasks.
3. Input: `tuhdoo task <id>` accepts a short form or any unambiguous ID fragment — resolve by unique suffix/substring match against known tasks; ambiguous → error listing candidates; full IDs keep working. (MCP verbs unchanged — agents pass full IDs from hydration JSON and never retype them.)
4. Document the contract with a small dated note in 002 T7 (display/input sugar; canonical IDs remain ULIDs; stored bytes untouched).

Acceptance: no full ULID visible in normal TUI screens except the detail screen's designated canonical line; dep/parent annotations covered by model-driven tests (resolved, done, unresolvable); `tuhdoo task t-d83w`-style lookup works with tests for unique, ambiguous, and unknown fragments; one-shot OUTPUT stays byte-identical (input resolution is additive); make test lint green.

Constraints: boring Go; display/input only — no event, API, or stored-ID changes; the blockedReasonDisp pattern (cmd/tuhdoo/snapshot.go) shows how TUI display diverges without touching one-shot output.

## History

### 2026-07-31 05:31 UTC — run by `4099114+brandonbews/claude-code-2` — done

- Branch: `main`
- Commits: `5bf8eaf`

All four asks landed. Display: TUI detail screen leads with the short form, shows the full ULID exactly once (dimmed canonical `id` line), and shortens parents/depends_on, waiting: reasons, and priority/cancel footers+status messages (API calls still send full IDs). Edge annotation: new snapshot.taskRef renders `t-d83w (done — old chore)` for any ID resolvable in the state listing — which does include done/cancelled tasks, killing the dangling-edge illusion; unresolvable IDs render bare, never an invented status; titles ellipsized at 40 runes. Input: `tuhdoo task <id>` resolves full IDs (outright), short forms, or any case-insensitive unambiguous fragment of the ULID tail (optional t- prefix); ambiguity errors listing each candidate as short + title + full ID; unknown keeps the "unknown task" contract. MCP verbs untouched. One-shot output verified byte-identical via a before/after fixture dump diff during development, pinned going forward by TestPrintTaskOneShotKeepsFullIDs plus the untouched exact-format CLI tests. Rendering is printTaskRef(ref) where nil ref = the old one-shot bytes and the TUI passes snapshot.taskRef. Model-driven tests cover resolved/done/cancelled/unresolvable refs, detail annotations, full-ULID-appears-once counting, and the resolver table; real-daemon CLI tests cover short-form lookup and ambiguous t-0. Contract documented as a dated paragraph in 002 T7 (canonical IDs remain ULIDs; display/input sugar only; stored bytes untouched).
