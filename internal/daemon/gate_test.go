package daemon

// Integration tests for the D6 confirmation gate (2026-08-04): the
// collision-harness shape in miniature — two real daemons on two clones
// of one bare remote racing confirm_claim — plus the T2 remoteless and
// unreachable-remote arms. Everything asserted is machine-checked
// against the real branch trees, not daemon caches.

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brandonbews/tuhdoo/internal/event"
	"github.com/brandonbews/tuhdoo/internal/gitx"
	"github.com/brandonbews/tuhdoo/internal/syncer"
)

// gateOpts keeps the background sync loop out of the way (interval far
// beyond the test) so every fetch/push in these tests is either the
// gate's own or an explicit Cycle — eager-claim pokes still fire, as
// they do in production.
func gateOpts() Options {
	return Options{
		Quiet:        25 * time.Millisecond,
		SyncInterval: time.Hour,
		Log:          log.New(io.Discard, "", 0),
	}
}

// twoDaemonRemote wires the collision-harness scaffold shared by the
// two-daemon race tests: a bare origin, two clones of it, a daemon on
// each. B joins from A's published data branch root (the harness's
// joinSecondClone shape), so the pair share one history and every
// merge is a race merge.
func twoDaemonRemote(t *testing.T) (dA, dB *Daemon, rootA, rootB string) {
	t.Helper()
	setGitEnv(t)
	base := shortTempDir(t)
	bare := filepath.Join(base, "origin.git")
	runGit(t, base, "init", "--quiet", "--bare", "-b", "main", bare)
	clone := func(name string) string {
		root := filepath.Join(base, name)
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
		runGit(t, root, "init", "--quiet", "-b", "main")
		runGit(t, root, "remote", "add", "origin", bare)
		return root
	}
	rootA, rootB = clone("alpha"), clone("bravo")

	dA, _ = startDaemonAt(t, rootA, gateOpts())
	mustCycle(t, dA) // publish A's data branch root
	runGit(t, rootB, "fetch", "--quiet", "origin", "refs/heads/tuhdoo:refs/heads/tuhdoo")
	dB, _ = startDaemonAt(t, rootB, gateOpts())
	return dA, dB, rootA, rootB
}

// confirmedEvents returns the claim.confirmed events per task in evs.
func confirmedEvents(t *testing.T, evs []event.Event) map[string][]event.ClaimConfirmed {
	t.Helper()
	out := make(map[string][]event.ClaimConfirmed)
	for _, e := range evs {
		if e.Type != event.TypeClaimConfirmed {
			continue
		}
		var p event.ClaimConfirmed
		unmarshalInto(t, e.Data, &p)
		out[e.Task] = append(out[e.Task], p)
	}
	return out
}

// TestConfirmClaimRemoteless: no remote configured is a normal state
// (T2) — the daemon is the sole writer, so confirmation is locally
// sound, instant, and idempotent.
func TestConfirmClaimRemoteless(t *testing.T) {
	d, c := startDaemon(t)
	const actor = "brandon/impl-1"
	task := createOne(t, c, "brandon", map[string]any{"title": "solo work"})

	h, oe := d.opClaimTask(actor, task)
	if oe != nil {
		t.Fatalf("claim: %v", oe)
	}

	res, oe := d.opConfirmClaim(actor, task)
	if oe != nil {
		t.Fatalf("confirm: %v", oe)
	}
	if !res.Confirmed || res.Claim != h.Claim.ID {
		t.Fatalf("confirm = %+v, want confirmed for claim %s", res, h.Claim.ID)
	}
	if !strings.Contains(res.Message, "irrevocably") {
		t.Fatalf("confirmed message should say the verdict is final, got %q", res.Message)
	}

	// Idempotent: the second call answers instantly and writes nothing.
	again, oe := d.opConfirmClaim(actor, task)
	if oe != nil || !again.Confirmed {
		t.Fatalf("re-confirm = %+v, %v — want idempotent confirmed", again, oe)
	}
	confs := confirmedEvents(t, flushedEvents(t, d))
	if got := confs[task]; len(got) != 1 || got[0].Claim != h.Claim.ID {
		t.Fatalf("ledger carries %+v confirmations for the task, want exactly one for claim %s", got, h.Claim.ID)
	}
}

