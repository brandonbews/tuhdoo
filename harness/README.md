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

### What it tests, and why

tuhdoo promises that every machine converges on the same ledger: the same
events, the same replayed state, byte-identical generated views. The code
that keeps that promise when machines disagree is the set-union merge path —
divergent histories merged by application logic, a two-parent commit via
`commit-tree`, deterministic view regeneration — and it never runs when one
machine writes the ledger alone, because a single machine never diverges
from itself. This harness makes two machines disagree on purpose, many times
over, and checks that they still converge.

Two clones on one box is a faithful stand-in for two machines. The machine
id is minted per repository directory (`internal/daemon/daemon.go`
`machineID`), so the clones get distinct ids, and event ordering is ULID
order — no wall clock is ever trusted across machines.

The run, end to end:

1. builds `./cmd/tuhdoo` into a temp dir (it never trusts `bin/tuhdoo`);
2. makes a scratch bare origin and two clones under a short `/tmp` path —
   the daemon's socket lives at `<repo>/.git/tuhdoo/daemon.sock` and macOS
   caps socket paths at 103 bytes, so the default `/var/folders/...` TMPDIR
   would blow the limit (the same trap `npm/smoke.sh` documents);
3. seeds a pool of tasks that need no work — claiming one *is* the run;
4. opens one MCP session per clone by spawning `tuhdoo mcp --as <principal>`
   and speaking MCP over its stdio. The claim lifecycle is session-only by
   design: there is deliberately no one-shot `tuhdoo claim`, so a scripted
   actor has to hold a session, and holding it is what keeps the leases
   renewed;
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
6. storms the confirmation gate: both actors deliberately claim the same
   task (`claim_task` behind a barrier — claims are optimistic and judged
   locally, so both succeed: a certain collision), then race
   `confirm_claim` from both sessions at once. The remote's ref CAS is the
   referee; exactly one `claim.confirmed` may land per contest, and a
   contest where both actors are told "confirmed" fails the run on the
   spot. Every confirmed verdict is asked again and must answer the same
   (confirmed, same claim — a confirmation is irrevocable). Winners record
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

The harness checks what tuhdoo promises, not what the code happens to do
today: a check goes red when the two disagree, and that is the point.

### How to read the output

The `== verification` block is the acceptance list. Every line is
machine-checked; `[FAIL]` on any of them exits non-zero.

| Line | What it establishes |
| --- | --- |
| identical event sets | set-union merge lost nothing |
| byte-identical replayed state | the deterministic core agrees on both machines (both replays use one `Now`, since lease expiry is evaluated against it) |
| byte-identical generated views | the machines converge on byte-identical views, not just on the same events |
| identical data-branch trees | the strongest single check: one tree hash covers stored events *and* generated views |
| at least one real merge commit | the two-parent `commit-tree` path actually executed |
| no task carries more than one `claim.confirmed` | at most one confirmation per task, counted straight off the stored events — a duplicate is a hard failure at any probability |
| both machines claimed every storm contest | every deliberate collision really was one |
| every storm contest: one confirmation, one done, one superseded | the confirmation-race storm settled every deliberate collision through the real gate |
| claim races observed | both actors claimed the same task in the same round |
| claims voided by the winner rule | replay called the race, i.e. `core.ClaimVoided` |
| exactly one survivor — the confirmed claim, else earliest ULID | the winner rule re-derived from the replayed claims, not trusted: a confirmed claim wins outright, otherwise the earliest claim holds the task (ULIDs are unique, so two claims never tie); the storm makes confirmations that out-rank an earlier ULID actually happen |
| every race crossed machines | winner and loser carry different machine ids |
| every tool-discovered fate matches replay | what each daemon told its agent (done / lost) is what the ledger says |
| every done is certified | a `done` run's claim carries its `claim.confirmed` — no uncertified `done` exists |
| reporting losers coerced, branch kept | `finish_run(done)` on a lost attempt was recorded `superseded`, with the reported branch kept as salvage |
| silent losers closed by synthesis | a stand-down with no report was closed by replay's branch-less synthesized `superseded` run |
| natural-expiry losers closed at the lapse, lease unreleased | a loser that neither reported nor released was closed by replay's synthesized run once its lease lapsed — no real run, no release event, and the lease file on both trees is a plain lease (not a released tombstone) lapsing where the daemon said it would |
| both daemons showed each natural-expiry close after the lapse | each daemon's own read surface showed the close (expiry is a scheduled transition, not a reaper sweep), with the slowest observed lag |
| every claim response carried the confirm-before-merge warning | counted over every `claim_next` and `claim_task` answer of the run: a race loser learns it lost only when it next calls a tool, so the rule is stated when the claim is handed over |
| every confirmed verdict answered the same when asked again | `confirm_claim` re-asked after every confirmed verdict: still confirmed, same claim (a confirmation is irrevocable and the tool idempotent); the zero-duplicates line separately proves the repeat wrote nothing |
| the push cycle never exhausted maxCycleRetries | see `[note]` below |

