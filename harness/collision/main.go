// Command collision is tuhdoo's deliberate claim-collision harness: two
// independent daemons, one remote, many claim races in minutes.
//
// The hole it closes is the D3 set-union merge path. A single machine
// never diverges from itself, so on a solo-dogfooded ledger the
// divergent-history merge — union the event files, regenerate views,
// write a two-parent commit via commit-tree — has never run outside unit
// tests. Two clones on one box is a faithful stand-in for two machines:
// the machine id is minted per repository directory (daemon.go
// machineID), so the clones get distinct ids, and event ordering is ULID
// order, never a wall clock (T3). No second physical machine is needed.
//
// What it does, end to end:
//
//  1. builds ./cmd/tuhdoo into a temp dir (never trusts bin/tuhdoo);
//  2. creates a scratch bare origin plus two clones under a short /tmp
//     path — the daemon's unix socket lives at <repo>/.git/tuhdoo and
//     macOS caps socket paths at 103 bytes, so the default TMPDIR
//     (/var/folders/...) would blow the limit (see npm/smoke.sh);
//  3. seeds tasks that need no work at all, so a "run" is one claim;
//  4. opens one MCP session per clone (`tuhdoo mcp --as <principal>`) —
//     the claim lifecycle is session-only by design (T7), so a scripted
//     actor must hold a session;
//  5. storms the D6 confirmation gate: both actors deliberately claim
//     the same task (claim_task behind a barrier), then race
//     confirm_claim — the remote's ref CAS is the referee, and exactly
//     one claim.confirmed may land per contest;
//  6. fires claim_next from both actors behind a barrier, repeatedly;
//  7. lets both daemons converge, then closes every attempt through the
//     public tools alone: winners record done through the gate, and
//     losers discover their fate from the daemon — a reported finish
//     coerced to superseded, or a stand-down closed by replay's
//     branch-less synthesized run. The harness never writes an outcome
//     on a daemon's behalf;
//  8. verifies convergence, the refereed winner rule, and the loser
//     records, and prints a report.
//
// Everything it asserts is machine-checked; a failed check exits
// non-zero. It cleans up its temp dirs and kills the daemons it spawned,
// including on failure.
//
// Run it from the repo root:
//
//	go run ./harness/collision
//
// See harness/README.md for how to read the output.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/brandonbews/tuhdoo/internal/core"
	"github.com/brandonbews/tuhdoo/internal/event"
	"github.com/brandonbews/tuhdoo/internal/gitx"
	"github.com/brandonbews/tuhdoo/internal/store"
)

// tempPrefix is where every scratch repo lives. Short on purpose (see
// the socket-length note in the package comment) and asserted before any
// daemon starts, so the harness structurally cannot be pointed at a real
// repository.
const tempPrefix = "/tmp"

// dataRef is the data branch the daemons keep their ledger on.
const dataRef = "refs/heads/tuhdoo"

type config struct {
	rounds   int
	spare    int
	storm    int
	contests int
	gap      time.Duration
	converge time.Duration
	keep     bool
}

func main() {
	var cfg config
	flag.IntVar(&cfg.rounds, "rounds", 10, "claim rounds to fire (one contested task per round)")
	flag.IntVar(&cfg.spare, "spare", 3, "extra seeded tasks beyond the rounds, so the pool never runs dry")
	flag.IntVar(&cfg.storm, "storm", 40, "simultaneous eager-write bursts aimed at the sync loop's push cycle")
	flag.IntVar(&cfg.contests, "confirm-storm", 40,
		"confirmation-race storm: deliberately collided claims whose confirm_claim verdicts are raced from both machines")
	flag.DurationVar(&cfg.gap, "storm-gap", 0, "pause between storm bursts; 0 is back-to-back, the harshest setting")
	flag.DurationVar(&cfg.converge, "converge-timeout", 5*time.Minute,
		"how long to wait for the two clones to reach identical data-branch trees "+
			"(T8 fetch cadence is 60s, so an unpoked hop costs up to a minute)")
	flag.BoolVar(&cfg.keep, "keep", false, "keep the scratch repos on exit instead of deleting them")
	flag.Parse()

	if err := run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "collision harness:", err)
		os.Exit(1)
	}
}

func run(cfg config) error {
	if cfg.rounds < 1 {
		return fmt.Errorf("-rounds must be at least 1")
	}
	lab, err := setup(cfg)
	if lab != nil {
		defer lab.close()
		// A ^C must still take the daemons and the scratch repos with it.
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
		go func() {
			s := <-sigs
			fmt.Printf("\n== interrupted (%s); cleaning up\n", s)
			lab.close()
			os.Exit(130)
		}()
	}
	if err != nil {
		return err
	}
	return lab.experiment()
}

// ---------------------------------------------------------------------
// the lab: two clones of one scratch remote, each with its own daemon
// ---------------------------------------------------------------------

// actor is one scripted agent: a clone, the daemon serving it, and the
// MCP session its claims hang off.
type actor struct {
	name      string // "alpha" / "bravo", for the report
	principal string // D7 principal, fixed with --as so the ledger is legible
	root      string // clone path
	bin       string // the tuhdoo binary under test

	pid    int
	socket string
	hc     *http.Client // the daemon's JSON API over its unix socket

	sess    *mcp.ClientSession
	shimLog *os.File
}

type lab struct {
	cfg       config
	work      string // scratch root under /tmp
	origin    string // bare remote
	bin       string
	actors    []*actor
	tasks     []string // seeded claimable task IDs for the claim_next rounds
	contest   []string // seeded task IDs for the confirmation-race storm
	carrier   string   // held task the sync-storm escalations hang off
	stormTook time.Duration
	watch     *syncWatch
	closing   sync.Once

	statMu      sync.Mutex
	gateRetries int // tool-level retries of the gate's honest 503 refusals
}

func setup(cfg config) (*lab, error) {
	root, err := moduleRoot()
	if err != nil {
		return nil, err
	}
	work, err := os.MkdirTemp(tempPrefix, "tuh-collide-")
	if err != nil {
		return nil, fmt.Errorf("scratch dir: %w", err)
	}
	l := &lab{cfg: cfg, work: work, origin: filepath.Join(work, "origin.git"), bin: filepath.Join(work, "tuhdoo")}
	fmt.Printf("== scratch repos under %s\n", work)

	fmt.Println("== building ./cmd/tuhdoo")
	if out, err := runCmd(root, "go", "build", "-o", l.bin, "./cmd/tuhdoo"); err != nil {
		return l, fmt.Errorf("build tuhdoo: %w: %s", err, out)
	}

	// The remote is an ordinary bare repo on the filesystem: tuhdoo
	// speaks the git protocol and nothing else (T2), so no host is
	// involved and none is needed.
	if out, err := runCmd(work, "git", "init", "--bare", "--initial-branch=main", l.origin); err != nil {
		return l, fmt.Errorf("init origin: %w: %s", err, out)
	}

	l.actors = []*actor{
		{name: "alpha", principal: "harness/alpha", root: filepath.Join(work, "alpha"), bin: l.bin},
		{name: "bravo", principal: "harness/bravo", root: filepath.Join(work, "bravo"), bin: l.bin},
	}
	for _, a := range l.actors {
		if !strings.HasPrefix(a.root, tempPrefix+"/") {
			return l, fmt.Errorf("refusing to run: clone path %s is not under %s", a.root, tempPrefix)
		}
	}
	return l, nil
}

// close stops everything the harness started and removes the scratch
// tree. Safe to call twice (deferred exit and the signal handler race).
func (l *lab) close() {
	l.closing.Do(func() {
		if l.watch != nil {
			l.watch.stop()
		}
		for _, a := range l.actors {
			a.closeSession()
		}
		for _, a := range l.actors {
			if err := a.stopDaemon(); err != nil {
				fmt.Fprintf(os.Stderr, "collision harness: stopping %s daemon: %v\n", a.name, err)
			}
		}
		if l.cfg.keep {
			fmt.Printf("== keeping scratch repos at %s (-keep)\n", l.work)
			return
		}
		if err := os.RemoveAll(l.work); err != nil {
			fmt.Fprintf(os.Stderr, "collision harness: removing %s: %v\n", l.work, err)
		}
	})
}

// ---------------------------------------------------------------------
// the experiment
// ---------------------------------------------------------------------

// attempt is one actor's claim in one contest: a claim_next call in a
// race round, or a deliberate claim_task in a storm contest.
type attempt struct {
	round  int
	actor  *actor
	task   string
	claim  string
	branch string // the branch this attempt would have worked on
	fate   string // how the daemon said it ended (fateDone/fateReported/fateSilent)
	runID  string // the real run.finished event recorded, when the attempt wrote one
}

