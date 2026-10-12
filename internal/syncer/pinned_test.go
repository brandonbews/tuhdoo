package syncer

// Tests for a pinned sync target (Options.Pinned — the daemon pins the
// target a .tuhdoo file or tuhdoo.remote names). The syncer fetches
// and pushes the target exactly as given, a path or URL no remote
// config names, and never reads it as remoteless: nothing falls back
// to origin, and nothing turns into local-only mode. Every repo here
// has a reachable bare origin, so "origin never receives the data
// branch" is a real check, not a vacuous one.

import (
	"errors"
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