`[note]` lines report something rather than pass or fail it. Today there
is one: if the push cycle exhausted `maxCycleRetries`, the run says how
often instead of failing (see the finding below).

`== numbers` is the measurement record. Read it carefully:

- **non-fast-forward pushes** is `syncer.Status.Collisions`. It counts push
  contention, **not** claim collisions. Nothing anywhere counts voided
  claims; this harness derives them from replayed state.
- **app-level merges built** is `syncer.Status.Merges`, which increments
  when a merge commit is *constructed*. A merge whose ref update or push
  then loses a race is discarded, so fewer merge commits land on the branch
  than were built. The "merge commits on data branch" line is the landed
  count (`git rev-list --merges --count`). Which machine builds a merge
  depends on who pushed first: the daemon that lands its push owns the
  remote ref, and the other fetches and merges. A consistent speed
  difference between the machines can put most of the merges on one side,
  so one machine's `merges` counter is not a fleet-wide number.
- **gate retries at the tools** counts the harness retrying the gate's
  honest refusals (the remote kept moving, could not be consulted, or the
  confirmation push failed — nothing written, try again), the way a
  protocol-following agent would.
- **pushes losing the ref lock** counts sync errors where two pushes hit
  the remote's ref at the same instant and git rejected one with
  `cannot lock ref 'refs/heads/tuhdoo': is at X but expected Y` — a lost
  ref-update race rather than a stale history. `gitx` classifies that as
  ordinary contention (`internal/gitx/cli.go`), so it is counted under
  non-fast-forward pushes and retried, and this line should read 0. A
  non-zero count means the classification has regressed: the push fell
  through as a generic error, skipped the retry loop for a cycle, and went
  uncounted.

`== FINDINGS` only appears when the run hit something worth writing down.

### Why it takes minutes

The daemon fetches every 60s, and only claim and escalation writes push
eagerly. The harness's one lever on that is an eager write, and an eager
write is exactly what has to *stop* before the two machines can be still at
the same moment — so the convergence waits are quiet waits, one fetch
interval per hop.

The natural-expiry arm adds the lease TTL to that floor: its losers' leases
lapse one `-lease-ttl` after the claim, and the run waits for it. The
default is 4m, and it cannot be much shorter. Lease renewals are ordinary
batched writes riding the 60s cycle, not eager pushes, so a peer's copy of
a live lease can trail by one renewal period (TTL/3) plus the batcher's 2s
quiet plus two 60s hops — the writer's push and the reader's fetch. The
TTL has to exceed TTL/3 + 122s, about three minutes, or a peer can see a
live claim as lapsed between renewals: the exact flapping the production
default (a 15m TTL renewed every 5m) rules out. Four minutes clears the
bound with a margin. The arm's contests run first so the clock starts
early; the post-race wait is an events-only wait (both machines hold every
claim) rather than a tree comparison, because the still-open sessions renew
their live leases every TTL/3 and a renewal moves a tree — the
acceptance-grade tree check is the final convergence, taken with the
sessions closed. A full run with the defaults takes about four minutes, most of it spent
waiting on purpose.

