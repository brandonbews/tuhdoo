package daemon

// The loser's whole story (D6 clause 3, 2026-08-04): losers learn at
// call-time, a finish on a lost attempt is coerced to superseded with
// the reported links kept for salvage, a loser that never reports is
// closed by replay's synthesized superseded run at lease expiry (after
// which a late finish is turned away toward add_note), and a voided
// claimant's release_claim is an acknowledged stand-down, not an error.

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brandonbews/tuhdoo/internal/core"
	"github.com/brandonbews/tuhdoo/internal/event"
)

// mintVoidedClaim writes a claim.made for actor directly — the way a
// peer's claim arrives by sync — and leases it for leaseFor from now,
// exactly as the peer's daemon would have. Minted after someone already
// holds the task, replay voids it: the single-daemon way to stage a
// race loser. A negative leaseFor stages a loser whose report window
// has already closed.
func mintVoidedClaim(t *testing.T, d *Daemon, task, actor string, leaseFor time.Duration) string {
	t.Helper()
	d.mu.Lock()
	ev, err := d.newEventLocked(event.TypeClaimMade, actor, task, event.ClaimMade{})
	if err == nil {
		err = d.commitLocked(false, ev)
	}
	d.mu.Unlock()
	if err != nil {
		t.Fatalf("mint claim by %s on %s: %v", actor, task, err)
	}
	if err := d.store.WriteLease(ev.ID, time.Now().Add(leaseFor)); err != nil {
		t.Fatalf("lease minted claim: %v", err)
	}
	if err := d.Refresh(); err != nil {
		t.Fatalf("refresh after minting claim: %v", err)
	}
	return ev.ID
}

// runEvents returns the run.finished payloads per task in evs.
func runEvents(t *testing.T, evs []event.Event) map[string][]event.RunFinished {
	t.Helper()
	out := make(map[string][]event.RunFinished)
	for _, e := range evs {
		if e.Type != event.TypeRunFinished {
			continue
		}
		var p event.RunFinished
		unmarshalInto(t, e.Data, &p)
		out[e.Task] = append(out[e.Task], p)
	}
	return out
}

// TestFinishDoneByRaceLoserCoercedToSuperseded: the referee at the
// finish. A loser reporting done records superseded — links kept as the
// salvage record, the result saying so plainly — while the winner's
// done stands (refereed through the gate, which remoteless is the T2
// local arm writing the claim.confirmed itself).
func TestFinishDoneByRaceLoserCoercedToSuperseded(t *testing.T) {
	d, c := startDaemon(t)
	const winner, loser = "brandon/win", "brandon/lose"
	task := createOne(t, c, "brandon", map[string]any{"title": "raced work"})

	hw, oe := d.opClaimTask(winner, task)
	if oe != nil {
		t.Fatalf("winner claim: %v", oe)
	}
	mintVoidedClaim(t, d, task, loser, time.Hour)

	res, oe := d.opFinishRun(loser, finishRunReq{
		Task: task, Outcome: event.OutcomeDone,
		Branch: "tuh-x/lost-work", PR: "https://example.com/pr/7",
		Commits: []string{"abc1234"}, Summary: "built it all, then lost the race",
	})
	if oe != nil {
		t.Fatalf("loser finish: %v", oe)
	}
	if res.Outcome != event.OutcomeSuperseded {
		t.Fatalf("loser's recorded outcome = %q, want superseded", res.Outcome)
	}
	for _, want := range []string{"Recorded as superseded, not done", "close any open PR", "salvage"} {
		if !strings.Contains(res.Message, want) {
			t.Fatalf("loser's result message %q missing %q", res.Message, want)
		}
	}

	wres, oe := d.opFinishRun(winner, finishRunReq{
		Task: task, Outcome: event.OutcomeDone, Summary: "merged",
	})
	if oe != nil {
		t.Fatalf("winner finish: %v", oe)
	}
	if wres.Outcome != event.OutcomeDone || wres.Message != "" {
		t.Fatalf("winner's record = %+v, want a plain done", wres)
	}

	// The ledger: the loser's run stored superseded with every reported
	// link kept; the winner's stored done, certified by exactly one
	// claim.confirmed naming the winner's claim; the task closed done.
	events := flushedEvents(t, d)
	runs := runEvents(t, events)[task]
	if len(runs) != 2 {
		t.Fatalf("stored runs = %+v, want the loser's and the winner's", runs)
	}
	lost := runs[0] // loser finished first
	if lost.Outcome != event.OutcomeSuperseded || lost.Branch != "tuh-x/lost-work" ||
		lost.PR != "https://example.com/pr/7" || len(lost.Commits) != 1 ||
		lost.Summary != "built it all, then lost the race" {
		t.Fatalf("loser's stored run = %+v, want superseded with all reported links kept", lost)
	}
	if runs[1].Outcome != event.OutcomeDone {
		t.Fatalf("winner's stored run = %+v, want done", runs[1])
	}
	confs := confirmedEvents(t, events)[task]
	if len(confs) != 1 || confs[0].Claim != hw.Claim.ID {
		t.Fatalf("confirmations = %+v, want exactly one for the winner's claim %s", confs, hw.Claim.ID)
	}
	if err := d.Refresh(); err != nil {
		t.Fatal(err)
	}
	d.mu.Lock()
	status := d.state.Tasks[task].Status
	d.mu.Unlock()
	if status != core.StatusDone {
		t.Fatalf("task status = %q, want done", status)
	}
}