// How an attempt's actor learned its attempt ended — always from a
// daemon tool's answer, never from the harness reading replayed state
// and deciding for it (that was the impersonation this harness used to
// commit before the 2026-08-04 D6 machinery existed).
const (
	fateDone     = "done"          // finish_run recorded done, refereed through the gate
	fateReported = "lost-reported" // finish_run(done) coerced to superseded, branch kept
	fateSilent   = "lost-silent"   // stood down via release_claim; replay synthesizes the close
)

func (l *lab) experiment() error {
	alpha, bravo := l.actors[0], l.actors[1]

	if err := l.seed(alpha); err != nil {
		return err
	}
	if err := l.joinSecondClone(bravo); err != nil {
		return err
	}

	// Both daemons are live from here on; watch their sync loops for the
	// whole experiment so a transient "remote kept moving" cannot pass
	// unseen (the syncer records it in Status.LastError and clears it on
	// the next good cycle).
	l.watch = newSyncWatch(l.actors)

	if err := l.openSessions(); err != nil {
		return err
	}

	// The confirmation-race storm runs first, on its own task pool:
	// every contest ends with the task done, so the claim_next rounds
	// below can never be served a contest task.
	stormAttempts, stats, err := l.confirmStorm()
	if err != nil {
		return err
	}

	raceAttempts, races, err := l.race()
	if err != nil {
		return err
	}

	if err := l.storm(); err != nil {
		return err
	}

	// The race attempts settle after the claims have crossed. The gate
	// would referee correctly either way — that is its whole point — but
	// waiting here makes the split deterministic: every loser discovers
	// its fate at call-time from local state, and every winner's gate
	// round-trip certifies against a remote that already holds the
	// loser's claim.
	if err := l.converge("post-race", append(claimIDs(raceAttempts), requiredEvents(stormAttempts)...)); err != nil {
		return err
	}

	if err := l.settle(raceAttempts); err != nil {
		return err
	}

	// Sessions closed before the final convergence: lease renewal is what
	// keeps writing after the work stops, and a renewal landing mid-check
	// would move one tree under the comparison.
	fmt.Println("== closing MCP sessions")
	for _, a := range l.actors {
		a.closeSession()
	}

	attempts := append(stormAttempts, raceAttempts...)
	if err := l.converge("final", requiredEvents(attempts)); err != nil {
		return err
	}

	return l.verify(attempts, races, stats)
}

// requiredEvents lists the stored events these attempts produced — the
// claims and the real run records. Synthesized closes are replay-derived
// and never stored (D6 clause 3), so they have no event to wait for;
// tree equality covers them.
func requiredEvents(attempts []attempt) []string {
	var ids []string
	for _, at := range attempts {
		ids = append(ids, at.claim)
		if at.runID != "" {
			ids = append(ids, at.runID)
		}
	}
	return ids
}

// seed creates the scratch task pool on the first clone and hands it to
// the remote. The daemon is stopped once at the end: its shutdown flushes
// pending events and runs a final sync cycle, which is what publishes the
// seed without waiting out the 60s fetch cadence (T8).
func (l *lab) seed(a *actor) error {
	fmt.Printf("== seeding %d tasks on %s (%d race bait, %d storm contests)\n",
		l.cfg.rounds+l.cfg.spare+l.cfg.contests, a.name, l.cfg.rounds+l.cfg.spare, l.cfg.contests)
	if out, err := runCmd(l.work, "git", "clone", "--quiet", l.origin, a.root); err != nil {
		return fmt.Errorf("clone %s: %w: %s", a.name, err, out)
	}
	if err := gitIdentity(a.root); err != nil {
		return err
	}
	// A repo needs a commit before tuhdoo will report on it, and a real
	// repo always has one.
	if err := os.WriteFile(filepath.Join(a.root, "README.md"), []byte("collision harness scratch repo\n"), 0o644); err != nil {
		return err
	}
	for _, args := range [][]string{
		{"add", "README.md"},
		{"commit", "--quiet", "-m", "seed"},
		{"push", "--quiet", "origin", "main"},
	} {
		if out, err := runCmd(a.root, "git", args...); err != nil {
			return fmt.Errorf("git %s in %s: %w: %s", args[0], a.name, err, out)
		}
	}

	if err := a.startDaemon(); err != nil {
		return err
	}

	// The carrier is seeded held, so it is never served to claim_next: it
	// exists only to give the sync-storm escalations a subject.
	items := []map[string]any{{
		"title":       "sync storm carrier",
		"status":      "held",
		"description": "Never claimable. Carries the harness's eager-write bursts (see storm).",
	}}
	for i := 0; i < l.cfg.rounds+l.cfg.spare; i++ {
		items = append(items, map[string]any{
			"title": fmt.Sprintf("collision bait %02d", i+1),
			"description": "Scratch task for the collision harness. There is no work here: " +
				"claiming it is the whole run.",
		})
	}
	for i := 0; i < l.cfg.contests; i++ {
		items = append(items, map[string]any{
			"title": fmt.Sprintf("confirm-storm contest %02d", i+1),
			"description": "Scratch task for the confirmation-race storm: both machines claim it " +
				"deliberately, then race confirm_claim for the one verdict.",
		})
	}
	var created struct {
		IDs []string `json:"ids"`
	}
	if err := a.post("/v0/tasks", a.principal, items, &created); err != nil {
		return fmt.Errorf("seed tasks: %w", err)
	}
	l.carrier = created.IDs[0]
	l.tasks = created.IDs[1 : 1+l.cfg.rounds+l.cfg.spare]
	l.contest = created.IDs[1+l.cfg.rounds+l.cfg.spare:]

	// The commit debounce is 2s (T8); give it room, then let the
	// shutdown's final flush + sync cycle publish the branch.
	time.Sleep(3 * time.Second)
	if err := a.stopDaemon(); err != nil {
		return err
	}
	if _, err := runCmd(l.origin, "git", "rev-parse", "--verify", dataRef); err != nil {
		return fmt.Errorf("the seed never reached the remote: %s has no %s", l.origin, dataRef)
	}
	return a.startDaemon()
}

// joinSecondClone brings the second machine online. It fetches the data
// branch into a local ref before its daemon starts: without that the
// daemon would mint its own orphan root (store.Init) and the first sync
// would merge two unrelated histories — a real merge, but a join merge,
// not a race merge. Starting from the same root keeps every merge commit
// this harness counts attributable to a claim race.
func (l *lab) joinSecondClone(a *actor) error {
	fmt.Printf("== joining %s to the remote\n", a.name)
	if out, err := runCmd(l.work, "git", "clone", "--quiet", l.origin, a.root); err != nil {
		return fmt.Errorf("clone %s: %w: %s", a.name, err, out)
	}
	if err := gitIdentity(a.root); err != nil {
		return err
	}
	if out, err := runCmd(a.root, "git", "fetch", "--quiet", "origin", dataRef+":"+dataRef); err != nil {
		return fmt.Errorf("fetch data branch into %s: %w: %s", a.name, err, out)
	}
	if err := a.startDaemon(); err != nil {
		return err
	}

	// Both machines must be looking at the same pool, or a "race" would
	// just be two actors picking different tasks.
	seeded := len(l.tasks) + len(l.contest)
	for _, x := range l.actors {
		ready, err := x.readyTasks()
		if err != nil {
			return err
		}
		if len(ready) != seeded {
			return fmt.Errorf("%s sees %d ready tasks, seeded %d", x.name, len(ready), seeded)
		}
	}
	fmt.Printf("   both machines see %d ready tasks\n", seeded)
	return nil
}

func (l *lab) openSessions() error {
	fmt.Println("== opening one MCP session per machine")
	for _, a := range l.actors {
		if err := a.openSession(); err != nil {
			return err
		}
		fmt.Printf("   %s: session as %s (machine %s)\n", a.name, a.principal, a.machineID())
	}
	if l.actors[0].machineID() == l.actors[1].machineID() {
		return fmt.Errorf("both clones minted machine id %s — they are not two machines", l.actors[0].machineID())
	}
	return nil
}

