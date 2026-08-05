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