// TestFinishDoneByConfirmedWinnerRecordsDone: a winner that followed
// the protocol — confirm_claim before merging — records done through
// the fast path: the confirmation is irrevocable, so the finish needs
// no second round-trip and mints no second claim.confirmed.
func TestFinishDoneByConfirmedWinnerRecordsDone(t *testing.T) {
	d, c := startDaemon(t)
	const actor = "brandon/impl-1"
	task := createOne(t, c, "brandon", map[string]any{"title": "clean win"})

	h, oe := d.opClaimTask(actor, task)
	if oe != nil {
		t.Fatalf("claim: %v", oe)
	}
	if res, oe := d.opConfirmClaim(actor, task); oe != nil || !res.Confirmed {
		t.Fatalf("confirm = %+v, %v — want confirmed", res, oe)
	}

	res, oe := d.opFinishRun(actor, finishRunReq{
		Task: task, Outcome: event.OutcomeDone, Summary: "merged",
	})
	if oe != nil {
		t.Fatalf("finish: %v", oe)
	}
	if res.Outcome != event.OutcomeDone || res.Message != "" {
		t.Fatalf("record = %+v, want a plain done", res)
	}

	events := flushedEvents(t, d)
	confs := confirmedEvents(t, events)[task]
	if len(confs) != 1 || confs[0].Claim != h.Claim.ID {
		t.Fatalf("confirmations = %+v, want exactly the one confirm_claim minted", confs)
	}
	runs := runEvents(t, events)[task]
	if len(runs) != 1 || runs[0].Outcome != event.OutcomeDone {
		t.Fatalf("stored runs = %+v, want one done", runs)
	}
}