// race fires claim_next from both actors as close to simultaneously as
// the runtime allows, for cfg.rounds rounds. The window a collision needs
// is the gap between one daemon writing a claim and the other daemon
// fetching it: eager claim pushes (T8) close it in about a second, and
// the barrier below aims both calls at that second.
func (l *lab) race() ([]attempt, int, error) {
	fmt.Printf("== racing %d rounds\n", l.cfg.rounds)
	var all []attempt
	races := 0

	for round := 1; round <= l.cfg.rounds; round++ {
		results := make([]attempt, len(l.actors))
		errs := make([]error, len(l.actors))

		// A closed channel is the starting gun: every actor blocks on the
		// same receive, so the calls leave together.
		var wg sync.WaitGroup
		gun := make(chan struct{})
		for i, a := range l.actors {
			wg.Add(1)
			go func(i int, a *actor) {
				defer wg.Done()
				<-gun
				out, err := a.claimNext()
				if err != nil {
					errs[i] = err
					return
				}
				if !out.Claimed || out.Task == nil || out.Task.Claim == nil {
					return
				}
				results[i] = attempt{
					round: round, actor: a,
					task:   out.Task.Task.ID,
					claim:  out.Task.Claim.ID,
					branch: fmt.Sprintf("%s/round-%02d-%s", event.ShortID(out.Task.Task.ID), round, a.name),
				}
			}(i, a)
		}
		close(gun)
		wg.Wait()

		for i, err := range errs {
			if err != nil {
				return nil, 0, fmt.Errorf("round %d: %s claim_next: %w", round, l.actors[i].name, err)
			}
		}

		claimed := make([]attempt, 0, len(results))
		for _, r := range results {
			if r.claim != "" {
				claimed = append(claimed, r)
			}
		}
		all = append(all, claimed...)

		switch {
		case len(claimed) == len(l.actors) && sameTask(claimed):
			races++
			fmt.Printf("   round %02d: both claimed %s — race\n", round, event.ShortID(claimed[0].task))
		case len(claimed) == len(l.actors):
			fmt.Printf("   round %02d: different tasks (%s, %s) — no race\n",
				round, event.ShortID(claimed[0].task), event.ShortID(claimed[1].task))
		default:
			fmt.Printf("   round %02d: %d of %d actors claimed\n", round, len(claimed), len(l.actors))
		}
	}
	return all, races, nil
}

// settle closes every race attempt through the public tools alone. The
// harness never reads replayed state to decide who won — the daemons'
// answers are the only oracle (D6, 2026-08-04: the daemon is the referee
// of how attempts ended). Two agent styles alternate, both
// protocol-legal:
//
//   - confirm-first (even attempts): confirm_claim discovers the fate;
//     a confirmed winner records done (the gate already ran, so the
//     finish is judged locally); a loser stands down with release_claim
//     and never reports a run — replay closes that attempt with a
//     branch-less synthesized superseded run.
//   - report-only (odd attempts): finish_run(done) straight away; the
//     gate rides the tool (D6 clause 2), so a winner's done is certified
//     at the remote and a loser's report is coerced to superseded with
//     its branch kept as the salvage record.
func (l *lab) settle(attempts []attempt) error {
	fmt.Println("== settling race attempts through the daemons (fates discovered at the verbs, never scripted)")
	done, reported, silent := 0, 0, 0
	for i := range attempts {
		at := &attempts[i]
		if i%2 == 0 {
			// Confirm-first: ask the referee before doing anything else.
			res, err := l.confirmClaim(at.actor, at.task, fmt.Sprintf("round %02d", at.round))
			if err != nil {
				return err
			}
			if res.Confirmed {
				fin, err := l.finishDone(at.actor, at.task, at.branch,
					fmt.Sprintf("won the race on round %d (confirmed first)", at.round))
				if err != nil {
					return err
				}
				if fin.Outcome != event.OutcomeDone {
					return fmt.Errorf("round %d: %s confirmed but finish recorded %q", at.round, at.actor.name, fin.Outcome)
				}
				at.fate, at.runID = fateDone, fin.ID
				done++
			} else {
				rel, err := at.actor.releaseClaim(at.task,
					fmt.Sprintf("confirm_claim answered lost on round %d; standing down", at.round))
				if err != nil {
					return fmt.Errorf("round %d: %s stand-down: %w", at.round, at.actor.name, err)
				}
				if rel.Message == "" {
					return fmt.Errorf("round %d: %s's stand-down was not acknowledged", at.round, at.actor.name)
				}
				at.fate = fateSilent
				silent++
			}
			continue
		}
		// Report-only: finish_run(done) and let the referee judge.
		fin, err := l.finishDone(at.actor, at.task, at.branch,
			fmt.Sprintf("reporting done on round %d", at.round))
		if err != nil {
			return err
		}
		switch fin.Outcome {
		case event.OutcomeDone:
			at.fate, at.runID = fateDone, fin.ID
			done++
		case event.OutcomeSuperseded:
			if fin.Message == "" {
				return fmt.Errorf("round %d: %s was coerced to superseded without the referee's statement", at.round, at.actor.name)
			}
			at.fate, at.runID = fateReported, fin.ID
			reported++
		default:
			return fmt.Errorf("round %d: %s's finish recorded %q — want done or superseded", at.round, at.actor.name, fin.Outcome)
		}
	}
	fmt.Printf("   %d done through the gate, %d losers coerced on report, %d losers stood down for synthesis\n",
		done, reported, silent)
	return nil
}

// stormStats is the confirmation-race storm's measurement record.
type stormStats struct {
	contests int
	wins     map[string]int // actor name → confirmation races won
	reported int            // losers that reported and were coerced
	silent   int            // losers that stood down for replay synthesis
}

// confirmStorm is the confirmation-race storm (roadmap v1 DoD clause 2,
// extended 2026-08-04): N deliberate claim collisions, each contest's
// verdict raced through the real D6 gate from both machines at once.
//
// The collision is certain, not probable: claims are optimistic and
// judged against local state only (D6 clause 4), and a peer's eager
// claim push needs about a second to cross, so two claim_task calls
// fired behind one barrier both succeed. The verdict is then raced —
// both sessions call confirm_claim simultaneously — and the remote's
// ref CAS referees: at most one claim.confirmed can land per task, by
// construction. A contest where both actors are told "confirmed" is a
// duplicate certification and fails the run on the spot.
func (l *lab) confirmStorm() ([]attempt, stormStats, error) {
	stats := stormStats{wins: make(map[string]int, len(l.actors))}
	if len(l.contest) == 0 {
		return nil, stats, nil
	}
	fmt.Printf("== confirmation-race storm: %d deliberate collisions, confirm_claim raced from both machines\n", len(l.contest))
	started := time.Now()
	var all []attempt
	for i, task := range l.contest {
		seq := i + 1

		// Deliberate collision: both machines claim the same task at once.
		atts := make([]attempt, len(l.actors))
		err := l.pair(func(j int, a *actor) error {
			claimID, err := a.claimTask(task)
			if err != nil {
				return fmt.Errorf("contest %02d: %s claim_task: %w", seq, a.name, err)
			}
			atts[j] = attempt{round: seq, actor: a, task: task, claim: claimID,
				branch: fmt.Sprintf("%s/storm-%02d-%s", event.ShortID(task), seq, a.name)}
			return nil
		})
		if err != nil {
			return nil, stats, err
		}

		// Race the gate for the one verdict.
		res := make([]confirmOut, len(l.actors))
		err = l.pair(func(j int, a *actor) error {
			var e error
			res[j], e = l.confirmClaim(a, task, fmt.Sprintf("contest %02d", seq))
			return e
		})
		if err != nil {
			return nil, stats, err
		}
		winner := -1
		for j := range res {
			if !res[j].Confirmed {
				continue
			}
			if winner != -1 {
				return nil, stats, fmt.Errorf("contest %02d: DUPLICATE CONFIRMATION — both %s and %s were told confirmed",
					seq, l.actors[winner].name, l.actors[j].name)
			}
			winner = j
		}
		if winner == -1 {
			return nil, stats, fmt.Errorf("contest %02d: no confirmation — both machines were told lost", seq)
		}
		loser := 1 - winner
		stats.wins[atts[winner].actor.name]++

		// The winner records done. Its claim is already confirmed, so the
		// finish is judged locally and instantly (D6: idempotent).
		fin, err := l.finishDone(atts[winner].actor, task, atts[winner].branch,
			fmt.Sprintf("won the confirmation race in contest %02d", seq))
		if err != nil {
			return nil, stats, err
		}
		if fin.Outcome != event.OutcomeDone {
			return nil, stats, fmt.Errorf("contest %02d: confirmed winner's finish recorded %q, want done", seq, fin.Outcome)
		}
		atts[winner].fate, atts[winner].runID = fateDone, fin.ID

		// The loser was told "lost"; its two honest exits alternate.
		how := ""
		if i%2 == 0 {
			// Reporting loser: finish_run(done) — the referee coerces the
			// record to superseded, keeping the branch for salvage.
			fin, err := l.finishDone(atts[loser].actor, task, atts[loser].branch,
				fmt.Sprintf("reporting my attempt in contest %02d", seq))
			if err != nil {
				return nil, stats, err
			}
			if fin.Outcome != event.OutcomeSuperseded || fin.Message == "" {
				return nil, stats, fmt.Errorf("contest %02d: loser's finish recorded %+v — want coerced superseded with the referee's statement", seq, fin)
			}
			atts[loser].fate, atts[loser].runID = fateReported, fin.ID
			stats.reported++
			how = "reported, coerced to superseded"
		} else {
			// Silent loser: stands down without ever reporting a run;
			// replay closes the attempt with a branch-less synthesized
			// superseded run the moment the stand-down drops its lease.
			rel, err := atts[loser].actor.releaseClaim(task,
				fmt.Sprintf("confirm_claim answered lost in contest %02d; standing down", seq))
			if err != nil {
				return nil, stats, fmt.Errorf("contest %02d: loser stand-down: %w", seq, err)
			}
			if rel.Message == "" {
				return nil, stats, fmt.Errorf("contest %02d: loser's stand-down was not acknowledged", seq)
			}
			atts[loser].fate = fateSilent
			stats.silent++
			how = "stood down, closed by synthesis"
		}
		fmt.Printf("   contest %02d: %s confirmed, %s lost (%s)\n",
			seq, atts[winner].actor.name, atts[loser].actor.name, how)
		all = append(all, atts...)
	}
	stats.contests = len(l.contest)
	fmt.Printf("   %d contests in %s, one confirmation each (%s %d, %s %d); losers: %d reported, %d silent\n",
		stats.contests, round1s(time.Since(started)),
		l.actors[0].name, stats.wins[l.actors[0].name],
		l.actors[1].name, stats.wins[l.actors[1].name],
		stats.reported, stats.silent)
	return all, stats, nil
}

