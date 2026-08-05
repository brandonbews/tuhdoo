package syncer

// Tests for the clone-join path (AdoptRemoteBranch) and the
// simultaneous-init race it deliberately does not try to prevent: two
// machines minting roots before either pushes must converge through the
// app-level union merge with no manual repair.

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/brandonbews/tuhdoo/internal/event"
	"github.com/brandonbews/tuhdoo/internal/gitx"
	"github.com/brandonbews/tuhdoo/internal/store"
)

// gitOut runs a raw git command and returns its output — runGit above
// discards it, and root-counting needs the rev-list text.
func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// roots lists the parentless commits reachable from the data branch —
// one entry per orphan root in its history.
func roots(t *testing.T, dir string) []string {
	t.Helper()
	out := strings.TrimSpace(gitOut(t, dir, "rev-list", "--max-parents=0", store.DefaultRef))
	if out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

// gitEnv pins identity and isolates the user's config, matching
// newPair's setup for tests that build their own repos.
func gitEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	t.Setenv("GIT_AUTHOR_NAME", "test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.invalid")
	t.Setenv("GIT_COMMITTER_NAME", "test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.invalid")
}

// mkRepo builds a working repo wired to remote (empty means no remote)
// and returns its dir and gitx handle. No data branch is created.
func mkRepo(t *testing.T, name, remote string) (string, gitx.Git) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "init", "-b", "main")
	if remote != "" {
		runGit(t, dir, "remote", "add", "origin", remote)
	}
	g, err := gitx.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	return dir, g
}

func ident(name string) gitx.Identity {
	return gitx.Identity{Name: name, Email: name + "@test.invalid"}
}

// TestAdoptRemoteBranchJoinsExistingHistory is the clone-join path end
// to end: a remote already carries the data branch; a fresh repo with
// no local ref adopts it, and the subsequent Init is a no-op — one
// root, no second orphan commit.
func TestAdoptRemoteBranchJoinsExistingHistory(t *testing.T) {
	gitEnv(t)
	bare := filepath.Join(t.TempDir(), "remote.git")
	runGit(t, t.TempDir(), "init", "--bare", "-b", "main", bare)

	// Seed the remote: one machine inits, writes an event, pushes.
	aDir, ga := mkRepo(t, "seeder", bare)
	sta := store.New(ga, "", ident("seeder"))
	if err := sta.Init(); err != nil {
		t.Fatal(err)
	}
	if err := sta.AppendBatch(store.Batch{Events: []event.Event{
		evt(t, 1, event.TypeTaskCreated, "brandon", "m-a", "t1", event.TaskCreated{Title: "seeded work"}),
	}}); err != nil {
		t.Fatal(err)
	}
	sya := New(ga, Options{Ident: ident("seeder")})
	if err := sya.Cycle(); err != nil {
		t.Fatalf("seed cycle: %v", err)
	}

	// The joiner: fresh repo, remote configured, no local data branch.
	bDir, gb := mkRepo(t, "joiner", bare)
	syb := New(gb, Options{Ident: ident("joiner")})
	syb.AdoptRemoteBranch()

	remoteHead := strings.TrimSpace(gitOut(t, bare, "rev-parse", store.DefaultRef))
	localHead, err := gb.ReadRef(store.DefaultRef)
	if err != nil {
		t.Fatalf("adoption left no local data branch: %v", err)
	}
	if localHead != remoteHead {
		t.Fatalf("adopted head = %s, want remote head %s", localHead, remoteHead)
	}

	// Init after adoption must be the no-op arm, not a second root.
	stb := store.New(gb, "", ident("joiner"))
	if err := stb.Init(); err != nil {
		t.Fatal(err)
	}
	if got := roots(t, bDir); len(got) != 1 {
		t.Fatalf("joiner history carries %d roots %v, want exactly one", len(got), got)
	}
	if got := roots(t, aDir); len(got) != 1 {
		t.Fatalf("seeder history carries %d roots %v, want exactly one", len(got), got)
	}

	// The adopted branch replays: the seeded event is already visible.
	evs, err := stb.LoadEvents()
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 || evs[0].Task != "t1" {
		t.Fatalf("adopted events = %+v, want the one seeded event", evs)
	}
}

