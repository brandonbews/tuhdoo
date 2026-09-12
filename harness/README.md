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

Flags: `-rounds` (claim rounds, default 10), `-spare` (extra seeded tasks
beyond the rounds so the claim_next pool never runs dry, default 3),
`-confirm-storm` (deliberate claim collisions whose `confirm_claim`
verdicts are raced from both machines, default 40), `-expiry-contests`
(deliberate collisions whose losers go silent so their leases lapse on
their own, default 2; 0 skips the arm), `-lease-ttl` (the claim lease TTL
the spawned daemons run, default 4m — see "Why it takes minutes" for the
lower bound), `-storm` / `-storm-gap` (the eager-write burst aimed at the
sync loop), `-converge-timeout` (default 5m), `-keep` (leave the scratch
repos behind for inspection).

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
5. runs the natural-expiry arm: deliberate collisions (the same shape as
   the storm's, below) whose losers then do nothing at all — no
   `finish_run`, no `release_claim`, the session stays open and silent.
   The loser's daemon learned "lost" at the gate and never renews a
   voided claim's lease, so the lease lapses one TTL after the claim and
   replay on every machine closes the attempt with a branch-less
   synthesized `superseded` run at that instant. The daemons run a short
   TTL for this (`-lease-ttl`, handed to them through the
   `TUHDOO_LEASE_TTL` environment variable, which `tuhdoo daemon` reads and
   which is unset everywhere else); the harness checks each claim's
   reported lease against the flag before building any wait on it;
6. storms the D6 confirmation gate: both actors deliberately claim the
   same task (`claim_task` behind a barrier — claims are optimistic and
   judged locally, so both succeed: a certain collision), then race
   `confirm_claim` from both sessions at once. The remote's ref CAS is the
   referee; exactly one `claim.confirmed` may land per contest, and a
   contest where both actors are told "confirmed" fails the run on the
   spot. Every confirmed verdict is asked again and must answer the same
   (confirmed, same claim — D6 calls it irrevocable). Winners record
   `done`; losers alternate between the two honest exits — reporting
   `finish_run(done)` and being coerced to `superseded` (branch kept), or
   standing down via `release_claim` and being closed by replay's
   branch-less synthesized run;
7. fires `claim_next` from both actors behind a barrier, once per round;
8. storms the sync loop with simultaneous eager writes on both machines;
9. waits for every claim to reach both machines, then settles the claim
   rounds through the public tools alone — every fate is discovered from
   a daemon's answer (`confirm_claim` answering lost, or
   `finish_run(done)` refereed through the gate), never decided by the
   harness reading state;
10. waits for the natural-expiry losers' leases to lapse and for both
    daemons to show the synthesized close through `GET /v0/snapshot` —
    the sessions, the losers' included, still open — recording how long
    after the lapse each daemon showed it;
11. closes the sessions, lets the two machines converge, and verifies. The
    harness writes no outcome on any daemon's behalf.

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
| every tool-discovered fate matches replay | what each daemon told its agent (done / lost) is what the ledger says |
| every done is certified | a `done` run's claim carries its `claim.confirmed` — no uncertified `done` exists |
| reporting losers coerced, branch kept | D6 clause 3's real coercion: `finish_run(done)` on a lost attempt recorded `superseded` with the reported branch as salvage |
| silent losers closed by synthesis | D6 clause 3's other arm: a stand-down with no report is closed by replay's branch-less synthesized `superseded` run |
| natural-expiry losers closed at the lapse, lease unreleased | D6 clause 3's never-reports arm: a loser that neither reported nor released is closed by replay's synthesized run once its lease lapses — no real run, no release event, and the lease file on both trees is a plain lease (not a released tombstone) lapsing where the daemon said it would. The arm the 2026-08-04 resurrection bug lived in |
| both daemons showed each natural-expiry close after the lapse | the daemons' own read surface showed the close (D6 clause 5's scheduled transition, on each machine), with the slowest observed lag |
| every claim response carried the confirm-before-merge warning | D6 clause 3's "every claim response carries the warning to confirm before merging", counted over every `claim_next` and `claim_task` answer of the run |
| every confirmed verdict answered the same when asked again | `confirm_claim` re-asked after every confirmed verdict: still confirmed, same claim (D6 clause 2: irrevocable and idempotent); the zero-duplicates line separately proves the repeat wrote nothing |

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
fetch interval per hop.