// pair fires fn for both actors behind one barrier, as close to
// simultaneously as the runtime allows.
func (l *lab) pair(fn func(i int, a *actor) error) error {
	errs := make([]error, len(l.actors))
	var wg sync.WaitGroup
	gun := make(chan struct{})
	for i, a := range l.actors {
		wg.Add(1)
		go func(i int, a *actor) {
			defer wg.Done()
			<-gun
			errs[i] = fn(i, a)
		}(i, a)
	}
	close(gun)
	wg.Wait()
	return errors.Join(errs...)
}

// toolRetries bounds the harness's patience with the gate's honest
// retryable refusals. Under the storm both daemons' sync loops and both
// gates contend for the one remote ref, so an occasional "remote kept
// moving" 503 is expected; a protocol-following agent retries, and so
// does the harness — counting every retry for the report.
const toolRetries = 10

// gateRetryable matches the gate's honest refusals — nothing written,
// retry later (ops.go gateVerdict): the remote kept moving past the
// bounded loop, could not be consulted, or the confirmation push failed.
func gateRetryable(err error) bool {
	s := err.Error()
	return strings.Contains(s, "remote kept moving") ||
		strings.Contains(s, "cannot consult the remote") ||
		strings.Contains(s, "confirmation push failed")
}

// retryGate runs call, retrying the gate's retryable refusals.
func (l *lab) retryGate(what string, call func() error) error {
	var err error
	for i := 0; i < toolRetries; i++ {
		if err = call(); err == nil || !gateRetryable(err) {
			return err
		}
		l.statMu.Lock()
		l.gateRetries++
		l.statMu.Unlock()
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("%s: still refused after %d retries: %w", what, toolRetries, err)
}

// confirmClaim is confirm_claim with the harness's retry posture.
func (l *lab) confirmClaim(a *actor, task, what string) (confirmOut, error) {
	var out confirmOut
	err := l.retryGate(fmt.Sprintf("%s: %s confirm_claim", what, a.name), func() error {
		var e error
		out, e = a.confirmClaim(task)
		return e
	})
	return out, err
}

// finishDone is finish_run(done) with the harness's retry posture: the
// gate rides the tool, so it shares confirm_claim's retryable refusals.
func (l *lab) finishDone(a *actor, task, branch, summary string) (finishOut, error) {
	var out finishOut
	err := l.retryGate(fmt.Sprintf("%s finish_run on %s", a.name, event.ShortID(task)), func() error {
		var e error
		out, e = a.finishRunDone(task, branch, summary)
		return e
	})
	return out, err
}

// storm hammers the sync loop's push cycle. Claim and escalation writes
// are the two eager cases (T8): the daemon flushes them immediately and
// pokes the syncer, so each burst puts both daemons into fetch → merge →
// push at the same instant. That overlap is what produces
// non-fast-forward pushes, the bounded retry loop's only real exercise.
func (l *lab) storm() error {
	if l.cfg.storm <= 0 {
		return nil
	}
	fmt.Printf("== sync storm: %d simultaneous eager writes per machine\n", l.cfg.storm)
	started := time.Now()
	for i := 1; i <= l.cfg.storm; i++ {
		if _, err := l.burst(i); err != nil {
			return err
		}
		if l.cfg.gap > 0 {
			time.Sleep(l.cfg.gap)
		}
	}
	l.stormTook = time.Since(started)
	w := l.watch.snapshot()
	fmt.Printf("   %d bursts in %s; non-fast-forward pushes so far: %s %d, %s %d\n",
		l.cfg.storm, round1s(time.Since(started)),
		l.actors[0].name, w[l.actors[0].name].collisions,
		l.actors[1].name, w[l.actors[1].name].collisions)
	return nil
}

// burst writes one escalation on every machine at once, behind the same
// barrier the claim rounds use. It doubles as the harness's only lever on
// the T8 cadence: an eager write pokes the sync loop, and a poked cycle
// fetches before it pushes, so a burst pulls the peer's work in too.
func (l *lab) burst(seq int) ([]string, error) {
	ids := make([]string, len(l.actors))
	errs := make([]error, len(l.actors))
	var wg sync.WaitGroup
	gun := make(chan struct{})
	for i, a := range l.actors {
		wg.Add(1)
		go func(i int, a *actor) {
			defer wg.Done()
			<-gun
			var out struct {
				ID string `json:"id"`
			}
			err := a.post("/v0/escalations", a.principal, map[string]any{
				"task":     l.carrier,
				"question": fmt.Sprintf("sync-storm burst %d from %s: no answer wanted", seq, a.name),
				"context":  "Written by harness/collision to force simultaneous eager pushes.",
				"blocking": false,
			}, &out)
			if err != nil {
				errs[i] = err
				return
			}
			ids[i] = out.ID
		}(i, a)
	}
	close(gun)
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			return nil, fmt.Errorf("burst %d on %s: %w", seq, l.actors[i].name, err)
		}
	}
	return ids, nil
}

// diagnose prints each daemon's live sync health. Called whenever a wait
// is going badly: the sync loop records its failures in Status.LastError
// and clears them on the next good cycle, so a stall is only legible
// while it is happening.
func (l *lab) diagnose() {
	for _, a := range l.actors {
		var st stateResp
		if err := a.get("/v0/state", &st); err != nil {
			fmt.Printf("      %s: state unreadable: %v\n", a.name, err)
			continue
		}
		events, err := a.branchEventIDs()
		if err != nil {
			fmt.Printf("      %s: %v\n", a.name, err)
			continue
		}
		fmt.Printf("      %s: %d events on branch, sync %s, non-ff %d, merges %d, last_error %q\n",
			a.name, len(events), st.Sync.Mode, st.Sync.Collisions, st.Sync.Merges, st.Sync.LastError)
	}
}