### What runs here, and what the unit tests cover

Covered end to end by this harness, through the public tools on two
daemons: the confirmation race, the coerced report and the stand-down
synthesis, the never-reports natural-expiry synthesis with its scheduled
transition, the claim-response warning, and the repeat-confirm stability.

Not run here, on purpose — they need one daemon and a remote that is
absent or unreachable, which two live clones of one origin cannot stage
without severing the very link the rest of the run measures — and covered
by `internal/daemon` unit tests instead:

- **remote configured but unreachable → honest retryable refusal, nothing
  written**: `TestConfirmClaimUnreachableRemoteRefusesHonestly`
  (`gate_test.go`);
- **remoteless confirmation is local, instant, idempotent**:
  `TestConfirmClaimRemoteless` (`gate_test.go`);
- **late-loser messaging** — a loser returning after its attempt was
  closed by synthesis is refused with the `add_note` salvage pointer, and
  the salvage note carries no stand-down nag:
  `TestLateLoserFinishRejectedWithAddNotePointer` (`loser_test.go`); the
  call-time stand-down notices on `add_note` and `escalate`, and the
  coerced finish through the MCP surface: `TestCallTimeStandDownNotices`
  (`loser_test.go`);
- the rule the natural-expiry arm leans on — a provisionally-voided claim
  stays tracked by its session but is never renewed:
  `TestRenewOnceKeepsVoidedClaimsTracked` (`loser_test.go`).

### A sample run

One run on macOS with the default flags:

```
== numbers
   ("built" counts merge commits the syncer constructed; a merge whose ref
    update or push loses a race is discarded, so fewer land on the branch)
   claim rounds fired            10
   claim races observed          10
   storm contests fired          40
   claims made                   104
   claims voided (race losers)   52
   claim.confirmed on the branch 52
   duplicate confirmations       0
   confirmation races won        (alpha 17, bravo 23)
   winners recorded done         52
   losers coerced on report      23
   losers closed by synthesis    27
   losers closed by lease lapse  2  (natural expiry, lease TTL 4m0s)
   confirmations re-asked        45
   gate retries at the tools     5
   merge commits on data branch  161
   non-fast-forward pushes       99  (alpha 45, bravo 54)
   app-level merges built        161  (alpha 83, bravo 78)
   pushes losing the ref lock    0  (alpha 0, bravo 0) — should be 0, see FINDINGS if not
   maxCycleRetries exhausted     1 (1.0% of non-fast-forward pushes)
   sync errors seen on bravo:
      syncer: remote origin kept moving for 4 attempts
```

394 events, byte-identical replayed state (~122 kB) and 60 byte-identical
view files on both machines, on an identical data-branch tree. Every hard
check passed; the one `[note]` was the push-retry finding below. Both
daemons showed each natural-expiry close within 100 ms of the lapse, with
the sessions still open, and every one of the 104 claim responses carried
the confirm-before-merge warning. Wall time about four minutes.

### Current finding

**The bounded push-retry loop does not survive sustained symmetric eager
writes.** `syncer.maxCycleRetries` is 4. Under the storm phase — 40 eager
writes per machine, both machines at once, several per second each — a
daemon can report `remote origin kept moving for 4 attempts`, and its work
stops reaching the remote for the length of the burst, because the peer's
eager pushes keep winning the ref. Nothing is lost: the ledger converges as
soon as the burst stops. But while it lasts, one machine is invisible to
the other, which is precisely the window claim races open in. The load is
unrealistic by design — a real fleet's claims and escalations arrive per
minute, not several per second — so this is a ceiling measurement, not a
bug report. The run reports it as a `[note]` with its rate rather than a
failure. Run with `-storm-gap` to find where it starts.

### Safety

The harness only ever touches scratch repositories it creates under `/tmp`,
and refuses to start if its clone paths are not under that prefix. It never
looks at the repository it is run from beyond building the binary, and it
only ever SIGTERMs daemons it spawned itself. Nothing it does can reach this
repo's `tuhdoo` data branch.
