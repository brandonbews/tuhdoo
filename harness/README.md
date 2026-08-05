# harness/

Experiments that exercise tuhdoo against itself. These are **not** tests:
`make test` never runs them, CI never runs them, and they take minutes, not
milliseconds. They are here because some properties of a distributed system
can only be shown by running two of it.

Each harness is a plain `package main` under `harness/<name>/`, so
`go build ./...` and `go vet ./...` cover it and nothing else changes.

## collision — two-machine convergence

```
go run ./harness/collision
```

Flags: `-rounds` (claim rounds, default 10), `-confirm-storm` (deliberate
claim collisions whose `confirm_claim` verdicts are raced from both
machines, default 40), `-storm` / `-storm-gap` (the eager-write burst aimed
at the sync loop), `-converge-timeout` (default 5m), `-keep` (leave the
scratch repos behind for inspection).

### What it proves, and why it exists

The v1 definition of done asks for evidence that cross-machine convergence
holds. The hole is concrete: the D3 set-union merge path — divergent
histories merged by application logic, a two-parent commit via `commit-tree`,
deterministic view regeneration — never runs on a solo-dogfooded ledger,
because a single machine never diverges from itself.

Two clones on one box is a faithful stand-in for two machines. The machine
id is minted per repository directory (`internal/daemon/daemon.go`
`machineID`), so the clones get distinct ids, and event ordering is ULID
order — no wall clock is ever trusted across machines (T3).

The run, end to end:

1. builds `./cmd/tuhdoo` into a temp dir (it never trusts `bin/tuhdoo`);
2. makes a scratch bare origin and two clones under a short `/tmp` path —
   the daemon's socket lives at `<repo>/.git/tuhdoo/daemon.sock` and macOS
   caps socket paths at 103 bytes, so the default `/var/folders/...` TMPDIR
   would blow the limit (the same trap `npm/smoke.sh` documents);
3. seeds a pool of tasks that need no work — claiming one *is* the run;
4. opens one MCP session per clone by spawning `tuhdoo mcp --as <principal>`
   and speaking MCP over its stdio. The claim lifecycle is session-only by
   design (T7): there is deliberately no one-shot `tuhdoo claim`, so a
   scripted actor has to hold a session, and holding it is what keeps the
   leases renewed;
5. storms the D6 confirmation gate: both actors deliberately claim the
   same task (`claim_task` behind a barrier — claims are optimistic and
   judged locally, so both succeed: a certain collision), then race
   `confirm_claim` from both sessions at once. The remote's ref CAS is the
   referee; exactly one `claim.confirmed` may land per contest, and a
   contest where both actors are told "confirmed" fails the run on the
   spot. Winners record `done`; losers alternate between the two honest
   exits — reporting `finish_run(done)` and being coerced to `superseded`
   (branch kept), or standing down via `release_claim` and being closed by
   replay's branch-less synthesized run;
6. fires `claim_next` from both actors behind a barrier, once per round;
7. storms the sync loop with simultaneous eager writes on both machines;
8. lets the two machines converge, then settles the claim rounds through
   the public verbs alone — every fate is discovered from a daemon's
   answer (`confirm_claim` answering lost, or `finish_run(done)` refereed
   through the gate), never decided by the harness reading state — and
   verifies. The harness writes no outcome on any daemon's behalf.

### How to read the output

The `== verification` block is the acceptance list. Every line is
machine-checked; `[FAIL]` on any of them exits non-zero.

| Line | What it establishes |
| --- | --- |
| identical event sets | set-union merge lost nothing |
| byte-identical replayed state | the deterministic core agrees on both machines (both replays use one `Now`, since lease expiry is evaluated against it) |
| byte-identical generated views | D3's "all machines converge to byte-identical views" |
| identical data-branch trees | the strongest single check: one tree hash covers stored events *and* generated views |
| at least one real merge commit | the two-parent `commit-tree` path actually executed |
| no task carries more than one `claim.confirmed` | D6 clause 2's at-most-one-confirmation, counted straight off the stored events — a duplicate is a hard failure at any probability |
| every storm contest: one confirmation, one done, one superseded | the confirmation-race storm settled every deliberate collision through the real gate |
| claim races observed | both actors claimed the same task in the same round |
| claims voided by the winner rule | replay called the race, i.e. `core.ClaimVoided` |
| exactly one survivor — the confirmed claim, else earliest ULID | the refereed D6 rule re-derived from the replayed claims, not trusted; the storm makes confirmations that out-rank an earlier ULID actually happen |
| every race crossed machines | winner and loser carry different machine ids |
| every verb-discovered fate matches replay | what each daemon told its agent (done / lost) is what the ledger says |
| every done is certified | a `done` run's claim carries its `claim.confirmed` — no uncertified `done` exists |
| reporting losers coerced, branch kept | D6 clause 3's real coercion: `finish_run(done)` on a lost attempt recorded `superseded` with the reported branch as salvage |
| silent losers closed by synthesis | D6 clause 3's other arm: a stand-down with no report is closed by replay's branch-less synthesized `superseded` run |