// converge waits until the writes named by required are on both clones
// and the two data branches point at identical trees. Tree equality is
// the strongest single check available: the tree holds both the stored
// event blobs and the generated views, so identical trees mean identical
// events *and* byte-identical views. The required set is what keeps the
// check honest — two clones that have not yet flushed a write are also
// trivially "identical".
//
// Nothing is poked here, deliberately. The harness's only lever on the
// T8 cadence is an eager write, and a write is exactly what has to stop
// before the two sides can be still at the same moment; a poke every
// half-second also starves the peer's push (see the storm finding in
// harness/README.md). So this is a quiet wait, and one hop costs up to
// the 60s fetch interval.
func (l *lab) converge(stage string, required []string) error {
	deadline := time.Now().Add(l.cfg.converge)
	started := time.Now()
	lastReport := time.Now()
	fmt.Printf("== waiting for the two machines to converge (%s)\n", stage)
	for {
		absent, err := l.missingEvents(required)
		if err != nil {
			return err
		}
		trees, err := l.trees()
		if err != nil {
			return err
		}
		if len(absent) == 0 && trees[0] == trees[1] {
			fmt.Printf("   converged on tree %s after %s\n", trees[0][:12], round1s(time.Since(started)))
			return nil
		}
		if time.Now().After(deadline) {
			l.diagnose()
			return fmt.Errorf("%s convergence failed after %s: %d of %d expected events missing (%s); trees %s vs %s",
				stage, l.cfg.converge, len(absent), 2*len(required),
				strings.Join(absent, " "), trees[0][:12], trees[1][:12])
		}
		if time.Since(lastReport) >= 20*time.Second {
			lastReport = time.Now()
			fmt.Printf("   %s: %d events still to cross, trees %s vs %s (%s elapsed)\n",
				stage, len(absent), trees[0][:12], trees[1][:12], round1s(time.Since(started)))
			l.diagnose()
		}
		time.Sleep(2 * time.Second)
	}
}

// missingEvents names the required events each clone still lacks, as
// "<machine>:<short-id>".
func (l *lab) missingEvents(required []string) ([]string, error) {
	var absent []string
	for _, a := range l.actors {
		have, err := a.branchEventIDs()
		if err != nil {
			return nil, err
		}
		for _, id := range required {
			if !have[id] {
				absent = append(absent, a.name+":"+event.ShortID(id))
			}
		}
	}
	return absent, nil
}

func (l *lab) trees() ([]string, error) {
	trees := make([]string, len(l.actors))
	for i, a := range l.actors {
		t, err := a.treeOID()
		if err != nil {
			return nil, err
		}
		trees[i] = t
	}
	return trees, nil
}

// ---------------------------------------------------------------------
// verification — every claim below is machine-checked
// ---------------------------------------------------------------------

// checklist accumulates pass/fail verdicts so one run reports every
// failure, not just the first.
type checklist struct {
	failed int
}

func (c *checklist) check(ok bool, format string, args ...any) {
	mark := "ok  "
	if !ok {
		mark = "FAIL"
		c.failed++
	}
	fmt.Printf("   [%s] %s\n", mark, fmt.Sprintf(format, args...))
}

// note records something the acceptance asks to be *reported* rather
// than passed: the maxCycleRetries clause is "never exhausts — or, if it
// does, that is reported as a finding with the rate".
func (c *checklist) note(format string, args ...any) {
	fmt.Printf("   [note] %s\n", fmt.Sprintf(format, args...))
}

func (l *lab) verify(attempts []attempt, races int, stats stormStats) error {
	alpha, bravo := l.actors[0], l.actors[1]

	// One instant for both replays: lease expiry is evaluated against
	// Input.Now, so two different clocks would be two different questions.
	now := time.Now()
	stateA, eventsA, err := replayClone(alpha.root, now)
	if err != nil {
		return fmt.Errorf("replay %s: %w", alpha.name, err)
	}
	stateB, eventsB, err := replayClone(bravo.root, now)
	if err != nil {
		return fmt.Errorf("replay %s: %w", bravo.name, err)
	}

	fmt.Println("\n== verification")
	fmt.Println("   (every outcome checked below was written by a daemon verb; the harness wrote none)")
	var c checklist

	// --- convergence ---
	idsA, idsB := eventIDs(eventsA), eventIDs(eventsB)
	c.check(equalStrings(idsA, idsB), "identical event sets on both clones (%d events each; %d vs %d)",
		len(idsA), len(idsA), len(idsB))

	jsonA, err := json.Marshal(stateA)
	if err != nil {
		return err
	}
	jsonB, err := json.Marshal(stateB)
	if err != nil {
		return err
	}
	c.check(string(jsonA) == string(jsonB), "byte-identical replayed state (%d bytes)", len(jsonA))

	viewsA, err := viewFiles(alpha.root)
	if err != nil {
		return err
	}
	viewsB, err := viewFiles(bravo.root)
	if err != nil {
		return err
	}
	c.check(len(viewsA) > 0 && sameFiles(viewsA, viewsB),
		"byte-identical generated views (%d view files compared)", len(viewsA))

	treeA, err := alpha.treeOID()
	if err != nil {
		return err
	}
	treeB, err := bravo.treeOID()
	if err != nil {
		return err
	}
	c.check(treeA == treeB, "identical data-branch trees (%s)", treeA[:12])

	merges, err := mergeCount(alpha.root)
	if err != nil {
		return err
	}
	c.check(merges >= 1, "at least one real merge commit on the data branch (%d)", merges)

	// --- the confirmation gate (D6 clause 2, 2026-08-04) ---
	// Read-side verification straight off the stored events: at most one
	// claim.confirmed may ever land per task, by construction — a
	// duplicate is a bug at any probability.
	confs := make(map[string][]string)
	for _, e := range eventsA {
		if e.Type == event.TypeClaimConfirmed {
			confs[e.Task] = append(confs[e.Task], e.ID)
		}
	}
	totalConfs, dup := 0, 0
	for task, ids := range confs {
		totalConfs += len(ids)
		if len(ids) > 1 {
			dup++
			fmt.Printf("        task %s carries %d confirmations: %s\n",
				event.ShortID(task), len(ids), strings.Join(ids, " "))
		}
	}
	c.check(dup == 0, "no task carries more than one claim.confirmed (%d confirmations across %d tasks)",
		totalConfs, len(confs))

	contested, stormOK := 0, true
	for _, task := range l.contest {
		var doneRuns, superRuns int
		for i := range stateA.Runs {
			if r := &stateA.Runs[i]; r.Task == task {
				switch r.Outcome {
				case event.OutcomeDone:
					doneRuns++
				case event.OutcomeSuperseded:
					superRuns++
				}
			}
		}
		claims := len(stateA.ClaimsByTask[task])
		if claims == 2 {
			contested++
		}
		if claims != 2 || len(confs[task]) != 1 || doneRuns != 1 || superRuns != 1 {
			stormOK = false
			fmt.Printf("        contest %s: %d claims, %d confirmations, %d done, %d superseded\n",
				event.ShortID(task), claims, len(confs[task]), doneRuns, superRuns)
		}
	}
	c.check(contested == len(l.contest), "both machines claimed every storm contest (%d deliberate collisions)", contested)
	c.check(stormOK, "every storm contest landed exactly one claim.confirmed, one done run, one superseded run (%d contests)",
		len(l.contest))

	// --- the D6 winner rule, refereed ---
	voided := voidedClaims(stateA)
	c.check(races > 0, "claim races observed in the claim_next rounds (%d)", races)
	c.check(len(voided) > 0, "claims voided by the winner rule (%d)", len(voided))

	byClaim := make(map[string]*attempt, len(attempts))
	for i := range attempts {
		byClaim[attempts[i].claim] = &attempts[i]
	}

	crossMachine := 0
	winnerOK := true
	for _, loser := range voided {
		// Exactly one claim survives on the task, and it is the refereed
		// winner: the confirmed claim when a confirmation exists (a
		// confirmed claim wins unconditionally — it can out-rank an
		// earlier ULID, which the storm makes happen), the earliest ULID
		// otherwise (the provisional rule). Re-derived here, not trusted.
		var survivors []*core.Claim
		earliest, confirmed := "", ""
		for _, cid := range stateA.ClaimsByTask[loser.Task] {
			cl := stateA.Claims[cid]
			if cl.Status != core.ClaimVoided {
				survivors = append(survivors, cl)
			}
			if cl.Confirmation != "" {
				confirmed = cid
			}
			if earliest == "" || cid < earliest {
				earliest = cid
			}
		}
		want := earliest
		if confirmed != "" {
			want = confirmed
		}
		if len(survivors) != 1 || survivors[0].ID != want || loser.ID == want {
			winnerOK = false
			fmt.Printf("        task %s: %d survivors, refereed winner %s, loser %s\n",
				event.ShortID(loser.Task), len(survivors), event.ShortID(want), event.ShortID(loser.ID))
		} else if survivors[0].Machine != loser.Machine {
			crossMachine++
		}
	}
	c.check(winnerOK, "every voided claim leaves exactly one survivor — the confirmed claim, else earliest ULID (%d checked)",
		len(voided))
	c.check(crossMachine == len(voided), "every race crossed machines (%d of %d)", crossMachine, len(voided))

	// --- loser records: real coercion and real synthesis (D6 clause 3) ---
	runsByID := make(map[string]*core.Run, len(stateA.Runs))
	for i := range stateA.Runs {
		runsByID[stateA.Runs[i].ID] = &stateA.Runs[i]
	}
	fatesOK, winnersOK, reportedOK, silentOK := true, true, true, true
	winners, reported, silent := 0, 0, 0
	for _, loser := range voided {
		at := byClaim[loser.ID]
		if at == nil || at.fate == fateDone || at.fate == "" {
			fatesOK = false
			fmt.Printf("        voided claim %s (%s) closed with fate %q — the daemon told its actor something else\n",
				event.ShortID(loser.ID), loser.Actor, fateOf(at))
			continue
		}
		switch at.fate {
		case fateReported:
			// The loser reported; the referee coerced the record to
			// superseded, keeping the reported branch as salvage.
			reported++
			r := runsByID[at.runID]
			if r == nil || r.Synthesized || r.Outcome != event.OutcomeSuperseded ||
				r.Task != at.task || r.Actor != loser.Actor || r.Branch != at.branch {
				reportedOK = false
				fmt.Printf("        reporting loser %s on task %s: run %+v, want real superseded carrying branch %q\n",
					loser.Actor, event.ShortID(at.task), r, at.branch)
			}
		case fateSilent:
			// The loser stood down without a report; replay synthesized
			// the branch-less close for exactly this claim.
			silent++
			found := false
			for i := range stateA.Runs {
				r := &stateA.Runs[i]
				if r.Synthesized && r.Claim == loser.ID && r.Outcome == event.OutcomeSuperseded && r.Branch == "" {
					found = true
					break
				}
			}
			if !found {
				silentOK = false
				fmt.Printf("        silent loser %s on task %s: no synthesized branch-less superseded run for claim %s\n",
					loser.Actor, event.ShortID(at.task), event.ShortID(loser.ID))
			}
		}
	}
	// And the reverse: every fate a daemon handed out matches replay.
	for i := range attempts {
		at := &attempts[i]
		cl := stateA.Claims[at.claim]
		switch {
		case cl == nil:
			fatesOK = false
			fmt.Printf("        claim %s (%s) is missing from replayed state\n", at.claim, at.actor.name)
		case at.fate == fateDone:
			winners++
			r := runsByID[at.runID]
			if cl.Status == core.ClaimVoided || cl.Confirmation == "" ||
				r == nil || r.Outcome != event.OutcomeDone {
				winnersOK = false
				fmt.Printf("        winner %s on task %s: claim %s %s (confirmation %q), run %+v\n",
					at.actor.name, event.ShortID(at.task), event.ShortID(at.claim), cl.Status, cl.Confirmation, r)
			}
		default:
			if cl.Status != core.ClaimVoided {
				fatesOK = false
				fmt.Printf("        %s was told lost on task %s but replay says claim %s is %s\n",
					at.actor.name, event.ShortID(at.task), event.ShortID(at.claim), cl.Status)
			}
		}
	}
	c.check(fatesOK, "every verb-discovered fate matches replay's verdict (%d attempts)", len(attempts))
	c.check(winnersOK, "every done record is certified — the winner's claim carries its claim.confirmed (%d winners)", winners)
	c.check(reportedOK, "every reporting loser was coerced to a real superseded run keeping its branch (%d)", reported)
	c.check(silentOK, "every silent stand-down was closed by replay's branch-less synthesized run (%d)", silent)

	// --- the push-retry loop ---
	// Exhausting maxCycleRetries is not a failure of the experiment: the
	// acceptance asks for it to be reported with a rate if it happens.
	watch := l.watch.snapshot()
	exhausted := 0
	for _, w := range watch {
		exhausted += w.keptMoving
	}
	if exhausted == 0 {
		c.check(true, "the push cycle never exhausted maxCycleRetries")
	} else {
		c.note("the push cycle exhausted maxCycleRetries %d time(s) — see FINDINGS", exhausted)
	}

	l.report(attempts, races, stats, voided, merges, watch, exhausted, totalConfs, dup)

	if c.failed > 0 {
		return fmt.Errorf("%d acceptance check(s) failed", c.failed)
	}
	return nil
}

