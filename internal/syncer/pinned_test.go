package syncer

// Tests for a pinned sync target (Options.Pinned — the daemon pins the
// target a .tuhdoo file or tuhdoo.remote names). The syncer fetches
// and pushes the target exactly as given, a path or URL no remote
// config names, and never reads it as remoteless: nothing falls back
// to origin, and nothing turns into local-only mode. Every repo here
// has a reachable bare origin, so "origin never receives the data
// branch" is a real check, not a vacuous one.

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brandonbews/tuhdoo/internal/event"
	"github.com/brandonbews/tuhdoo/internal/gitx"
	"github.com/brandonbews/tuhdoo/internal/store"
)

// newBare makes an empty bare repository and returns its path.
func newBare(t *testing.T, name string) string {
	t.Helper()
	bare := filepath.Join(t.TempDir(), name)
	runGit(t, t.TempDir(), "init", "--quiet", "--bare", "-b", "main", bare)
	return bare
}

// hasDataBranch reports whether the repository at dir carries the data
// branch. Not runGit: a missing ref is an answer here, not a failure.
func hasDataBranch(t *testing.T, dir string) bool {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "--verify", "--quiet", store.DefaultRef)
	cmd.Dir = dir
	return cmd.Run() == nil
}

// pinnedPeer builds a repo whose origin is the reachable bare repo
// origin, with no local data branch yet, and a syncer pinned to target.
func pinnedPeer(t *testing.T, origin, target string) peer {
	t.Helper()
	dir, g := mkRepo(t, "pinned", origin)
	st := store.New(g, "", ident("pinned"))
	sy := New(g, st, Options{Remote: target, Pinned: true, Source: ".tuhdoo", Ident: ident("pinned")})
	return peer{dir: dir, git: g, store: st, sync: sy}
}

// seedBare publishes a data branch holding one task to bare, through a
// throwaway unpinned peer, and returns the published head.
func seedBare(t *testing.T, bare, name, title string) string {
	t.Helper()
	_, g := mkRepo(t, name, bare)
	st := store.New(g, "", ident(name))
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	if err := st.AppendBatch(store.Batch{Events: []event.Event{
		evt(t, 1, event.TypeTaskCreated, "brandon", "m-"+name, "t-"+name, event.TaskCreated{Title: title}),
	}}); err != nil {
		t.Fatal(err)
	}
	if err := New(g, st, Options{Ident: ident(name)}).Cycle(); err != nil {
		t.Fatalf("seed %s: %v", name, err)
	}
	return strings.TrimSpace(runGit(t, bare, "rev-parse", store.DefaultRef))
}

// TestPinnedTargetSyncsThereNotOrigin: with origin reachable and the
// target a second bare repo, the clone-join, the cycle's fetch and
// push, and status all name the target; origin never receives the
// data branch.
func TestPinnedTargetSyncsThereNotOrigin(t *testing.T) {
	gitEnv(t)
	origin, ledger := newBare(t, "origin.git"), newBare(t, "ledger.git")
	p := pinnedPeer(t, origin, ledger)

	p.sync.AdoptRemoteBranch() // the target has no branch yet: mint
	if err := p.store.Init(); err != nil {
		t.Fatal(err)
	}
	if err := p.store.AppendBatch(store.Batch{Events: []event.Event{
		evt(t, 1, event.TypeTaskCreated, "brandon", "m-p", "t1", event.TaskCreated{Title: "private plans"}),
	}}); err != nil {
		t.Fatal(err)
	}
	cycle(t, p)

	if st := p.sync.Status(); st.Mode != "syncing" || st.Remote != ledger || st.Source != ".tuhdoo" {
		t.Fatalf("status = %+v, want syncing with %s from .tuhdoo", st, ledger)
	}
	local := strings.TrimSpace(runGit(t, p.dir, "rev-parse", store.DefaultRef))
	if got := strings.TrimSpace(runGit(t, ledger, "rev-parse", store.DefaultRef)); got != local {
		t.Fatalf("target head = %s, want the local head %s", got, local)
	}
	if hasDataBranch(t, origin) {
		t.Fatal("origin received the data branch from a pinned syncer")
	}
}