`[note]` lines are things the acceptance asks to be *reported* rather than
passed — currently only the `maxCycleRetries` clause.

`== numbers` is the measurement record. Read it carefully:

- **non-fast-forward pushes** is `syncer.Status.Collisions`. It counts push
  contention, **not** claim collisions. Nothing anywhere counts voided
  claims; this harness derives them from replayed state.
- **app-level merges built** is `syncer.Status.Merges`, which increments
  when a merge commit is *constructed*. A merge whose ref update or push
  then loses a race is discarded, so fewer merge commits land on the branch
  than were built. The "merge commits on data branch" line is the landed
  count (`git rev-list --merges --count`).
- **pushes losing the ref lock** is a shape of push failure the daemon does
  not classify as contention at all — see the findings below.

`== FINDINGS` only appears when the run hit something worth writing down.

### Why it takes minutes

T8's cadence: the fetch interval is 60s, and only claim and escalation
writes push eagerly. The harness's one lever on that is an eager write, and
an eager write is exactly what has to *stop* before the two machines can be
still at the same moment — so the convergence waits are quiet waits, one
fetch interval per hop. A full run is roughly three to five minutes, most of
it spent waiting on purpose.

### Observed run (2026-08-03, defaults, macOS)

```
claim rounds fired            10
claim races observed          10   (every round contested)
claims made                   20
claims voided (D6 losers)     10
merge commits on data branch  2
non-fast-forward pushes       11   (alpha 0, bravo 11)
app-level merges built        13   (alpha 0, bravo 13)
pushes losing the ref lock    1    (alpha 0, bravo 1)
maxCycleRetries exhausted     1    (9.1% of non-fast-forward pushes)
```

134 events, byte-identical replayed state (~50 kB) and 18 byte-identical view
files on both machines, on an identical data-branch tree. All ten acceptance
checks passed. Wall time about four minutes, three of them quiet convergence
waits. Two independent runs on the same day produced the same figures on every
line — the per-round race is reliable, not lucky.

### Findings from running it (2026-08-04, driving the real D6 machinery)

