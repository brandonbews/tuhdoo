package daemon

// End-to-end test of the clone-join path: a real `git clone` of a
// remote already carrying the data branch starts its daemon and adopts
// that branch instead of minting a second orphan root.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brandonbews/tuhdoo/internal/event"
)

func TestFreshCloneDaemonAdoptsRemoteDataBranch(t *testing.T) {
	setGitEnv(t)
	base := shortTempDir(t)
	bare := filepath.Join(base, "origin.git")
	runGit(t, base, "init", "--quiet", "--bare", "-b", "main", bare)

	// First machine: daemon mints the branch, seeds a task, publishes.
	rootA := filepath.Join(base, "first")
	if err := os.MkdirAll(rootA, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, rootA, "init", "--quiet", "-b", "main")
	runGit(t, rootA, "remote", "add", "origin", bare)
	dA, cA := startDaemonAt(t, rootA, gateOpts())
	task := createOne(t, cA, "brandon", map[string]any{"title": "seeded before the clone"})
	if err := dA.batcher.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	mustCycle(t, dA)
	remoteHead := strings.TrimSpace(runGit(t, bare, "rev-parse", "refs/heads/tuhdoo"))

	// Second machine: a genuine clone — origin configured, no local
	// data branch (git clone creates no local refs/heads/tuhdoo). Its
	// daemon's startup must adopt the remote branch, not mint.
	rootB := filepath.Join(base, "second")
	runGit(t, base, "clone", "--quiet", bare, rootB)
	dB, _ := startDaemonAt(t, rootB, gateOpts())

	localHead := strings.TrimSpace(runGit(t, rootB, "rev-parse", "--verify", "refs/heads/tuhdoo"))
	if localHead != remoteHead {
		t.Fatalf("clone's data branch = %s, want the remote head %s", localHead, remoteHead)
	}
	roots := strings.Fields(runGit(t, rootB, "rev-list", "--max-parents=0", "refs/heads/tuhdoo"))
	if len(roots) != 1 {
		t.Fatalf("clone's history carries %d roots %v, want exactly one — a second root means adoption minted", len(roots), roots)
	}

	// The adopted history is live state, not just a ref: the seeded
	// task is visible without a single sync cycle on B.
	evs, err := dB.store.LoadEvents()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range evs {
		if e.Type == event.TypeTaskCreated && e.Task == task {
			found = true
		}
	}
	if !found {
		t.Fatalf("adopted branch does not carry the seeded task %s", task)
	}
}

// cloneShapeAdopt is the shared body of the clone-shape tests: seed a
// bare origin through one daemon (with a code commit on main, so every
// clone shape has a HEAD branch to check out), clone it with the given
// arguments, let checkShape pin that the clone really has the claimed
// shape, then start the clone's daemon and assert adoption exactly as
// the plain-clone test above does: the data branch at the remote head,
// exactly one root, and the seeded task visible without a sync cycle.
//
// url maps the bare repo's path to the clone URL — file:// for shapes
// (like --depth) that git quietly ignores on plain local paths.
func cloneShapeAdopt(t *testing.T, url func(bare string) string, cloneArgs []string, checkShape func(t *testing.T, dir string)) {
	t.Helper()
	setGitEnv(t)
	base := shortTempDir(t)
	bare := filepath.Join(base, "origin.git")
	runGit(t, base, "init", "--quiet", "--bare", "-b", "main", bare)

	// First machine: publish main (the project's code — real joins clone
	// a repo that has some), then let the daemon mint the data branch,
	// seed a task, and publish it.
	rootA := filepath.Join(base, "first")
	if err := os.MkdirAll(rootA, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, rootA, "init", "--quiet", "-b", "main")
	runGit(t, rootA, "remote", "add", "origin", bare)
	runGit(t, rootA, "commit", "--quiet", "--allow-empty", "-m", "project code")
	runGit(t, rootA, "push", "--quiet", "origin", "main")
	dA, cA := startDaemonAt(t, rootA, gateOpts())
	task := createOne(t, cA, "brandon", map[string]any{"title": "seeded before the clone"})
	if err := dA.batcher.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	mustCycle(t, dA)
	remoteHead := strings.TrimSpace(runGit(t, bare, "rev-parse", "refs/heads/tuhdoo"))

	// Second machine: the clone under test. Whatever the shape leaves
	// out, adoption fetches refs/heads/tuhdoo by explicit refspec — no
	// remote-tracking config is consulted — so its daemon must still
	// adopt the remote branch, not mint.
	rootB := filepath.Join(base, "second")
	args := append([]string{"clone", "--quiet"}, cloneArgs...)
	runGit(t, base, append(args, url(bare), rootB)...)
	checkShape(t, rootB)
	dB, _ := startDaemonAt(t, rootB, gateOpts())

	localHead := strings.TrimSpace(runGit(t, rootB, "rev-parse", "--verify", "refs/heads/tuhdoo"))
	if localHead != remoteHead {
		t.Fatalf("clone's data branch = %s, want the remote head %s", localHead, remoteHead)
	}
	roots := strings.Fields(runGit(t, rootB, "rev-list", "--max-parents=0", "refs/heads/tuhdoo"))
	if len(roots) != 1 {
		t.Fatalf("clone's history carries %d roots %v, want exactly one — a second root means adoption minted", len(roots), roots)
	}
	evs, err := dB.store.LoadEvents()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range evs {
		if e.Type == event.TypeTaskCreated && e.Task == task {
			found = true
		}
	}
	if !found {
		t.Fatalf("adopted branch does not carry the seeded task %s", task)
	}
}

// TestSingleBranchCloneDaemonAdoptsRemoteDataBranch: a --single-branch
// clone carries no trace of the data branch — not even a remote-tracking
// ref — and its fetch refspec covers main alone. Adoption must still
// join the existing history (docs/joining.md relies on this).
func TestSingleBranchCloneDaemonAdoptsRemoteDataBranch(t *testing.T) {
	cloneShapeAdopt(t,
		func(bare string) string { return bare },
		[]string{"--single-branch"},
		func(t *testing.T, dir string) {
			t.Helper()
			if out := runGit(t, dir, "branch", "-r"); strings.Contains(out, "tuhdoo") {
				t.Fatalf("clone is not single-branch — remote-tracking refs:\n%s", out)
			}
		})
}

// TestShallowCloneDaemonAdoptsRemoteDataBranch: a --depth=1 clone (which
// implies --single-branch) has truncated history and no data branch at
// all; adoption fetches the data branch fresh from origin, and replay
// reads only its tip tree — so shallowness never bites (docs/joining.md
// relies on this). file:// forces a genuinely shallow clone: git
// ignores --depth on plain local paths.
func TestShallowCloneDaemonAdoptsRemoteDataBranch(t *testing.T) {
	cloneShapeAdopt(t,
		func(bare string) string { return "file://" + bare },
		[]string{"--depth=1"},
		func(t *testing.T, dir string) {
			t.Helper()
			if got := strings.TrimSpace(runGit(t, dir, "rev-parse", "--is-shallow-repository")); got != "true" {
				t.Fatalf("clone is not shallow (is-shallow-repository = %q)", got)
			}
		})
}