// TestConfirmClaimUnreachableRemoteRefusesHonestly: with a remote
// configured but unreachable the referee refuses with a retryable
// error and writes nothing — it never guesses (D6: accuracy outranks
// liveness).
func TestConfirmClaimUnreachableRemoteRefusesHonestly(t *testing.T) {
	setGitEnv(t)
	root := shortTempDir(t)
	runGit(t, root, "init", "--quiet", "-b", "main")
	runGit(t, root, "remote", "add", "origin", filepath.Join(root, "no-such-remote.git"))
	d, _ := startDaemonAt(t, root, gateOpts())

	const actor = "brandon/impl-1"
	ids, _, oe := d.opCreateTasks("brandon", []createTaskItem{{Title: "stranded work"}})
	if oe != nil {
		t.Fatalf("create: %v", oe)
	}
	task := ids[0]
	if _, oe := d.opClaimTask(actor, task); oe != nil {
		t.Fatalf("claim: %v", oe)
	}

	_, oe = d.opConfirmClaim(actor, task)
	if oe == nil {
		t.Fatal("confirm against an unreachable remote must refuse, not guess")
	}
	if oe.code != http.StatusServiceUnavailable {
		t.Fatalf("refusal code = %d, want %d (retryable)", oe.code, http.StatusServiceUnavailable)
	}
	if len(confirmedEvents(t, flushedEvents(t, d))) != 0 {
		t.Fatal("an honest refusal must write nothing")
	}
}

// TestConfirmClaimTwoDaemonsOneWinner is the collision harness in test
// form: two daemons, one bare remote, both racing confirm_claim on the
// same task, round after round. Exactly one confirmation per contest
// lands, every time; the loser is told it lost; zero duplicates across
// the run.
func TestConfirmClaimTwoDaemonsOneWinner(t *testing.T) {
	dA, dB, rootA, rootB := twoDaemonRemote(t)
	if dA.machine == dB.machine {
		t.Fatalf("both clones minted machine id %s — they are not two machines", dA.machine)
	}

	const actorA, actorB = "harness/alpha", "harness/bravo"
	const rounds = 5
	tasks := make([]string, 0, rounds)
	claims := make(map[string]string, 2*rounds) // actor:task → claim ID

	for round := 1; round <= rounds; round++ {
		ids, _, oe := dA.opCreateTasks("seeder", []createTaskItem{{
			Title:       fmt.Sprintf("collision bait %02d", round),
			Description: "No work here: claiming and confirming it is the whole run.",
		}})
		if oe != nil {
			t.Fatalf("round %d: seed: %v", round, oe)
		}
		task := ids[0]
		tasks = append(tasks, task)
		if err := dA.batcher.Flush(); err != nil {
			t.Fatalf("round %d: flush: %v", round, err)
		}
		mustCycle(t, dA)
		mustCycle(t, dB)

		// Both daemons claim behind one starting gun — a genuine D6
		// race: neither has fetched the other's claim when it claims.
		type claimOut struct {
			claim string
			err   error
		}
		outs := make([]claimOut, 2)
		var wg sync.WaitGroup
		gun := make(chan struct{})
		for i, side := range []struct {
			d     *Daemon
			actor string
		}{{dA, actorA}, {dB, actorB}} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-gun
				h, oe := side.d.opClaimTask(side.actor, task)
				if oe != nil {
					outs[i] = claimOut{err: oe}
					return
				}
				outs[i] = claimOut{claim: h.Claim.ID}
			}()
		}
		close(gun)
		wg.Wait()
		for i, out := range outs {
			if out.err != nil {
				t.Fatalf("round %d: claim %d: %v", round, i, out.err)
			}
		}
		claims[actorA+":"+task] = outs[0].claim
		claims[actorB+":"+task] = outs[1].claim

		// Both race the gate, behind the same gun. A transient 503
		// ("remote kept moving") is an honest retryable answer under
		// contention — retried here the way a real agent would.
		type confirmOut struct {
			res confirmClaimResult
			err error
		}
		couts := make([]confirmOut, 2)
		var cwg sync.WaitGroup
		cgun := make(chan struct{})
		for i, side := range []struct {
			d     *Daemon
			actor string
		}{{dA, actorA}, {dB, actorB}} {
			cwg.Add(1)
			go func() {
				defer cwg.Done()
				<-cgun
				for tries := 0; tries < 5; tries++ {
					res, oe := side.d.opConfirmClaim(side.actor, task)
					if oe == nil {
						couts[i] = confirmOut{res: res}
						return
					}
					if oe.code != http.StatusServiceUnavailable {
						couts[i] = confirmOut{err: oe}
						return
					}
					time.Sleep(100 * time.Millisecond)
				}
				couts[i] = confirmOut{err: fmt.Errorf("gate still retryable after 5 tries")}
			}()
		}
		close(cgun)
		cwg.Wait()

		winners := 0
		for i, out := range couts {
			if out.err != nil {
				t.Fatalf("round %d: confirm %d: %v", round, i, out.err)
			}
			if out.res.Confirmed {
				winners++
			} else if !strings.Contains(out.res.Message, "Stand down") {
				t.Fatalf("round %d: loser's answer must say to stand down, got %q", round, out.res.Message)
			}
		}
		if winners != 1 {
			t.Fatalf("round %d: %d confirmed verdicts, want exactly one (results: %+v)", round, winners, couts)
		}

		// Let the round settle before the next: both clones on one tree.
		convergeTrees(t, rootA, rootB, dA, dB)
	}

	// Zero duplicates across the run, verified on the branch itself:
	// exactly one confirmation per task (one contest each), naming a
	// claim one of the two racers actually made.
	events, err := dA.store.LoadEvents()
	if err != nil {
		t.Fatal(err)
	}
	confs := confirmedEvents(t, events)
	for _, task := range tasks {
		got := confs[task]
		if len(got) != 1 {
			t.Fatalf("task %s carries %d confirmations, want exactly one", task, len(got))
		}
		if got[0].Claim != claims[actorA+":"+task] && got[0].Claim != claims[actorB+":"+task] {
			t.Fatalf("task %s confirmed claim %s, which neither racer made", task, got[0].Claim)
		}
	}
}