// TestLateLoserFinishRejectedWithAddNotePointer: once a voided claim's
// lease expires unreported, replay has closed the attempt with its
// synthesized superseded run — one close per attempt — and a late
// finish is turned away toward add_note for branch salvage.
func TestLateLoserFinishRejectedWithAddNotePointer(t *testing.T) {
	d, c := startDaemon(t)
	const winner, loser = "brandon/win", "brandon/lose"
	task := createOne(t, c, "brandon", map[string]any{"title": "raced work"})

	if _, oe := d.opClaimTask(winner, task); oe != nil {
		t.Fatalf("winner claim: %v", oe)
	}
	loserClaim := mintVoidedClaim(t, d, task, loser, -time.Minute)

	_, oe := d.opFinishRun(loser, finishRunReq{
		Task: task, Outcome: event.OutcomeDone, Branch: "tuh-x/lost-work",
	})
	if oe == nil {
		t.Fatal("late finish succeeded, want rejection: the attempt is closed forever")
	}
	if oe.code != http.StatusConflict {
		t.Fatalf("late finish code = %d, want %d", oe.code, http.StatusConflict)
	}
	for _, want := range []string{"closed as superseded", "add_note"} {
		if !strings.Contains(oe.msg, want) {
			t.Fatalf("late-finish rejection %q missing %q", oe.msg, want)
		}
	}

	// The synthesized close is visible in hydration, branch-less and
	// tied to the loser's claim.
	h, hoe := d.opGetTask(task)
	if hoe != nil {
		t.Fatal(hoe)
	}
	var synth *runJSON
	for i := range h.Runs {
		if h.Runs[i].Synthesized {
			synth = &h.Runs[i]
		}
	}
	if synth == nil || synth.Outcome != event.OutcomeSuperseded ||
		synth.Claim != loserClaim || synth.Branch != "" {
		t.Fatalf("synthesized close = %+v, want a branch-less superseded run for claim %s", synth, loserClaim)
	}

	// Salvage still works — and carries no stand-down nag, the loss
	// already being on the record.
	id, warning, noe := d.opAddNote(loser, task, "salvage: branch tuh-x/lost-work has the CSS fix")
	if noe != nil || id == "" {
		t.Fatalf("salvage add_note = %q, %v — want success", id, noe)
	}
	if warning != "" {
		t.Fatalf("salvage note warning = %q, want none once the attempt is closed", warning)
	}
}

// TestReleaseByVoidedClaimantIsAcknowledgedStandDown: standing down is
// exactly what the loser was asked to do, so release_claim from a
// voided claimant succeeds as acknowledgment — reason on the ledger,
// lease gone, the attempt closed by the synthesized superseded run —
// and the winner's hold is untouched.
func TestReleaseByVoidedClaimantIsAcknowledgedStandDown(t *testing.T) {
	d, c := startDaemon(t)
	const winner, loser = "brandon/win", "brandon/lose"
	task := createOne(t, c, "brandon", map[string]any{"title": "raced work"})

	if _, oe := d.opClaimTask(winner, task); oe != nil {
		t.Fatalf("winner claim: %v", oe)
	}
	loserClaim := mintVoidedClaim(t, d, task, loser, time.Hour)

	claimID, message, oe := d.opReleaseClaim(loser, task, "peer won; standing down")
	if oe != nil {
		t.Fatalf("loser release = %v, want acknowledged stand-down", oe)
	}
	if claimID != loserClaim {
		t.Fatalf("acknowledged claim = %s, want the loser's %s", claimID, loserClaim)
	}
	for _, want := range []string{"Stand-down acknowledged", "superseded", "add_note"} {
		if !strings.Contains(message, want) {
			t.Fatalf("stand-down ack %q missing %q", message, want)
		}
	}

	// The stand-down closed the attempt: lease tombstoned at the
	// stand-down instant, superseded run synthesized, and a later
	// finish is turned away.
	d.mu.Lock()
	winnerClaim := d.state.ActiveClaim(task)
	var synthesized bool
	for i := range d.state.Runs {
		if r := &d.state.Runs[i]; r.Synthesized && r.Claim == loserClaim && r.Outcome == event.OutcomeSuperseded {
			synthesized = true
		}
	}
	d.mu.Unlock()
	if !synthesized {
		t.Fatal("stand-down did not close the attempt with a synthesized superseded run")
	}
	if winnerClaim == nil || winnerClaim.Actor != winner {
		t.Fatalf("winner's hold = %+v, want undisturbed", winnerClaim)
	}
	if _, oe := d.opFinishRun(loser, finishRunReq{Task: task, Outcome: event.OutcomeFailed}); oe == nil ||
		!strings.Contains(oe.msg, "add_note") {
		t.Fatalf("finish after stand-down = %v, want the add_note pointer", oe)
	}

	// The reason landed as a real claim.released event from the loser.
	var released bool
	for _, e := range flushedEvents(t, d) {
		if e.Type == event.TypeClaimReleased && e.Actor == loser && e.Task == task {
			released = true
		}
	}
	if !released {
		t.Fatal("no claim.released event from the loser on the ledger")
	}
}