// fateOf reads an attempt's fate, tolerating the nil attempt of a claim
// the harness never made (which would itself be a failed check).
func fateOf(at *attempt) string {
	if at == nil {
		return "<no attempt>"
	}
	return at.fate
}

func (l *lab) report(attempts []attempt, races int, stats stormStats, voided []*core.Claim, merges int,
	watch map[string]syncSample, exhausted, totalConfs, dup int) {

	nonFF := l.total(watch, func(w syncSample) int { return w.collisions })
	built := l.total(watch, func(w syncSample) int { return w.merges })
	refLock := l.total(watch, func(w syncSample) int { return w.refLock })

	done, coerced, stoodDown := 0, 0, 0
	for _, at := range attempts {
		switch at.fate {
		case fateDone:
			done++
		case fateReported:
			coerced++
		case fateSilent:
			stoodDown++
		}
	}

	fmt.Println("\n== numbers")
	fmt.Println("   (\"built\" counts merge commits the syncer constructed; a merge whose ref")
	fmt.Println("    update or push loses a race is discarded, so fewer land on the branch)")
	fmt.Printf("   claim rounds fired            %d\n", l.cfg.rounds)
	fmt.Printf("   claim races observed          %d\n", races)
	fmt.Printf("   storm contests fired          %d\n", stats.contests)
	fmt.Printf("   claims made                   %d\n", len(attempts))
	fmt.Printf("   claims voided (D6 losers)     %d\n", len(voided))
	fmt.Printf("   claim.confirmed on the branch %d\n", totalConfs)
	fmt.Printf("   duplicate confirmations       %d\n", dup)
	fmt.Printf("   confirmation races won        (%s %d, %s %d)\n",
		l.actors[0].name, stats.wins[l.actors[0].name],
		l.actors[1].name, stats.wins[l.actors[1].name])
	fmt.Printf("   winners recorded done         %d\n", done)
	fmt.Printf("   losers coerced on report      %d\n", coerced)
	fmt.Printf("   losers closed by synthesis    %d\n", stoodDown)
	fmt.Printf("   gate retries at the verbs     %d\n", l.gateRetries)
	fmt.Printf("   merge commits on data branch  %d\n", merges)
	fmt.Printf("   non-fast-forward pushes       %d  %s\n", nonFF,
		l.perMachine(watch, func(w syncSample) int { return w.collisions }))
	fmt.Printf("   app-level merges built        %d  %s\n", built,
		l.perMachine(watch, func(w syncSample) int { return w.merges }))
	fmt.Printf("   pushes losing the ref lock    %d  %s — not counted as collisions, see FINDINGS\n", refLock,
		l.perMachine(watch, func(w syncSample) int { return w.refLock }))
	if nonFF > 0 {
		fmt.Printf("   maxCycleRetries exhausted     %d (%.1f%% of non-fast-forward pushes)\n",
			exhausted, 100*float64(exhausted)/float64(nonFF))
	} else {
		fmt.Printf("   maxCycleRetries exhausted     %d\n", exhausted)
	}
	for _, a := range l.actors {
		if errs := watch[a.name].errors; len(errs) > 0 {
			fmt.Printf("   sync errors seen on %s:\n", a.name)
			for _, e := range errs {
				fmt.Printf("      %s\n", e)
			}
		}
	}

	if exhausted == 0 && refLock == 0 {
		return
	}
	fmt.Println("\n== FINDINGS")
	if exhausted > 0 {
		fmt.Printf("   The bounded push-retry loop (syncer.maxCycleRetries = 4) does not survive\n"+
			"   sustained symmetric eager writes. The storm phase wrote %d eager events per\n"+
			"   machine in %s (~%.1f/s each, both machines at once) and a daemon reported\n"+
			"   \"remote kept moving for 4 attempts\" %d time(s): its work stopped reaching\n"+
			"   the remote for the length of the burst, because the peer's eager pushes kept\n"+
			"   winning the ref. Nothing was lost — the ledger converged once the burst\n"+
			"   stopped — but while it lasted one machine was invisible to the other, which\n"+
			"   is exactly the window D6 races open in.\n",
			l.cfg.storm, round1s(l.stormTook),
			float64(l.cfg.storm)/stormSeconds(l.stormTook), exhausted)
	}
	if refLock > 0 {
		fmt.Printf("\n   %d push(es) were rejected as a lost ref-update race at the remote\n"+
			"   (\"cannot lock ref 'refs/heads/tuhdoo': is at X but expected Y\") rather than\n"+
			"   as a stale history. gitx.Push classifies a rejection as ErrNonFastForward\n"+
			"   only when git's porcelain output says \"non-fast-forward\" or \"fetch first\"\n"+
			"   (internal/gitx/cli.go Push), so this shape falls through as a generic error:\n"+
			"   Syncer.Cycle returns instead of going around the retry loop, the daemon\n"+
			"   records mode=error, and Status.Collisions does not count it. The next cycle\n"+
			"   recovers, so nothing is lost — but the push-contention counter T8 says the\n"+
			"   daemon keeps undercounts real contention by this many.\n", refLock)
	}
}