// movingRemoteGit wraps the daemon's real git and advances the bare
// remote's data branch just before delegating each push — the
// deterministic worst case of D6 contention: a peer lands between every
// fetch and push, so every push loses the remote's ref CAS for real
// (the rejection travels the same gitx classification as production).
type movingRemoteGit struct {
	gitx.Git
	t     *testing.T
	bare  string
	moves int
}

func (m *movingRemoteGit) Push(remote, refspec string) error {
	m.moves++
	head := strings.TrimSpace(runGit(m.t, m.bare, "rev-parse", "refs/heads/tuhdoo"))
	tree := strings.TrimSpace(runGit(m.t, m.bare, "rev-parse", "refs/heads/tuhdoo^{tree}"))
	next := strings.TrimSpace(runGit(m.t, m.bare, "commit-tree", "-p", head,
		"-m", fmt.Sprintf("peer move %d", m.moves), tree))
	runGit(m.t, m.bare, "update-ref", "refs/heads/tuhdoo", next)
	return m.Git.Push(remote, refspec)
}

// TestConfirmClaimGateRetryExhaustion: when the remote moves between
// every fetch and push, the gate's bounded loop gives up after exactly
// confirmGateRetries attempts with a retryable 503 and writes nothing.
// The exact wording is production tooling's contract: the collision
// harness classifies honest retryable refusals by string-matching
// "remote kept moving" (harness/collision/main.go, gateRetryable).
func TestConfirmClaimGateRetryExhaustion(t *testing.T) {
	setGitEnv(t)
	base := shortTempDir(t)
	bare := filepath.Join(base, "origin.git")
	runGit(t, base, "init", "--quiet", "--bare", "-b", "main", bare)
	root := filepath.Join(base, "clone")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "init", "--quiet", "-b", "main")
	runGit(t, root, "remote", "add", "origin", bare)

	// Run is never called, so no background loop ever touches d.sync:
	// the test drives ops directly and swaps the syncer for one whose
	// pushes always lose the CAS. (New-without-Run is the successor
	// idiom from TestShutdownCleansUp.)
	d, err := New(root, gateOpts())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { d.Shutdown("test cleanup") })
	mustCycle(t, d) // publish the data branch: the bare now has a head to move

	const actor = "brandon/impl-1"
	ids, _, oe := d.opCreateTasks("brandon", []createTaskItem{{Title: "contested to exhaustion"}})
	if oe != nil {
		t.Fatalf("create: %v", oe)
	}
	task := ids[0]
	if _, oe := d.opClaimTask(actor, task); oe != nil {
		t.Fatalf("claim: %v", oe)
	}

	g, err := gitx.New(root)
	if err != nil {
		t.Fatalf("gitx.New: %v", err)
	}
	moving := &movingRemoteGit{Git: g, t: t, bare: bare}
	d.sync = syncer.New(moving, syncer.Options{
		Interval: time.Hour,
		Ident:    testIdent,
		OnMerged: func() {
			if err := d.Refresh(); err != nil {
				t.Errorf("refresh after gate reconcile: %v", err)
			}
		},
		Log: log.New(io.Discard, "", 0),
	})

	_, oe = d.opConfirmClaim(actor, task)
	if oe == nil {
		t.Fatal("gate against a remote that always moves must exhaust, not confirm")
	}
	if oe.code != http.StatusServiceUnavailable {
		t.Fatalf("exhaustion code = %d, want %d (retryable)", oe.code, http.StatusServiceUnavailable)
	}
	want := fmt.Sprintf("remote kept moving for %d attempts", confirmGateRetries)
	if !strings.Contains(oe.Error(), want) {
		t.Fatalf("exhaustion message %q does not carry %q — the collision harness greps that string", oe.Error(), want)
	}
	if moving.moves != confirmGateRetries {
		t.Fatalf("gate pushed %d times, want exactly confirmGateRetries = %d", moving.moves, confirmGateRetries)
	}
	// An honest exhaustion certifies nothing: no confirmation on the
	// ledger — the loop's built-but-rejected commits touched no ref.
	if n := len(confirmedEvents(t, flushedEvents(t, d))); n != 0 {
		t.Fatalf("exhausted gate left %d confirmations on the ledger, want none", n)
	}
}