// TestClaimResponsesCarryConfirmBeforeMergeWarning: call-time stand-down
// starts at the claim — every claim response carries the standing
// confirm-before-merge rule (agent protocol step 5), and plain
// hydration does not.
func TestClaimResponsesCarryConfirmBeforeMergeWarning(t *testing.T) {
	d, _ := startDaemon(t)
	cs := mcpConnect(t, d, "brandon/impl-1", nil)

	var created createTasksResult
	mustToolOK(t, cs, "create_task", map[string]any{
		"tasks": []map[string]any{{"title": "first"}, {"title": "second"}},
	}, &created)

	var byTask hydratedTask
	mustToolOK(t, cs, "claim_task", map[string]any{"task": created.IDs[0]}, &byTask)
	for _, want := range []string{"confirm_claim", "protocol violation"} {
		if !strings.Contains(byTask.Warning, want) {
			t.Fatalf("claim_task warning %q missing %q", byTask.Warning, want)
		}
	}

	var byNext claimNextResult
	mustToolOK(t, cs, "claim_next", map[string]any{}, &byNext)
	if !byNext.Claimed || byNext.Task == nil || !strings.Contains(byNext.Task.Warning, "confirm_claim") {
		t.Fatalf("claim_next = %+v, want the confirm-before-merge warning on the claimed task", byNext)
	}

	var hydrated hydratedTask
	mustToolOK(t, cs, "get_task", map[string]any{"task": created.IDs[0]}, &hydrated)
	if hydrated.Warning != "" {
		t.Fatalf("get_task warning = %q, want none on plain hydration", hydrated.Warning)
	}
}

// TestCallTimeStandDownNotices: any tool a provisionally-voided
// claimant touches on its task states the loss plainly — add_note and
// escalate carry the notice, the winner's tools stay clean, and a
// finish through the MCP surface delivers the coerced record with the
// referee's message.
func TestCallTimeStandDownNotices(t *testing.T) {
	d, c := startDaemon(t)
	const winner, loser = "brandon/win", "brandon/lose"
	task := createOne(t, c, "brandon", map[string]any{"title": "raced work"})

	if _, oe := d.opClaimTask(winner, task); oe != nil {
		t.Fatalf("winner claim: %v", oe)
	}
	mintVoidedClaim(t, d, task, loser, time.Hour)

	_, warning, oe := d.opAddNote(loser, task, "midway checkpoint")
	if oe != nil {
		t.Fatalf("loser add_note: %v", oe)
	}
	for _, want := range []string{"lost its race", "Stand down", "finish_run"} {
		if !strings.Contains(warning, want) {
			t.Fatalf("loser add_note warning %q missing %q", warning, want)
		}
	}
	_, warning, oe = d.opEscalate(loser, escalateReq{Task: task, Question: "does the race verdict stand?"})
	if oe != nil {
		t.Fatalf("loser escalate: %v", oe)
	}
	if !strings.Contains(warning, "lost its race") {
		t.Fatalf("loser escalate warning = %q, want the stand-down notice", warning)
	}
	if _, warning, oe = d.opAddNote(winner, task, "winner checkpoint"); oe != nil || warning != "" {
		t.Fatalf("winner add_note = warning %q, %v — want clean", warning, oe)
	}

	// The MCP surface delivers the loser's coerced finish: outcome
	// superseded, links kept, the statement in the result.
	cs := mcpConnect(t, d, loser, nil)
	var res finishRunResult
	mustToolOK(t, cs, "finish_run", map[string]any{
		"task": task, "outcome": "done", "branch": "tuh-x/lost-work",
		"summary": "lost the race at the end",
	}, &res)
	if res.Outcome != event.OutcomeSuperseded || !strings.Contains(res.Message, "Recorded as superseded, not done") {
		t.Fatalf("MCP loser finish = %+v, want coerced superseded with the referee's message", res)
	}
}