The natural-expiry arm adds the lease TTL to that floor: its losers' leases
lapse one `-lease-ttl` after the claim, and the run waits for it. The
default is 4m, and it cannot be much shorter. Lease renewals are ordinary
batched writes riding the 60s cycle, not eager pushes, so a peer's copy of
a live lease can trail by one renewal period (TTL/3) plus the batcher's 2s
quiet plus two 60s hops — the writer's push and the reader's fetch. The
TTL has to exceed TTL/3 + 122s, about three minutes, or a peer can see a
live claim as lapsed between renewals: the exact flapping T8's 15m/5m
ratio rules out in production. Four minutes clears the bound with a
margin. The arm's contests run first so the clock starts early; the post-
race wait is an events-only wait (both machines hold every claim) rather
than a tree comparison, because the still-open sessions renew their live
leases every TTL/3 and a renewal moves a tree — the acceptance-grade tree
check is the final convergence, taken with the sessions closed. A full run
is roughly six to seven minutes, most of it spent waiting on purpose.

### D6 arms: what runs here, what stays unit-covered

Covered end to end by this harness, through the public tools on two
daemons: the confirmation race (clause 2), the coerced report and the
stand-down synthesis (clause 3), the never-reports natural-expiry
synthesis (clause 3, with clause 5's scheduled transition), the
claim-response warning (clause 3), and the repeat-confirm stability
(clause 2).

Not run here, on purpose — they need one daemon and a remote that is
absent or unreachable, which two live clones of one origin cannot stage
without severing the very link the rest of the run measures — and covered
by `internal/daemon` unit tests instead (line ranges as of 2026-09-11):

- **remote configured but unreachable → honest retryable refusal, nothing
  written**: `gate_test.go:121-148`
  (`TestConfirmClaimUnreachableRemoteRefusesHonestly`);
- **remoteless confirmation is local, instant, idempotent**:
  `gate_test.go:85-115` (`TestConfirmClaimRemoteless`);
- **late-loser messaging** — a loser returning after its attempt was
  closed by synthesis is refused with the `add_note` salvage pointer, and
  the salvage note carries no stand-down nag: `loser_test.go:179-230`
  (`TestLateLoserFinishRejectedWithAddNotePointer`); the call-time
  stand-down notices on `add_note` and `escalate`, and the coerced finish
  through the MCP surface: `loser_test.go:334-375`
  (`TestCallTimeStandDownNotices`);
- the rule the natural-expiry arm leans on — a provisionally-voided claim
  stays tracked by its session but is never renewed:
  `loser_test.go:520-572` (`TestRenewOnceKeepsVoidedClaimsTracked`).

### Observed run (2026-09-11, defaults, macOS — the bounded extension)

```
claim rounds fired            10
claim races observed          10   (every round contested)
storm contests fired          40
claims made                   104
claims voided (D6 losers)     52
claim.confirmed on the branch 52   (0 duplicates; alpha 20, bravo 20)
winners recorded done         52
losers coerced on report      25
losers closed by synthesis    25
losers closed by lease lapse  2    (natural expiry, lease TTL 4m)
confirmations re-asked        47
merge commits on data branch  108
non-fast-forward pushes       103  (alpha 61, bravo 42)
maxCycleRetries exhausted     1    (1.0% of non-fast-forward pushes)
```

394 events, byte-identical replayed state (~122 kB) and 60 byte-identical
view files on both machines, on an identical data-branch tree. All twenty
hard checks passed. The two natural-expiry losers' leases lapsed 4m after
their claims with the sessions still open; both daemons showed the
synthesized close 500 ms after the lapse, and both trees carried the plain,
unreleased, unrenewed lease. Every one of the 104 claim responses carried
the confirm-before-merge warning, and all 47 re-asked verdicts answered
confirmed for the same claim. Wall time 4m08s (the storm now takes 29 s
for 40 contests, so the expiry wait overlaps what used to be quiet
convergence time rather than adding to it).

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

### Findings from running it (2026-09-11, the bounded extension)

The extension's first full run passed every new line — both natural-expiry
losers closed by replay at the lapse with the lease left unreleased and
unrenewed, both daemons showing the close 600 ms after it; 104 of 104 claim
responses carrying the warning; 46 confirmed verdicts stable on repeat — and
failed two of the forty storm contests on checks that had been green since
2026-08-04:

**A stand-down tombstone rounded to the second re-adjudicated the contest.**
*(Resolved the same day, in this change: `internal/store/lease.go` now stores
a tombstone's instant exactly — RFC3339 with fractional seconds, which every
existing reader already parses — while ordinary leases keep second
precision. The original text follows as the record.)* `encodeLeaseFile`
truncated every lease instant to the second, tombstones included. A plain
lease's expiry is a deadline and the sub-second is noise; a tombstone's
instant is the boundary replay judges past claims against — T8 promises
"live before the instant, lapsed from it onward" — and truncation moved that
boundary up to 999 ms into the past. In the two failing contests the loser
held the *earlier* ULID (the confirmation out-ranked it, the storm's marquee
case), stood down, and its release landed in the same wall-clock second as
the winner's claim; the truncated tombstone therefore read as lapsed *at* the
winner's claim instant, so `leaseExpiredBy(leases, incumbent, when)` found
the incumbent already gone when the winner's claim arrived: the loser was
recorded `expired` with an `interrupted` run, the confirmation bound through
the ordinary provisional-winner arm, and the promised `superseded` run never
existed — deterministic and converged on both machines, with the tool having
told the agent "recorded as superseded". The same shape as the 2026-08-04
deletion finding, one second wide instead of infinitely wide. It stayed
hidden until now because a storm contest used to take seconds; since the
2026-09-10 live-replica work (PRs #103–#105) one takes about 0.7 s, so a
stand-down now routinely lands in the winner's claim second. Not a
consequence of the shorter lease TTL: the truncated boundary is stored data
and re-adjudicates at every replay instant.

The port to `GET /v0/snapshot` is also from this session: the harness read
`GET /v0/state`, which the 2026-09-10 read-side revision retired (T4), so
before this change a default run could not get past joining the second
clone.

### Findings from running it (2026-08-04, driving the real D6 machinery)

*(All three findings below — the two lease gaps and the session-eviction
gap found while verifying their fix — are resolved 2026-08-04 by PR #32,
escalations 01KZ7W28PB9GPHM0CSQQ2QFABM and 01KZ88VCEP4AZ8CXY5DW1R72C6:
lease files are never deleted, only overwritten with a released tombstone
`{"expires": "<the instant>", "released": true}`; the lease merge rule
prefers tombstones; and the renewal tick keeps voided claims tracked
without renewing them. A run against the fixed tree exits 0 with all
16 hard checks green and the `maxCycleRetries` clause reporting as its
sanctioned `[note]` — 40 storm contests, one `claim.confirmed` each, 27
losers coerced on report, 23 closed by branch-less synthesis, 384 events
and byte-identical state and views on both machines. The original texts
are kept below as the record of what was found.)*

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
says `interrupted` where the tool told the agent "recorded as superseded"
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

**The renewal tick evicted voided claims from session tracking** *(found
2026-08-04 while verifying the lease fix; the settle phase died on it)*.
`renewOnce` (`internal/daemon/mcp.go`) dropped any tracked claim whose
status was no longer active — deliberately including "lost to a
cross-machine race" — so once a renewal tick (every TTL/3) landed between
a raced claim and its settle, the loser's `confirm_claim` answered "this
session holds no claim" instead of D6 clause 3's promised "lost". The
storm never trips it (contests confirm within seconds); the settle phase
sits minutes after its claims, so the bar was a run-timing coin flip.

Before the fixes, a default run exited red on three checks (the
storm-contest record shape, fate-vs-replay agreement, and silent-loser
synthesis) — or died in the settle phase when the eviction timing bit.
Everything else — including byte-identical convergence and exactly one
`claim.confirmed` per contest — passed throughout. The harness asserts
the design's promises, not the implementation's current behavior, on
purpose; all three findings are now fixed and the full run is green (see
the resolution note above).

### Findings from running it (2026-08-03)

**The daemon never writes the `superseded` run D6 promises.** *(Resolved
2026-08-04: this finding triggered the confirmation-gate grill that revised
D6 — the daemon now referees every finish, coercing a lost attempt's report
to `superseded` with its links kept, and replay synthesizes a branch-less
`superseded` close for a loser that never reports. The harness no longer
plays the losing daemon's part: `POST /v0/runs` is not called at all, and
every outcome it checks was written through the public tools. Since
2026-09-11 the op layer rejects daemon-synthesized outcomes from every
caller, the HTTP portal included — `002` T5. The original text is kept
below as the record of what was found.)* D6 clause 2
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
