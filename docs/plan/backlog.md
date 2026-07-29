# v0 Backlog

The build-out as an ordered, dependency-aware task list. Written the way tuhdoo tasks should be written (descriptions are prompts: context, acceptance criteria, doc references) — dogfooding the convention on paper until the tool can hold it. **The final task migrates this file into tuhdoo itself.**

Conventions: tasks are `B1…Bn`; "Depends" lists hard prerequisites; a task is done when its acceptance criteria hold **and its tests pass**. Agents may decompose any task further, but record the decomposition here.

---

## B1. Repo scaffold

**Depends:** —
Go module (`go.mod`, Go ≥ 1.22), directory layout separating the pure core from I/O shells (suggestion: `internal/core` for pure functions, `internal/gitx` for the Git interface, `internal/daemon`, `cmd/tuhdoo`), `justfile` or `Makefile` with `test`/`lint`/`build`, CI running tests on push **with a path filter excluding the future data branch** (dogfoods T2's own onboarding guidance). No application logic.
**Accept:** `just test` runs green on an empty test; `go build ./cmd/tuhdoo` produces a binary that prints version.

## B2. Git interface + subprocess implementation

**Depends:** B1 · **Refs:** `002` T2
Define the small internal `Git` interface and its subprocess implementation: `HashObject`, `MkTree`, `CommitTree` (incl. two-parent), `UpdateRef` (atomic, with expected-old-value CAS), `CatFile`, `LsTree`, `Fetch`, `Push`, `RemoteURL`. No checkout operations exist in the interface *at all* — make the wrong thing unrepresentable. Plumbing only, `-z`/stable output forms, version floor check (git ≥ 2.40) with a clear error.
**Accept:** integration tests against throwaway repos (created in `t.TempDir()`) prove: write a tree of blobs to a branch that is never checked out; read it back byte-identical; CAS-failure on concurrent ref update surfaces as a typed error; fetch/push work against a local bare "remote".

## B3. Event model + canonical JSON

**Depends:** B1 · **Refs:** `002` T3
The envelope struct, ULID generation, canonical JSON encoder (sorted keys, fixed forms, UTF-8, no insignificant whitespace), and the v1 event-type catalog with payloads: `task.created`, `task.updated`, `claim.made`, `claim.released`, `run.finished`, `escalation.raised`, `escalation.answered`, `note.added`. Date-sharded path derivation from ULID. Unknown-field preservation on decode/re-encode.
**Accept:** golden-file tests: same logical event always encodes to identical bytes; decode→encode round-trips unknown fields; property test: path derivation is a pure function of ID.

## B4. Deterministic core: replay engine

**Depends:** B3 · **Refs:** `001` D5/D6, `002` T3
Pure function: set of events → full state (tasks with DAG edges, claims, runs, escalations, notes). Includes: ULID-lexical total ordering; the claim winner rule (earliest wins, later claims voided); lease-expiry evaluation at read time (expiry inputs passed in, never read from a clock inside the core); auto-close of runs orphaned by expiry as `interrupted`; upcaster framework (per-type version lifting, in-memory only); **fail-safe detection** — unknown type or `v` above comprehension returns a typed "cannot honestly replay" result, never a partial state.
**Accept:** table-driven tests covering: concurrent-claim races converge identically regardless of event insertion order (property: replay is order-insensitive on the input *set*); lease expiry returns tasks to pool; a single incomprehensible event fails the whole replay with the offending ID; upcasted old events produce current-shape state.

## B5. Storage: events on the data branch

**Depends:** B2, B3 · **Refs:** `002` T2/T3, `001` D9
Write batches of events to `refs/heads/tuhdoo` via the Git interface (no checkout), read/load the full event set, bootstrap the orphan branch on first init (empty tree root commit), and the `leases/` area as mutable files outside the event log (owning machine only). Commit debounce batching (2s default) lives here.
**Accept:** integration tests: init creates a parentless root commit; N event writes under debounce produce ≤ expected commits; full load returns exactly the written set; lease file overwrite does not touch `events/`.

## B6. Daemon skeleton: lifecycle + HTTP API

**Depends:** B4, B5 · **Refs:** `002` T4
Single-instance daemon (lockfile), Unix socket at `.git/tuhdoo/daemon.sock`, `daemon.json` discovery file, lazy auto-spawn from CLI invocations, serialized write loop (all writes through one goroutine — this *is* D2's machine-local serialization), JSON HTTP API exposing the same verbs as T5 plus human/admin verbs (answer escalation, cancel, reprioritize). No idle shutdown; exits always logged.
**Accept:** two concurrent clients hammering writes produce a linear, gap-free event history; killing the daemon mid-write leaves the branch consistent (partial batch either fully committed or absent); second daemon start attempt fails cleanly with a pointer to the live one.

## B7. Sync loop + app-level merge

**Depends:** B6 · **Refs:** `002` T2/T8, `001` D2/D6
Background loop: fetch (60s), push (eager on claims/escalations, ≤30s piggyback otherwise), **app-level merge** — union of event trees, two-parent `commit-tree`, no `git merge` ever — then view regeneration trigger. Remoteless mode: loop dormant, `local-only` status, remote detected on later cycles. Collision counters and sync latencies logged (T8's evidence-based tuning).
**Accept:** integration test with two working repos + one bare remote: divergent event writes on both sides converge to identical replayed state and identical branch tip trees after both sync; a claim race across the two resolves per D6 with the loser's claim voided; pulling the network cable (remote dir removed) degrades silently and recovers.

## B8. View generation

**Depends:** B4 · **Refs:** `002` T6
Pure functions: state → the four markdown views (`README.md`, `backlog.md`, `escalations.md`, `tasks/<id>.md`), byte-deterministic, view-format version stamp in `views/.meta`, highest-version-wins guard in the daemon's regeneration path. Committed alongside events by B5's writer.
**Accept:** golden-file tests per view; property: same state → same bytes; regeneration guard test: daemon refuses to overwrite views stamped newer, still writes events.

## B9. MCP surface

**Depends:** B6 · **Refs:** `002` T5
Streamable-HTTP MCP endpoint in the daemon (official Go SDK); the ten verbs exactly; `tuhdoo mcp` stdio shim with `--as <agent>` principal binding; session-bound lease auto-renewal (15 min TTL / 5 min renew) tied to MCP connection liveness; batch `create_task` with intra-batch `tmp:` ref resolution, atomic.
**Accept:** end-to-end test with a scripted MCP client: full loop (claim_next → add_note → finish_run) lands correct events with correct actor stamps; dropping the client connection expires the lease and auto-closes the run as `interrupted` within TTL; a 15-task DAG batch with tmp-refs lands atomically or not at all.

## B10. CLI portal

**Depends:** B6, B8 · **Refs:** `002` T7
`tuhdoo init` (orphan branch bootstrap, works remoteless, prints CI path-filter guidance), `status`, `backlog`, `task <id>`, `escalations` — rendered from daemon state, not from view files — and `watch`: Bubble Tea read-only auto-refreshing dashboard (the v1 TUI skeleton, zero input handling).
**Accept:** `init` on a fresh remoteless repo → `status` shows `local-only` as a normal state; all read commands work with daemon auto-spawn from cold; `watch` reflects a new event within the debounce+refresh budget.

## B11. Agent protocol doc

**Depends:** B9 (to be testable) · **Refs:** `002` T5, open-questions Cycle 3
Write `docs/agent-protocol.md`: the instruction text a harness loads — claim before working; checkpoint notes as letters to the next incarnation; escalate → note → release → `finish_run(blocked)` for blocking questions; always finish or release; task-description conventions (acceptance criteria, constraints, file pointers). Then field-test it: run a real harness session against a scratch project and record where the agent deviates; amend.
**Accept:** the doc exists; at least one recorded field-test session with deviations noted and folded back in.

## B12. Dogfood cutover

**Depends:** B10, B11
Initialize tuhdoo on this repo. Migrate this backlog (remaining tasks + roadmap intent) into tuhdoo via batch `create_task` — the plan-materialization flow from `002` T5, exercised for real. Replace this file's contents with a pointer to the data branch. Begin the v0 definition-of-done clock (one week of real use, no manual repair — `roadmap.md`).
**Accept:** `tuhdoo backlog` shows the migrated plan; this file is a tombstone pointing at the live system; the next development session is driven through `claim_next`.