// TestAdoptRemoteBranchFallsBackToMinting covers every arm where
// adoption must quietly stand aside and let Init mint: no remote at
// all (T2 remoteless, a normal state), a remote without the branch,
// and an unreachable remote. None may error; all end in one fresh root.
func TestAdoptRemoteBranchFallsBackToMinting(t *testing.T) {
	gitEnv(t)

	cases := []struct {
		name   string
		remote func(t *testing.T) string
	}{
		{"remoteless", func(t *testing.T) string { return "" }},
		{"remote lacks the branch", func(t *testing.T) string {
			bare := filepath.Join(t.TempDir(), "empty.git")
			runGit(t, t.TempDir(), "init", "--bare", "-b", "main", bare)
			return bare
		}},
		{"unreachable remote", func(t *testing.T) string {
			return filepath.Join(t.TempDir(), "no-such-remote.git")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir, g := mkRepo(t, "solo", tc.remote(t))
			sy := New(g, Options{Ident: ident("solo")})
			sy.AdoptRemoteBranch()

			// Nothing adopted; Init mints exactly as without adoption.
			if _, err := g.ReadRef(store.DefaultRef); !errors.Is(err, gitx.ErrRefNotFound) {
				t.Fatalf("ref after failed adoption: err = %v, want ErrRefNotFound", err)
			}
			st := store.New(g, "", ident("solo"))
			if err := st.Init(); err != nil {
				t.Fatalf("init after fallback: %v", err)
			}
			if got := roots(t, dir); len(got) != 1 {
				t.Fatalf("history carries %d roots %v, want one fresh root", len(got), got)
			}

			// Re-running adoption against an existing branch is a no-op.
			head, err := g.ReadRef(store.DefaultRef)
			if err != nil {
				t.Fatal(err)
			}
			sy.AdoptRemoteBranch()
			if again, _ := g.ReadRef(store.DefaultRef); again != head {
				t.Fatalf("adoption moved an existing branch %s -> %s", head, again)
			}
		})
	}
}

// TestSimultaneousInitRaceConverges proves the race adoption cannot
// prevent: two machines each mint their own orphan root against one
// shared remote before either pushes. Both then sync; the union merge
// across histories with no common ancestor must converge them —
// byte-identical trees (events and views alike), identical replayed
// state, both roots preserved, no manual repair.
func TestSimultaneousInitRaceConverges(t *testing.T) {
	a, b := newPair(t) // each peer ran Init before any push: two roots

	rootA, rootB := roots(t, a.dir), roots(t, b.dir)
	if len(rootA) != 1 || len(rootB) != 1 || rootA[0] == rootB[0] {
		t.Fatalf("setup expects two distinct single-root histories, got %v / %v", rootA, rootB)
	}

	// Each side writes before hearing of the other, so the merge unions
	// real content, not just empty roots.
	if err := a.store.AppendBatch(store.Batch{Events: []event.Event{
		evt(t, 1, event.TypeTaskCreated, "brandon", "m-a", "t1", event.TaskCreated{Title: "born on A"}),
	}}); err != nil {
		t.Fatal(err)
	}
	if err := b.store.AppendBatch(store.Batch{Events: []event.Event{
		evt(t, 2, event.TypeTaskCreated, "sarah", "m-b", "t2", event.TaskCreated{Title: "born on B"}),
	}}); err != nil {
		t.Fatal(err)
	}

	cycle(t, a) // A publishes its root
	cycle(t, b) // B fetches A: no common ancestor — union merge, push
	cycle(t, a) // A fast-forwards to the merge
	sameTrees(t, a, b)
	if b.sync.Status().Merges == 0 {
		t.Fatal("expected B to build the cross-root merge")
	}

	// Both sides replay to the identical state, and both see both tasks.
	sa, sb := stateOf(t, a), stateOf(t, b)
	if !reflect.DeepEqual(sa, sb) {
		t.Fatalf("replayed states differ:\nA: %+v\nB: %+v", sa, sb)
	}
	if sa.Tasks["t1"] == nil || sa.Tasks["t2"] == nil {
		t.Fatalf("merged state lost a task: %v", sa.TaskOrder)
	}

	// Both orphan roots survive in both histories — the merge joined
	// them, it never rewrote anyone's bytes.
	want := map[string]bool{rootA[0]: true, rootB[0]: true}
	for _, p := range []peer{a, b} {
		got := roots(t, p.dir)
		if len(got) != 2 || !want[got[0]] || !want[got[1]] {
			t.Fatalf("history on %s carries roots %v, want both of %v", p.dir, got, want)
		}
	}
}