// TestConfirmGateRetriesMirrorsSyncerCycle pins confirmGateRetries to
// the literal syncer.maxCycleRetries is pinned to (its twin lives in
// internal/syncer/syncer_test.go): the mirroring is claimed by comment
// in ops.go and enforced nowhere else, and neither package can read the
// other's unexported constant — so each pins its own, and either one
// drifting trips a test naming the other.
func TestConfirmGateRetriesMirrorsSyncerCycle(t *testing.T) {
	if confirmGateRetries != 4 {
		t.Fatalf("confirmGateRetries = %d, want 4 — it mirrors syncer.maxCycleRetries; change both constants and both pinning tests together", confirmGateRetries)
	}
}

// TestMCPConfirmClaimTool: the twelfth tool end to end — session-held
// claims only, and a tool result an agent can act on without docs.
func TestMCPConfirmClaimTool(t *testing.T) {
	d, c := startDaemon(t)
	cs := mcpConnect(t, d, "brandon/impl-1", nil)

	held := createOne(t, c, "brandon", map[string]any{"title": "mine to confirm"})
	unheld := createOne(t, c, "brandon", map[string]any{"title": "never claimed here"})

	var claimed claimNextResult
	mustToolOK(t, cs, "claim_task", map[string]any{"task": held}, &claimed.Task)

	// Confirming a task this session never claimed is refused at the
	// tool: the gate certifies the session's own claim.
	res := callTool(t, cs, "confirm_claim", map[string]any{"task": unheld})
	if !res.IsError || !strings.Contains(contentText(res), "holds no claim") {
		t.Fatalf("confirming an unheld task = %v %q, want a holds-no-claim error", res.IsError, contentText(res))
	}

	// The held claim confirms (remoteless: instantly), with marching
	// orders in the result.
	var verdict confirmClaimResult
	mustToolOK(t, cs, "confirm_claim", map[string]any{"task": held}, &verdict)
	if !verdict.Confirmed || !strings.Contains(verdict.Message, "Merge freely") {
		t.Fatalf("verdict = %+v, want confirmed with merge-freely guidance", verdict)
	}
	confs := confirmedEvents(t, flushedEvents(t, d))
	if len(confs[held]) != 1 || len(confs[unheld]) != 0 {
		t.Fatalf("confirmations on ledger = %v, want exactly one on the held task", confs)
	}
}

// mustCycle runs one sync cycle, tolerating the bounded-retry loop's
// transient "kept moving" verdict under contention.
func mustCycle(t *testing.T, d *Daemon) {
	t.Helper()
	var err error
	for tries := 0; tries < 5; tries++ {
		if err = d.sync.Cycle(); err == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("cycle: %v", err)
}

// convergeTrees cycles both daemons until their data branches point at
// the identical tree.
func convergeTrees(t *testing.T, rootA, rootB string, dA, dB *Daemon) {
	t.Helper()
	tree := func(root string) string {
		return strings.TrimSpace(runGit(t, root, "rev-parse", "refs/heads/tuhdoo^{tree}"))
	}
	for tries := 0; tries < 20; tries++ {
		if tree(rootA) == tree(rootB) {
			return
		}
		mustCycle(t, dA)
		mustCycle(t, dB)
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("clones never converged: trees %s vs %s", tree(rootA), tree(rootB))
}