// TestTwoDaemonLoserStory is D6 clause 3 end to end: two daemons on one
// remote race a claim; after they converge, the loser's finish_run(done)
// is coerced to superseded with its branch kept, and the winner's
// finish_run(done) — never having called confirm_claim — wins the gate
// at the remote and records done. Exactly one confirmation, one done,
// one superseded, on the shared branch itself.
func TestTwoDaemonLoserStory(t *testing.T) {
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
	rootA, rootB := clone("alpha"), clone("bravo")

	dA, _ := startDaemonAt(t, rootA, gateOpts())
	mustCycle(t, dA)
	runGit(t, rootB, "fetch", "--quiet", "origin", "refs/heads/tuhdoo:refs/heads/tuhdoo")
	dB, _ := startDaemonAt(t, rootB, gateOpts())

	const actorA, actorB = "harness/alpha", "harness/bravo"
	ids, _, oe := dA.opCreateTasks("seeder", []createTaskItem{{
		Title:       "collision bait",
		Description: "claimed on both machines; only one finish records done",
	}})
	if oe != nil {
		t.Fatalf("seed: %v", oe)
	}
	task := ids[0]
	if err := dA.batcher.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	mustCycle(t, dA)
	mustCycle(t, dB)

	// Both claim behind one starting gun — a genuine D6 race.
	sides := []struct {
		d     *Daemon
		actor string
	}{{dA, actorA}, {dB, actorB}}
	var wg sync.WaitGroup
	gun := make(chan struct{})
	claimErrs := make([]error, 2)
	for i, side := range sides {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-gun
			if _, oe := side.d.opClaimTask(side.actor, task); oe != nil {
				claimErrs[i] = oe
			}
		}()
	}
	close(gun)
	wg.Wait()
	for i, err := range claimErrs {
		if err != nil {
			t.Fatalf("claim %d: %v", i, err)
		}
	}
	convergeTrees(t, rootA, rootB, dA, dB)

	// Both daemons now agree who provisionally holds; the other side is
	// the loser.
	if err := dA.Refresh(); err != nil {
		t.Fatal(err)
	}
	dA.mu.Lock()
	holder := dA.state.ActiveClaim(task)
	dA.mu.Unlock()
	if holder == nil {
		t.Fatal("no provisional holder after convergence")
	}
	win, lose := sides[0], sides[1]
	if holder.Actor == actorB {
		win, lose = sides[1], sides[0]
	}

	// The loser reports done — with the branch only it knows — and is
	// recorded superseded.
	lres, oe := lose.d.opFinishRun(lose.actor, finishRunReq{
		Task: task, Outcome: event.OutcomeDone,
		Branch: "tuh-x/lost-work", Summary: "full attempt, lost the race",
	})
	if oe != nil {
		t.Fatalf("loser finish: %v", oe)
	}
	if lres.Outcome != event.OutcomeSuperseded || !strings.Contains(lres.Message, "Recorded as superseded, not done") {
		t.Fatalf("loser record = %+v, want coerced superseded with the stand-down statement", lres)
	}

	// The winner reports done without ever calling confirm_claim:
	// finish_run runs the same gate and wins it at the remote. A
	// transient 503 under contention is an honest retryable answer.
	var wres finishRunResult
	for tries := 0; ; tries++ {
		var woe *opError
		wres, woe = win.d.opFinishRun(win.actor, finishRunReq{
			Task: task, Outcome: event.OutcomeDone, Summary: "merged for real",
		})
		if woe == nil {
			break
		}
		if woe.code != http.StatusServiceUnavailable || tries >= 5 {
			t.Fatalf("winner finish: %v", woe)
		}
		time.Sleep(100 * time.Millisecond)
	}
	if wres.Outcome != event.OutcomeDone {
		t.Fatalf("winner record = %+v, want done", wres)
	}

	// Settle both clones on one tree and verify the story on the branch
	// itself: one confirmation (minted by the winner's finish), one done,
	// one superseded with the loser's branch.
	if err := lose.d.batcher.Flush(); err != nil {
		t.Fatal(err)
	}
	convergeTrees(t, rootA, rootB, dA, dB)
	events, err := dA.store.LoadEvents()
	if err != nil {
		t.Fatal(err)
	}
	confs := confirmedEvents(t, events)[task]
	if len(confs) != 1 {
		t.Fatalf("confirmations on the branch = %+v, want exactly one", confs)
	}
	var done, superseded int
	for _, r := range runEvents(t, events)[task] {
		switch r.Outcome {
		case event.OutcomeDone:
			done++
		case event.OutcomeSuperseded:
			superseded++
			if r.Branch != "tuh-x/lost-work" {
				t.Fatalf("superseded run lost its salvage branch: %+v", r)
			}
		default:
			t.Fatalf("unexpected run outcome %q", r.Outcome)
		}
	}
	if done != 1 || superseded != 1 {
		t.Fatalf("runs on the branch: %d done, %d superseded — want exactly one of each", done, superseded)
	}
}