The first run of the harness against the revised D6 machinery (PRs #28/#30)
left the convergence checks, the winner rule, the coercion arm, and the
zero-duplicate-confirmations storm all green — and turned up two real gaps
in the silent-loser arm, both downstream of the same design decision:
release-by-a-voided-claimant *deletes the loser's lease file* so that
replay synthesizes the superseded close immediately.

**Deleting a voided claim's lease rewrites history when the loser held the
earlier claim.** `leaseExpiredBy` (`internal/core/replay.go`) counts a
*missing* lease as lapsed at every instant, including past ones. When a
stand-down's lease deletion lands for a loser whose claim was the earlier
ULID — which is precisely the contests where the confirmation gate
out-ranked the provisional rule, the new machinery's marquee case — every
future replay re-adjudicates the winner's claim-time: the incumbent loser
now looks lease-less, so it is recorded `expired` with a synthesized
`interrupted` run, the confirmation binds via the ordinary
provisional-winner arm, and the promised `superseded` run never exists.
Deterministic, converged, and permanent on both machines — but the ledger
says `interrupted` where the verb told the agent "recorded as superseded"
(13 of 40 storm contests in the observed run).

**The union merge resurrects deleted lease files, and that now matters.**
The merge comment (`internal/syncer/merge.go`) accepts resurrection with
the rationale "a resurrected lease only matters to an ACTIVE claim, and
active claims never had their lease deleted" — written before the D6
revision made a *voided* claim's lease deletion load-bearing. When the
peer's next merge unions a lease-bearing head back in, the synthesized
close vanishes again until the resurrected lease's natural 15-minute
expiry (5 of 10 still-voided silent losers were un-closed at verify time
in the observed run; the other half's deletions happened to survive). The
state self-heals after the TTL, but "the attempt is closed now" is not
true cross-machine, and whether a stand-down closes immediately is a
merge-timing coin flip.

Until those are resolved, a default run exits red on three checks (the
storm-contest record shape, fate-vs-replay agreement, and silent-loser
synthesis); everything else — including byte-identical convergence and
exactly one `claim.confirmed` per contest — passes. The harness asserts
the design's promises, not the implementation's current behavior, on
purpose.

### Findings from running it (2026-08-03)

**The daemon never writes the `superseded` run D6 promises.** *(Resolved
2026-08-04: this finding triggered the confirmation-gate grill that revised
D6 — the daemon now referees every finish, coercing a lost attempt's report
to `superseded` with its links kept, and replay synthesizes a branch-less
`superseded` close for a loser that never reports. The harness no longer
plays the losing daemon's part: `POST /v0/runs` is not called at all, and
every outcome it checks was written through the public verbs. The original
text is kept below as the record of what was found.)* D6 clause 2
said "the losing daemon tells its agent to stand down; half-done work is
recorded as a Run with outcome `superseded` (branch name included)". Replay
voided the loser's claim (`internal/core/replay.go`, the D6 winner rule) and
that is where it stopped: the only run replay ever synthesized was
`interrupted`, for lease expiry. The MCP surface rejected `superseded` from
agents on the grounds that it is "daemon-synthesized", and no daemon code
synthesized it. The only surface that accepted it was `POST /v0/runs` on
the daemon's unix socket, so the shape was anticipated in code but had no
writer, and the harness played the part D6 assigned to the losing daemon. A
real fleet at the time would have left voided claims with no run at all.

**D6's "machine-id tiebreak" is vacuous.** The winner rule as implemented is
"earliest event ID wins": replay sorts events by ULID and the first claim to
land on a task holds it. ULIDs are unique — the daemon mints them from
`ulid.Monotonic` with a crypto/rand reader — so two claims can never tie and
the machine id is never consulted. The harness asserts the rule it can
actually observe (the surviving claim is the lexicographically smallest on
its task) and separately checks that winner and loser sit on different
machines. The doc clause is harmless but describes a branch that does not
exist.

**The bounded push-retry loop does not survive sustained symmetric eager
writes.** `syncer.maxCycleRetries` is 4. Under the storm phase — 40 eager
writes per machine, both machines at once, roughly 4/s each — a daemon
reports `remote origin kept moving for 4 attempts` and its work stops
reaching the remote for the length of the burst, because the peer's eager
pushes keep winning the ref. Nothing is lost: the ledger converges as soon
as the burst stops. But while it lasts, one machine is invisible to the
other, which is precisely the window D6 races open in. Note the load is
unrealistic by design — a real fleet's claims and escalations arrive per
minute, not several per second — so this is a ceiling measurement, not a
bug report. Run with `-storm-gap` to find where it starts.

**With two peers, one of them does all the merging.** Whichever daemon
lands its push first owns the remote ref; the other must fetch, merge, and
push a commit that has the winner's tip as a parent — after which the
winner is strictly behind and fast-forwards, never merging. A marginal,
consistent speed difference between the two machines is therefore enough to
put every app-level merge on one side (in the run above: alpha 0, bravo 13).
The merge path is exercised, but only ever by the slower peer. Nothing here
is wrong; it is worth knowing before reading a single machine's `merges`
counter as a fleet-wide number.

**Some push contention is not counted as contention.** When two peers push
to the same bare repo at the same instant, git can reject one with
`cannot lock ref 'refs/heads/tuhdoo': is at X but expected Y` — a lost
ref-update race rather than a stale history. `gitx.Push` classifies a
rejection as `ErrNonFastForward` only when git's porcelain output contains
"non-fast-forward" or "fetch first" (`internal/gitx/cli.go`), so this shape
returns a generic error: `Syncer.Cycle` returns instead of going around its
retry loop, the daemon records `mode=error`, and `Status.Collisions` does
not count it. The next cycle recovers, so nothing is lost — but the
push-contention counter T8 says the daemon keeps is an undercount.

### Safety

The harness only ever touches scratch repositories it creates under `/tmp`,
and refuses to start if its clone paths are not under that prefix. It never
looks at the repository it is run from beyond building the binary, and it
only ever SIGTERMs daemons it spawned itself. Nothing it does can reach this
repo's `tuhdoo` data branch.