// TestPinnedAdoptJoinsTargetNotOrigin: origin and the target each carry
// a data branch of their own. A pinned clone with no local branch
// adopts the target's — and, with the target unreachable, mints a
// fresh root rather than adopting origin's.
func TestPinnedAdoptJoinsTargetNotOrigin(t *testing.T) {
	gitEnv(t)
	origin, ledger := newBare(t, "origin.git"), newBare(t, "ledger.git")
	originHead := seedBare(t, origin, "public", "stale public ledger")
	ledgerHead := seedBare(t, ledger, "private", "the real ledger")

	p := pinnedPeer(t, origin, ledger)
	p.sync.AdoptRemoteBranch()
	if got, err := p.git.ReadRef(store.DefaultRef); err != nil || got != ledgerHead {
		t.Fatalf("adopted %q (%v), want the target's head %s", got, err, ledgerHead)
	}

	gone := pinnedPeer(t, origin, filepath.Join(t.TempDir(), "no-such-ledger.git"))
	gone.sync.AdoptRemoteBranch()
	if got, err := gone.git.ReadRef(store.DefaultRef); err == nil && got == originHead {
		t.Fatal("an unreachable pinned target adopted origin's data branch")
	}
}

// TestTargetSwitchWithTrackingRefAhead replays the 2026-10-12 ledger
// cutover. A clone syncing with origin copies its ledger to a private
// repo (docs/adopting.md, "Keeping the ledger in a private repo"), its
// old daemon pushes to origin once more, and the restarted daemon is
// pinned to the private repo. TrackingRef still holds origin's newer
// head, and the private repo's branch is an ancestor of it. The fetch
// force-updates the tracking ref, so the cycles succeed: the local
// branch and the new target converge, and origin is never touched. The
// backwards move is logged as a warning exactly once.
func TestTargetSwitchWithTrackingRefAhead(t *testing.T) {
	gitEnv(t)
	origin, ledger := newBare(t, "origin.git"), newBare(t, "ledger.git")
	dir, g := mkRepo(t, "switcher", origin)
	write := func(st *store.Store, n int) {
		t.Helper()
		if err := st.AppendBatch(store.Batch{Events: []event.Event{
			evt(t, n, event.TypeTaskCreated, "brandon", "m-switcher", fmt.Sprintf("t%d", n), event.TaskCreated{Title: "work"}),
		}}); err != nil {
			t.Fatal(err)
		}
	}

	// Before the switch: the old daemon syncs with origin.
	oldStore := store.New(g, "", ident("switcher"))
	if err := oldStore.Init(); err != nil {
		t.Fatal(err)
	}
	oldSync := New(g, oldStore, Options{Ident: ident("switcher")})
	write(oldStore, 1)
	if err := oldSync.Cycle(); err != nil {
		t.Fatal(err)
	}
	// The copy step: push the ledger to the private repo.
	runGit(t, dir, "push", "--quiet", ledger, store.DefaultRef+":"+store.DefaultRef)
	// The old daemon pushes to origin again before its restart; its
	// next cycle fetches that head back into the tracking ref.
	write(oldStore, 2)
	for i := 0; i < 2; i++ {
		if err := oldSync.Cycle(); err != nil {
			t.Fatal(err)
		}
	}

	originRefs := runGit(t, origin, "for-each-ref")
	originHead := strings.TrimSpace(runGit(t, origin, "rev-parse", store.DefaultRef))
	ledgerHead := strings.TrimSpace(runGit(t, ledger, "rev-parse", store.DefaultRef))
	if tracking, err := g.ReadRef(TrackingRef); err != nil || tracking != originHead {
		t.Fatalf("setup: tracking ref = %q (%v), want origin's head %s", tracking, err, originHead)
	}
	if behind, err := g.IsAncestor(ledgerHead, originHead); err != nil || !behind || ledgerHead == originHead {
		t.Fatalf("setup: the ledger (%s) must be strictly behind origin (%s)", ledgerHead, originHead)
	}

	// The restart: a fresh store and a syncer pinned to the private
	// repo, plus one more local write so the push it owes is visible.
	st := store.New(g, "", ident("switcher"))
	if err := st.Load(); err != nil {
		t.Fatal(err)
	}
	write(st, 3)
	var logged bytes.Buffer
	sy := New(g, st, Options{Remote: ledger, Pinned: true, Source: ".tuhdoo", Ident: ident("switcher"), Log: log.New(&logged, "", 0)})
	// The first cycle pushes; the second fetches the pushed head back.
	for i := 1; i <= 2; i++ {
		if err := sy.Cycle(); err != nil {
			t.Fatalf("cycle %d after the switch: %v", i, err)
		}
	}

	local := strings.TrimSpace(runGit(t, dir, "rev-parse", store.DefaultRef))
	if got := strings.TrimSpace(runGit(t, ledger, "rev-parse", store.DefaultRef)); got != local {
		t.Fatalf("ledger head = %s, want the local head %s", got, local)
	}
	if tracking, err := g.ReadRef(TrackingRef); err != nil || tracking != local {
		t.Fatalf("tracking ref = %q (%v), want the converged head %s", tracking, err, local)
	}
	if st := sy.Status(); st.Mode != "syncing" || st.LastError != "" {
		t.Fatalf("status = %+v, want syncing with no error", st)
	}
	if got := runGit(t, origin, "for-each-ref"); got != originRefs {
		t.Fatalf("origin's refs changed after the switch:\nbefore:\n%s\nafter:\n%s", originRefs, got)
	}
	if n := strings.Count(logged.String(), rewindWarning); n != 1 {
		t.Fatalf("logged %d rewind warnings, want exactly 1:\n%s", n, logged.String())
	}
}