// TestRenewOnceKeepsVoidedClaimsTracked: the renewal tick must not
// evict a provisionally-voided claim from session tracking — a race
// loser has to hear "lost" from its own confirm_claim, which gates on
// that tracking (D6 clause 3; escalation-decided 2026-08-04, the
// collision harness's settle phase is the field shape). The voided
// claim stays tracked but is never renewed, so its lease still lapses
// on schedule and expiry synthesis closes a loser that never reports.
// Closed and unknown claims are still dropped.
func TestRenewOnceKeepsVoidedClaimsTracked(t *testing.T) {
	d, c := startDaemon(t)
	const winner, loser = "brandon/win", "brandon/lose"
	raced := createOne(t, c, "brandon", map[string]any{"title": "raced work"})
	mine := createOne(t, c, "brandon", map[string]any{"title": "solo work"})

	if _, oe := d.opClaimTask(winner, raced); oe != nil {
		t.Fatalf("winner claim: %v", oe)
	}
	voided := mintVoidedClaim(t, d, raced, loser, time.Hour)
	h, oe := d.opClaimTask(loser, mine)
	if oe != nil {
		t.Fatalf("loser's solo claim: %v", oe)
	}
	active := h.Claim.ID
	// Rewind the active lease below a renewal's now+TTL so the renewal
	// is visible as a strictly later expiry (leases store second
	// precision, and claim and renewal land in the same instant here).
	if err := d.store.WriteLease(active, time.Now().Add(time.Minute)); err != nil {
		t.Fatalf("rewind active lease: %v", err)
	}

	s := &mcpSession{actor: loser, stop: make(chan struct{}), claims: make(map[string]string)}
	s.track(raced, voided)
	s.track(mine, active)
	s.track("tuh-gone", "01JUNKCLAIMID0000000000000")

	before, err := d.store.ReadLeases()
	if err != nil {
		t.Fatalf("ReadLeases before: %v", err)
	}
	d.renewOnce(s)
	after, err := d.store.ReadLeases()
	if err != nil {
		t.Fatalf("ReadLeases after: %v", err)
	}

	if _, held := s.heldClaim(raced); !held {
		t.Fatal("voided claim evicted from tracking — confirm_claim would answer \"holds no claim\" instead of \"lost\"")
	}
	if _, held := s.heldClaim(mine); !held {
		t.Fatal("active claim evicted from tracking")
	}
	if _, held := s.heldClaim("tuh-gone"); held {
		t.Fatal("unknown claim survived the renewal tick")
	}
	if !after[voided].Equal(before[voided]) {
		t.Fatalf("voided lease renewed (%s -> %s) — expiry synthesis would never close a silent loser", before[voided], after[voided])
	}
	if !after[active].After(before[active]) {
		t.Fatalf("active lease not renewed (%s -> %s)", before[active], after[active])
	}
}