// total sums one field of the sync samples across both machines.
func (l *lab) total(watch map[string]syncSample, field func(syncSample) int) int {
	n := 0
	for _, a := range l.actors {
		n += field(watch[a.name])
	}
	return n
}

// perMachine renders one field as "(alpha 0, bravo 11)", in actor order.
func (l *lab) perMachine(watch map[string]syncSample, field func(syncSample) int) string {
	parts := make([]string, 0, len(l.actors))
	for _, a := range l.actors {
		parts = append(parts, fmt.Sprintf("%s %d", a.name, field(watch[a.name])))
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

// stormSeconds guards the rate division for very fast storms.
func stormSeconds(d time.Duration) float64 {
	if d <= 0 {
		return 1
	}
	return d.Seconds()
}

// ---------------------------------------------------------------------
// replay and git reads over a clone
// ---------------------------------------------------------------------

// replayClone computes state from a clone's data branch with the real
// core, exactly the way the daemon does. The harness is in-module, so
// this is the same code path under test, not a reimplementation.
func replayClone(root string, now time.Time) (*core.State, []event.Event, error) {
	g, err := gitx.New(root)
	if err != nil {
		return nil, nil, err
	}
	// Identity is a write-path concern; this Store only reads.
	st := store.New(g, "", gitx.Identity{})
	events, leases, err := st.LoadReplayInput()
	if err != nil {
		return nil, nil, err
	}
	state, err := core.NewReplayer().Replay(core.Input{Events: events, Leases: leases, Now: now})
	if err != nil {
		return nil, nil, err
	}
	return state, events, nil
}

func eventIDs(events []event.Event) []string {
	ids := make([]string, 0, len(events))
	for _, e := range events {
		ids = append(ids, e.ID)
	}
	sort.Strings(ids)
	return ids
}

func voidedClaims(s *core.State) []*core.Claim {
	var out []*core.Claim
	for _, t := range s.TaskOrder {
		for _, cid := range s.ClaimsByTask[t] {
			if c := s.Claims[cid]; c.Status == core.ClaimVoided {
				out = append(out, c)
			}
		}
	}
	return out
}

// viewFiles returns the generated view blobs on a clone's data branch,
// path → contents. Everything outside events/ and leases/ is a view (T6).
func viewFiles(root string) (map[string]string, error) {
	g, err := gitx.New(root)
	if err != nil {
		return nil, err
	}
	entries, err := g.LsTree(dataRef)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(entries))
	for _, e := range entries {
		if strings.HasPrefix(e.Path, "events/") || strings.HasPrefix(e.Path, "leases/") {
			continue
		}
		data, err := g.CatFile(e.OID)
		if err != nil {
			return nil, err
		}
		out[e.Path] = string(data)
	}
	return out, nil
}

func sameFiles(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for path, content := range a {
		if b[path] != content {
			return false
		}
	}
	return true
}

func mergeCount(root string) (int, error) {
	out, err := runCmd(root, "git", "rev-list", "--merges", "--count", dataRef)
	if err != nil {
		return 0, fmt.Errorf("count merges: %w: %s", err, out)
	}
	return strconv.Atoi(strings.TrimSpace(out))
}

func (a *actor) treeOID() (string, error) {
	out, err := runCmd(a.root, "git", "rev-parse", dataRef+"^{tree}")
	if err != nil {
		return "", fmt.Errorf("read %s tree in %s: %w: %s", dataRef, a.name, err, out)
	}
	return strings.TrimSpace(out), nil
}

// branchEventIDs is the set of event IDs stored on a clone's data
// branch, read straight from the tree — the honest answer to "has this
// machine actually got that event yet", independent of daemon caches.
func (a *actor) branchEventIDs() (map[string]bool, error) {
	out, err := runCmd(a.root, "git", "ls-tree", "-r", "--name-only", dataRef)
	if err != nil {
		return nil, fmt.Errorf("list %s tree in %s: %w: %s", dataRef, a.name, err, out)
	}
	ids := make(map[string]bool)
	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(line, "events/") {
			continue
		}
		ids[strings.TrimSuffix(filepath.Base(line), ".json")] = true
	}
	return ids, nil
}

func (a *actor) machineID() string {
	b, err := os.ReadFile(filepath.Join(a.root, ".git", "tuhdoo", "machine-id"))
	if err != nil {
		return "?"
	}
	return strings.TrimSpace(string(b))
}

// ---------------------------------------------------------------------
// daemon lifecycle and the JSON API over its unix socket
// ---------------------------------------------------------------------

// startDaemon brings up the clone's daemon the way any CLI call does —
// `tuhdoo init` auto-spawns it (T4 lazy lifecycle) — and records the pid
// and socket the harness will need.
func (a *actor) startDaemon() error {
	if out, err := runCmd(a.root, a.bin, "init"); err != nil {
		return fmt.Errorf("tuhdoo init in %s: %w: %s", a.name, err, out)
	}
	var disc struct {
		PID    int    `json:"pid"`
		Socket string `json:"socket"`
	}
	b, err := os.ReadFile(filepath.Join(a.root, ".git", "tuhdoo", "daemon.json"))
	if err != nil {
		return fmt.Errorf("read %s daemon.json: %w", a.name, err)
	}
	if err := json.Unmarshal(b, &disc); err != nil {
		return fmt.Errorf("parse %s daemon.json: %w", a.name, err)
	}
	a.pid, a.socket = disc.PID, disc.Socket
	a.hc = &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", disc.Socket)
		},
	}}
	return nil
}

// stopDaemon sends SIGTERM and waits for the process to actually exit —
// the daemon flushes, pushes, and releases its flock on the way out, and
// respawning before that finishes races the lock.
func (a *actor) stopDaemon() error {
	if a.pid == 0 {
		return nil
	}
	pid := a.pid
	a.pid = 0
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		return err
	}
	for i := 0; i < 200; i++ {
		if err := syscall.Kill(pid, 0); err != nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("%s daemon (pid %d) did not exit within 10s", a.name, pid)
}

func (a *actor) get(path string, dst any) error {
	resp, err := a.hc.Get("http://tuhdoo" + path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<24))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s: %s", path, resp.Status, strings.TrimSpace(string(body)))
	}
	return json.Unmarshal(body, dst)
}

func (a *actor) post(path, principal string, body, dst any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, "http://tuhdoo"+path, strings.NewReader(string(b)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tuhdoo-Actor", principal)
	resp, err := a.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<24))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("POST %s: %s: %s", path, resp.Status, strings.TrimSpace(string(respBody)))
	}
	if dst == nil {
		return nil
	}
	return json.Unmarshal(respBody, dst)
}

// stateResp is the slice of GET /v0/state this harness reads.
type stateResp struct {
	Sync struct {
		Mode       string `json:"mode"`
		LastError  string `json:"last_error"`
		Collisions int    `json:"collisions"`
		Merges     int    `json:"merges"`
	} `json:"sync"`
	Tasks []struct {
		ID        string `json:"id"`
		Situation string `json:"situation"`
	} `json:"tasks"`
}

func (a *actor) readyTasks() ([]string, error) {
	var st stateResp
	if err := a.get("/v0/state", &st); err != nil {
		return nil, fmt.Errorf("%s state: %w", a.name, err)
	}
	var ready []string
	for _, t := range st.Tasks {
		if t.Situation == core.SituationReady {
			ready = append(ready, t.ID)
		}
	}
	return ready, nil
}

// ---------------------------------------------------------------------
// the MCP session: one live `tuhdoo mcp` per clone
// ---------------------------------------------------------------------