// TestTargetSwitchAdoptWithTrackingRefAhead: adoption fetches with the
// same forced refspec as the cycle. A clone with no local data branch,
// but with a tracking ref left ahead of the target's branch, adopts the
// target's head. An unforced fetch would be refused, and the clone
// would mint a second root instead.
func TestTargetSwitchAdoptWithTrackingRefAhead(t *testing.T) {
	gitEnv(t)
	origin, ledger := newBare(t, "origin.git"), newBare(t, "ledger.git")
	ledgerHead := seedBare(t, ledger, "private", "the real ledger")

	p := pinnedPeer(t, origin, ledger)
	// Leave the tracking ref one commit ahead of the ledger's branch.
	runGit(t, p.dir, "fetch", "--quiet", ledger, store.DefaultRef+":"+TrackingRef)
	ahead := strings.TrimSpace(runGit(t, p.dir, "commit-tree", "-p", ledgerHead, "-m", "ahead", ledgerHead+"^{tree}"))
	runGit(t, p.dir, "update-ref", TrackingRef, ahead)

	p.sync.AdoptRemoteBranch()
	if got, err := p.git.ReadRef(store.DefaultRef); err != nil || got != ledgerHead {
		t.Fatalf("adopted %q (%v), want the target's head %s", got, err, ledgerHead)
	}
	if got, err := p.git.ReadRef(TrackingRef); err != nil || got != ledgerHead {
		t.Fatalf("tracking ref = %q (%v), want the target's head %s", got, err, ledgerHead)
	}
}

// TestPinnedTargetFailureIsAnErrorNeverLocalOnly: a pinned target that
// does not exist — a missing path, or a remote name nothing configures
// — is a sync error and a gate error, never the remoteless state
// (gitx.ErrNoRemote → local-only, or a local confirmation). Origin is
// reachable throughout and never receives the data branch.
func TestPinnedTargetFailureIsAnErrorNeverLocalOnly(t *testing.T) {
	gitEnv(t)
	for _, tc := range []struct {
		name   string
		target func(t *testing.T) string
	}{
		{"missing path", func(t *testing.T) string { return filepath.Join(t.TempDir(), "no-such-ledger.git") }},
		{"unconfigured remote name", func(*testing.T) string { return "ledger" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			origin := newBare(t, "origin.git")
			target := tc.target(t)
			p := pinnedPeer(t, origin, target)
			p.sync.AdoptRemoteBranch()
			if err := p.store.Init(); err != nil {
				t.Fatal(err)
			}
			if err := p.store.AppendBatch(store.Batch{Events: []event.Event{
				evt(t, 1, event.TypeTaskCreated, "brandon", "m-p", "t1", event.TaskCreated{Title: "stranded"}),
			}}); err != nil {
				t.Fatal(err)
			}

			err := p.sync.Cycle()
			if err == nil || errors.Is(err, gitx.ErrNoRemote) {
				t.Fatalf("Cycle = %v, want a fetch error (not ErrNoRemote)", err)
			}
			p.sync.cycleAndRecord()
			st := p.sync.Status()
			if st.Mode != "error" || st.LastError == "" || st.Remote != target {
				t.Fatalf("status = %+v, want an error naming %q", st, target)
			}

			if _, _, err := p.sync.GateHead(); err == nil || errors.Is(err, gitx.ErrNoRemote) {
				t.Fatalf("GateHead = %v, want a fetch error the referee refuses on (not ErrNoRemote)", err)
			}
			if hasDataBranch(t, origin) {
				t.Fatal("origin received the data branch from a pinned syncer")
			}
		})
	}
}
