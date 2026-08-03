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
//  5. fires claim_next from both actors behind a barrier, repeatedly;
//  6. lets both daemons converge, then records the D6 loser handling
//     (a superseded run per voided claim) and finishes the winners;
//  7. verifies convergence and the winner rule, and prints a report.
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
	gap      time.Duration
	converge time.Duration
	keep     bool
}

func main() {
	var cfg config
	flag.IntVar(&cfg.rounds, "rounds", 10, "claim rounds to fire (one contested task per round)")
	flag.IntVar(&cfg.spare, "spare", 3, "extra seeded tasks beyond the rounds, so the pool never runs dry")
	flag.IntVar(&cfg.storm, "storm", 40, "simultaneous eager-write bursts aimed at the sync loop's push cycle")
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
	tasks     []string // seeded claimable task IDs, creation order
	carrier   string   // held task the sync-storm escalations hang off
	stormTook time.Duration
	watch     *syncWatch
	closing   sync.Once
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

// attempt is one actor's claim_next call in one round.
type attempt struct {
	round  int
	actor  *actor
	task   string
	claim  string
	branch string // the branch this attempt would have worked on
}

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

	attempts, races, err := l.race()
	if err != nil {
		return err
	}

	if err := l.storm(); err != nil {
		return err
	}

	// The losers cannot stand down until their own daemons have replayed
	// the merged claim set, so the claims have to have crossed before
	// settle runs.
	if err := l.converge("post-race", claimIDs(attempts)); err != nil {
		return err
	}

	runs, err := l.settle(attempts)
	if err != nil {
		return err
	}

	// Sessions closed before the final convergence: lease renewal is what
	// keeps writing after the work stops, and a renewal landing mid-check
	// would move one tree under the comparison.
	fmt.Println("== closing MCP sessions")
	for _, a := range l.actors {
		a.closeSession()
	}

	if err := l.converge("final", runs); err != nil {
		return err
	}

	return l.verify(attempts, races)
}