// openSession spawns `tuhdoo mcp --as <principal>` as a child process and
// speaks MCP to it over stdio. One process is one live session, which is
// what keeps the claims it makes leased (T5/T8); the shim is used rather
// than the daemon's /mcp endpoint directly because that is the only
// sanctioned agent door (docs/agent-protocol.md).
func (a *actor) openSession() error {
	logPath := filepath.Join(a.root, ".git", "tuhdoo", "shim.log")
	logf, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	a.shimLog = logf

	cmd := exec.Command(a.bin, "mcp", "--as", a.principal)
	cmd.Dir = a.root
	cmd.Stderr = logf

	client := mcp.NewClient(&mcp.Implementation{Name: "collision-harness", Version: "0"}, nil)
	sess, err := client.Connect(context.Background(), &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		logf.Close()
		return fmt.Errorf("%s: open MCP session: %w (see %s)", a.name, err, logPath)
	}
	a.sess = sess
	return nil
}

func (a *actor) closeSession() {
	if a.sess != nil {
		a.sess.Close()
		a.sess = nil
	}
	if a.shimLog != nil {
		a.shimLog.Close()
		a.shimLog = nil
	}
}

// callTool invokes one MCP tool on this actor's session and decodes the
// structured result into dst. A tool error (IsError) comes back as a
// plain error carrying the daemon's message — the text a real agent
// would read and act on.
func (a *actor) callTool(name string, args map[string]any, dst any) error {
	res, err := a.sess.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return fmt.Errorf("%s on %s: %w", name, a.name, err)
	}
	if res.IsError {
		return fmt.Errorf("%s on %s: %s", name, a.name, toolText(res))
	}
	if dst == nil {
		return nil
	}
	b, err := json.Marshal(res.StructuredContent)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, dst); err != nil {
		return fmt.Errorf("decode %s result %s: %w", name, b, err)
	}
	return nil
}

// toolText flattens a tool error's content into one line.
func toolText(res *mcp.CallToolResult) string {
	var parts []string
	for _, c := range res.Content {
		if t, ok := c.(*mcp.TextContent); ok {
			parts = append(parts, t.Text)
		}
	}
	return flatten(strings.Join(parts, " "))
}

// claimNextOut mirrors the fields of claim_next's result the harness
// needs. Decoded from the tool's structured content rather than typed
// against the daemon's unexported shapes.
type claimNextOut struct {
	Claimed bool   `json:"claimed"`
	Reason  string `json:"reason"`
	Task    *struct {
		Task struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"task"`
		Claim *struct {
			ID      string `json:"id"`
			Actor   string `json:"actor"`
			Machine string `json:"machine"`
		} `json:"claim"`
	} `json:"task"`
}

func (a *actor) claimNext() (claimNextOut, error) {
	var out claimNextOut
	err := a.callTool("claim_next", nil, &out)
	return out, err
}

// claimTask claims one specific task through the session — the
// deliberate half of a storm collision — and returns the claim ID.
func (a *actor) claimTask(task string) (string, error) {
	var out struct {
		Claim *struct {
			ID string `json:"id"`
		} `json:"claim"`
	}
	if err := a.callTool("claim_task", map[string]any{"task": task}, &out); err != nil {
		return "", err
	}
	if out.Claim == nil {
		return "", fmt.Errorf("claim_task on %s returned no claim", a.name)
	}
	return out.Claim.ID, nil
}

// confirmOut is confirm_claim's answer: the referee's verdict.
type confirmOut struct {
	Confirmed bool   `json:"confirmed"`
	Claim     string `json:"claim"`
	Message   string `json:"message"`
}

func (a *actor) confirmClaim(task string) (confirmOut, error) {
	var out confirmOut
	err := a.callTool("confirm_claim", map[string]any{"task": task}, &out)
	return out, err
}

// finishOut is finish_run's answer: what was actually recorded, with
// the referee's statement when that is not what was reported.
type finishOut struct {
	ID      string `json:"id"`
	Outcome string `json:"outcome"`
	Message string `json:"message"`
}

// finishRunDone reports outcome done and returns what the referee
// recorded — done for a certified winner, superseded for a coerced
// loser (D6: agents report what they did; the daemon referees how the
// attempt ended).
func (a *actor) finishRunDone(task, branch, summary string) (finishOut, error) {
	var out finishOut
	err := a.callTool("finish_run", map[string]any{
		"task": task, "outcome": "done", "branch": branch, "summary": summary,
	}, &out)
	return out, err
}

// releaseOut is release_claim's answer; a non-empty message is the
// stand-down acknowledgment for a voided claimant (D6 clause 3).
type releaseOut struct {
	Released string `json:"released"`
	Message  string `json:"message"`
}

func (a *actor) releaseClaim(task, reason string) (releaseOut, error) {
	var out releaseOut
	err := a.callTool("release_claim", map[string]any{"task": task, "reason": reason}, &out)
	return out, err
}

// ---------------------------------------------------------------------
// sync watching
// ---------------------------------------------------------------------

// syncSample is what the harness learned about one daemon's sync loop.
// Collisions and merges are the daemon's own counters (syncer.Status);
// keptMoving counts the times a cycle gave up after maxCycleRetries,
// which only ever appears as a transient LastError — hence the poll.
type syncSample struct {
	collisions int
	merges     int
	keptMoving int
	// refLock counts pushes that lost the remote's ref-update race
	// outright ("cannot lock ref ... is at X but expected Y"). git
	// reports that differently from a stale-history rejection, and
	// gitx.Push only classifies the latter as ErrNonFastForward — see
	// the finding in harness/README.md.
	refLock int
	errors  []string
}

type syncWatch struct {
	mu       sync.Mutex
	samples  map[string]syncSample
	previous map[string]string // last error seen per actor, to count changes not polls
	done     chan struct{}
	once     sync.Once
}

func newSyncWatch(actors []*actor) *syncWatch {
	w := &syncWatch{
		samples:  make(map[string]syncSample, len(actors)),
		previous: make(map[string]string, len(actors)),
		done:     make(chan struct{}),
	}
	go func() {
		for {
			select {
			case <-w.done:
				return
			case <-time.After(500 * time.Millisecond):
				w.sample(actors)
			}
		}
	}()
	return w
}

func (w *syncWatch) sample(actors []*actor) {
	for _, a := range actors {
		var st stateResp
		if a.hc == nil || a.get("/v0/state", &st) != nil {
			continue // a daemon being restarted is not an observation
		}
		w.mu.Lock()
		s := w.samples[a.name]
		s.collisions = st.Sync.Collisions
		s.merges = st.Sync.Merges
		if e := st.Sync.LastError; e != "" && e != w.previous[a.name] {
			s.errors = append(s.errors, flatten(e))
			if strings.Contains(e, "kept moving") {
				s.keptMoving++
			}
			if strings.Contains(e, "cannot lock ref") {
				s.refLock++
			}
		}
		w.previous[a.name] = st.Sync.LastError
		w.samples[a.name] = s
		w.mu.Unlock()
	}
}

func (w *syncWatch) snapshot() map[string]syncSample {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make(map[string]syncSample, len(w.samples))
	for k, v := range w.samples {
		out[k] = v
	}
	return out
}

func (w *syncWatch) stop() { w.once.Do(func() { close(w.done) }) }

// ---------------------------------------------------------------------
// small helpers
// ---------------------------------------------------------------------

func runCmd(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// gitIdentity gives a scratch clone a committer, so `git commit` works
// regardless of what the surrounding machine has configured.
func gitIdentity(root string) error {
	for _, kv := range [][2]string{
		{"user.email", "harness@tuhdoo.invalid"},
		{"user.name", "collision harness"},
	} {
		if out, err := runCmd(root, "git", "config", kv[0], kv[1]); err != nil {
			return fmt.Errorf("git config %s in %s: %w: %s", kv[0], root, err, out)
		}
	}
	return nil
}

// moduleRoot walks up from the working directory to the directory holding
// go.mod — the repo root `go build ./cmd/tuhdoo` needs.
func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod above the working directory; run from the tuhdoo repo")
		}
		dir = parent
	}
}

func claimIDs(attempts []attempt) []string {
	ids := make([]string, 0, len(attempts))
	for _, a := range attempts {
		ids = append(ids, a.claim)
	}
	return ids
}

func sameTask(as []attempt) bool {
	for _, a := range as[1:] {
		if a.task != as[0].task {
			return false
		}
	}
	return true
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func round1s(d time.Duration) time.Duration { return d.Round(time.Second) }

// flatten folds a multi-line git error into one readable line.
func flatten(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