// seed creates the scratch task pool on the first clone and hands it to
// the remote. The daemon is stopped once at the end: its shutdown flushes
// pending events and runs a final sync cycle, which is what publishes the
// seed without waiting out the 60s fetch cadence (T8).
func (l *lab) seed(a *actor) error {
	fmt.Printf("== seeding %d tasks on %s\n", l.cfg.rounds+l.cfg.spare, a.name)
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
	var created struct {
		IDs []string `json:"ids"`
	}
	if err := a.post("/v0/tasks", a.principal, items, &created); err != nil {
		return fmt.Errorf("seed tasks: %w", err)
	}
	l.carrier, l.tasks = created.IDs[0], created.IDs[1:]

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
	for _, x := range l.actors {
		ready, err := x.readyTasks()
		if err != nil {
			return err
		}
		if len(ready) != len(l.tasks) {
			return fmt.Errorf("%s sees %d ready tasks, seeded %d", x.name, len(ready), len(l.tasks))
		}
	}
	fmt.Printf("   both machines see %d ready tasks\n", len(l.tasks))
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
					branch: fmt.Sprintf("%s/round-%02d", event.ShortID(out.Task.Task.ID), round),
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

// settle plays out D6's loser handling. Replay has already picked the
// winner of every race; each actor now closes its own attempt: a voided
// claim becomes a run with outcome "superseded" carrying the branch the
// work would have lived on, an active claim becomes an ordinary "done".
//
// The superseded run is written over the daemon's HTTP API, not through
// the MCP session, because the MCP surface rejects the outcome as
// "daemon-synthesized" — see the finding in harness/README.md: nothing in
// the daemon actually synthesizes it, so the loser's supervisor is the
// only writer there is.
func (l *lab) settle(attempts []attempt) ([]string, error) {
	fmt.Println("== recording outcomes (superseded for race losers, done for winners)")
	state, _, err := replayClone(l.actors[0].root, time.Now())
	if err != nil {
		return nil, err
	}
	var runs []string
	superseded, done := 0, 0
	for _, at := range attempts {
		c := state.Claims[at.claim]
		if c == nil {
			return nil, fmt.Errorf("claim %s (%s, round %d) is missing from replayed state", at.claim, at.actor.name, at.round)
		}
		body := map[string]any{"task": at.task, "branch": at.branch}
		switch c.Status {
		case core.ClaimVoided:
			body["outcome"] = "superseded"
			body["summary"] = fmt.Sprintf("lost the D6 race on round %d; work on %s is salvageable", at.round, at.branch)
			superseded++
		case core.ClaimActive:
			body["outcome"] = "done"
			body["summary"] = fmt.Sprintf("won the D6 race on round %d", at.round)
			done++
		default:
			return nil, fmt.Errorf("claim %s (%s, round %d) ended %s — expected active or voided",
				at.claim, at.actor.name, at.round, c.Status)
		}
		var out struct {
			ID string `json:"id"`
		}
		if err := at.actor.post("/v0/runs", at.actor.principal, body, &out); err != nil {
			return nil, fmt.Errorf("finish %s attempt on %s: %w", at.actor.name, event.ShortID(at.task), err)
		}
		runs = append(runs, out.ID)
	}
	fmt.Printf("   %d superseded, %d done\n", superseded, done)
	return runs, nil
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

func (l *lab) verify(attempts []attempt, races int) error {
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

	// --- the D6 winner rule ---
	voided := voidedClaims(stateA)
	c.check(races > 0, "claim races observed (%d rounds where both actors claimed the same task)", races)
	c.check(len(voided) > 0, "claims voided by the winner rule (%d)", len(voided))

	crossMachine := 0
	winnerOK, supersededOK := true, true
	branches := make(map[string]string, len(attempts))
	actorOf := make(map[string]*actor, len(attempts))
	for _, at := range attempts {
		branches[at.claim] = at.branch
		actorOf[at.claim] = at.actor
	}
	for _, loser := range voided {
		// Exactly one claim survives on the task, and it is the earliest
		// by ULID — D6's rule, re-derived here rather than trusted.
		var survivors []*core.Claim
		earliest := ""
		for _, cid := range stateA.ClaimsByTask[loser.Task] {
			cl := stateA.Claims[cid]
			if cl.Status != core.ClaimVoided {
				survivors = append(survivors, cl)
			}
			if earliest == "" || cid < earliest {
				earliest = cid
			}
		}
		if len(survivors) != 1 || survivors[0].ID != earliest || loser.ID <= earliest {
			winnerOK = false
			fmt.Printf("        task %s: %d survivors, earliest claim %s, loser %s\n",
				event.ShortID(loser.Task), len(survivors), event.ShortID(earliest), event.ShortID(loser.ID))
		} else if survivors[0].Machine != loser.Machine {
			crossMachine++
		}

		// D6 clause 2: the loser's half-done work is recorded as a run
		// with outcome superseded, branch included.
		found := false
		for _, r := range stateA.Runs {
			if r.Task == loser.Task && r.Actor == loser.Actor &&
				r.Outcome == "superseded" && r.Branch == branches[loser.ID] && r.Branch != "" {
				found = true
				break
			}
		}
		if !found {
			supersededOK = false
			fmt.Printf("        no superseded run carrying branch %q for %s on task %s\n",
				branches[loser.ID], loser.Actor, event.ShortID(loser.Task))
		}
	}
	c.check(winnerOK, "every voided claim leaves exactly one winner, earliest ULID (%d checked)", len(voided))
	c.check(crossMachine == len(voided), "every race crossed machines (%d of %d)", crossMachine, len(voided))
	c.check(supersededOK, "a superseded run carrying the loser's branch for every voided claim")

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

	l.report(attempts, races, voided, merges, watch, exhausted)

	if c.failed > 0 {
		return fmt.Errorf("%d acceptance check(s) failed", c.failed)
	}
	return nil
}

func (l *lab) report(attempts []attempt, races int, voided []*core.Claim, merges int,
	watch map[string]syncSample, exhausted int) {

	nonFF := l.total(watch, func(w syncSample) int { return w.collisions })
	built := l.total(watch, func(w syncSample) int { return w.merges })
	refLock := l.total(watch, func(w syncSample) int { return w.refLock })

	fmt.Println("\n== numbers")
	fmt.Println("   (\"built\" counts merge commits the syncer constructed; a merge whose ref")
	fmt.Println("    update or push loses a race is discarded, so fewer land on the branch)")
	fmt.Printf("   claim rounds fired            %d\n", l.cfg.rounds)
	fmt.Printf("   claim races observed          %d\n", races)
	fmt.Printf("   claims made                   %d\n", len(attempts))
	fmt.Printf("   claims voided (D6 losers)     %d\n", len(voided))
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
	res, err := a.sess.CallTool(context.Background(), &mcp.CallToolParams{Name: "claim_next"})
	if err != nil {
		return out, err
	}
	if res.IsError {
		return out, fmt.Errorf("claim_next reported an error: %v", res.Content)
	}
	b, err := json.Marshal(res.StructuredContent)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return out, fmt.Errorf("decode claim_next result %s: %w", b, err)
	}
	return out, nil
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
