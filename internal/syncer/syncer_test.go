package syncer

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brandonbews/tuhdoo/internal/core"
	"github.com/brandonbews/tuhdoo/internal/event"
	"github.com/brandonbews/tuhdoo/internal/gitx"
	"github.com/brandonbews/tuhdoo/internal/store"
	"github.com/brandonbews/tuhdoo/internal/views"
)

var testBase = time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)

func tick(t *testing.T, n int) string {
	t.Helper()
	entropy := make([]byte, 10)
	entropy[9] = byte(n)
	id, err := event.NewID(testBase.Add(time.Duration(n)*time.Minute), bytes.NewReader(entropy))
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func evt(t *testing.T, n int, typ, actor, machine, task string, payload any) event.Event {
	t.Helper()
	e, err := event.New(tick(t, n), typ, 1, actor, machine, task, payload)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

type peer struct {
	dir   string
	git   gitx.Git
	store *store.Store
	sync  *Syncer
}

// newPair returns two peers sharing one bare remote, each with an
// initialized data branch.
func newPair(t *testing.T) (peer, peer) {
	t.Helper()
	gitEnv(t)

	bare := filepath.Join(t.TempDir(), "remote.git")
	runGit(t, t.TempDir(), "init", "--bare", "-b", "main", bare)

	mk := func(name string) peer {
		dir := filepath.Join(t.TempDir(), name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		runGit(t, dir, "init", "-b", "main")
		runGit(t, dir, "remote", "add", "origin", bare)
		g, err := gitx.New(dir)
		if err != nil {
			t.Fatal(err)
		}
		st := store.New(g, "", gitx.Identity{Name: name, Email: name + "@test.invalid"})
		if err := st.Init(); err != nil {
			t.Fatal(err)
		}
		sy := New(g, Options{Ident: gitx.Identity{Name: name, Email: name + "@test.invalid"}})
		return peer{dir: dir, git: g, store: st, sync: sy}
	}
	return mk("machine-a"), mk("machine-b")
}

func cycle(t *testing.T, p peer) {
	t.Helper()
	if err := p.sync.Cycle(); err != nil {
		t.Fatalf("cycle: %v", err)
	}
}

func head(t *testing.T, p peer) map[string]string {
	t.Helper()
	m, err := treeMap(p.git, store.DefaultRef)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func stateOf(t *testing.T, p peer) *core.State {
	t.Helper()
	s, err := p.sync.replayTree(head(t, p))
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	return s
}

func sameTrees(t *testing.T, a, b peer) {
	t.Helper()
	ta, tb := head(t, a), head(t, b)
	if len(ta) != len(tb) {
		t.Fatalf("tree sizes differ: %d vs %d\nA: %v\nB: %v", len(ta), len(tb), ta, tb)
	}
	for path, oid := range ta {
		if tb[path] != oid {
			t.Fatalf("trees differ at %s: %s vs %s", path, oid, tb[path])
		}
	}
}

// mergeBothWays publishes B's head so A's object database holds both
// sides, merges the two heads directly in both orders, asserts the two
// directions produce the identical tree, and returns that tree — the
// order-independence half of every same-path merge-rule test.
func mergeBothWays(t *testing.T, a, b peer) map[string]string {
	t.Helper()
	cycle(t, b)
	remoteHead, err := a.sync.fetch()
	if err != nil {
		t.Fatal(err)
	}
	localHead, err := a.git.ReadRef(store.DefaultRef)
	if err != nil {
		t.Fatal(err)
	}
	m1, err := a.sync.merge(localHead, remoteHead)
	if err != nil {
		t.Fatal(err)
	}
	m2, err := a.sync.merge(remoteHead, localHead)
	if err != nil {
		t.Fatal(err)
	}
	t1, err := treeMap(a.git, m1)
	if err != nil {
		t.Fatal(err)
	}
	t2, err := treeMap(a.git, m2)
	if err != nil {
		t.Fatal(err)
	}
	if len(t1) != len(t2) {
		t.Fatalf("merge directions disagree: %d vs %d entries", len(t1), len(t2))
	}
	for p, oid := range t1 {
		if t2[p] != oid {
			t.Fatalf("merge directions disagree at %s: %s vs %s", p, oid, t2[p])
		}
	}
	return t1
}

func TestDivergentWritesConvergeAndClaimRaceResolves(t *testing.T) {
	a, b := newPair(t)
	alive := time.Now().Add(time.Hour)

	// A creates the task and claims it; pushes first.
	claimA, claimB := tick(t, 2), tick(t, 3)
	if err := a.store.AppendBatch(store.Batch{Events: []event.Event{
		evt(t, 1, event.TypeTaskCreated, "brandon", "m-a", "t1", event.TaskCreated{Title: "fix login"}),
		evt(t, 2, event.TypeClaimMade, "brandon/impl-1", "m-a", "t1", event.ClaimMade{}),
	}}); err != nil {
		t.Fatal(err)
	}
	if err := a.store.WriteLease(claimA, alive); err != nil {
		t.Fatal(err)
	}
	cycle(t, a)

	// B converges, then claims the same task (a later ULID: the loser).
	cycle(t, b)
	if err := b.store.AppendBatch(store.Batch{Events: []event.Event{
		evt(t, 3, event.TypeClaimMade, "sarah/impl-9", "m-b", "t1", event.ClaimMade{}),
	}}); err != nil {
		t.Fatal(err)
	}
	if err := b.store.WriteLease(claimB, alive); err != nil {
		t.Fatal(err)
	}
	cycle(t, b)

	// Meanwhile A writes more — true divergence — then both settle.
	if err := a.store.AppendBatch(store.Batch{Events: []event.Event{
		evt(t, 4, event.TypeNoteAdded, "brandon/impl-1", "m-a", "t1", event.NoteAdded{Text: "starting"}),
	}}); err != nil {
		t.Fatal(err)
	}
	cycle(t, a) // fetches B's claim, merges, pushes
	cycle(t, b) // fast-forwards to the merge
	sameTrees(t, a, b)

	// Both machines agree on the D6 verdict.
	for _, p := range []peer{a, b} {
		s := stateOf(t, p)
		if got := s.Claims[claimA].Status; got != core.ClaimActive {
			t.Fatalf("claim A status = %s, want active", got)
		}
		if got := s.Claims[claimB].Status; got != core.ClaimVoided {
			t.Fatalf("claim B status = %s, want voided", got)
		}
	}

	// The merge regenerated views from the merged events.
	ta := head(t, a)
	if _, ok := ta[views.MetaPath]; !ok {
		t.Fatal("merged tree has no view stamp")
	}
	backlog, err := a.git.CatFile(ta["backlog.md"])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(backlog), "fix login") {
		t.Fatal("regenerated backlog does not mention the task")
	}
	if a.sync.Status().Merges == 0 {
		t.Fatal("expected at least one app-level merge on A")
	}
}

func TestRemotelessIsNormalAndUnreachableRecovers(t *testing.T) {
	// Remoteless: a repo with no origin runs local-only, no error.
	dir := filepath.Join(t.TempDir(), "solo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	runGit(t, dir, "init", "-b", "main")
	g, err := gitx.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	st := store.New(g, "", gitx.Identity{Name: "solo", Email: "s@test.invalid"})
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	solo := New(g, Options{})
	if err := solo.Cycle(); err != nil {
		t.Fatalf("remoteless cycle must be a no-op, got %v", err)
	}
	if got := solo.Status().Mode; got != "local-only" {
		t.Fatalf("mode = %q, want local-only", got)
	}

	// Unreachable remote: degrade with an error status, recover cleanly.
	a, _ := newPair(t)
	cycle(t, a)
	url, err := a.git.RemoteURL("origin")
	if err != nil {
		t.Fatal(err)
	}
	moved := url + ".moved"
	if err := os.Rename(url, moved); err != nil {
		t.Fatal(err)
	}
	a.sync.cycleAndRecord()
	if st := a.sync.Status(); st.LastError == "" || st.Mode != "error" {
		t.Fatalf("expected error status while unreachable, got %+v", st)
	}
	if err := os.Rename(moved, url); err != nil {
		t.Fatal(err)
	}
	a.sync.cycleAndRecord()
	if st := a.sync.Status(); st.LastError != "" || st.Mode != "syncing" {
		t.Fatalf("expected recovery, got %+v", st)
	}
}

func TestLeaseRenewalSurvivesMerge(t *testing.T) {
	a, b := newPair(t)
	claim := tick(t, 2)
	early := time.Now().Add(10 * time.Minute)
	renewed := time.Now().Add(time.Hour)

	if err := a.store.AppendBatch(store.Batch{Events: []event.Event{
		evt(t, 1, event.TypeTaskCreated, "brandon", "m-a", "t1", event.TaskCreated{Title: "fix login"}),
		evt(t, 2, event.TypeClaimMade, "brandon/impl-1", "m-a", "t1", event.ClaimMade{}),
	}}); err != nil {
		t.Fatal(err)
	}
	if err := a.store.WriteLease(claim, early); err != nil {
		t.Fatal(err)
	}
	cycle(t, a)
	cycle(t, b) // B now carries a copy of the early lease

	// A renews locally; B diverges with a note and pushes first.
	if err := a.store.WriteLease(claim, renewed); err != nil {
		t.Fatal(err)
	}
	if err := b.store.AppendBatch(store.Batch{Events: []event.Event{
		evt(t, 3, event.TypeNoteAdded, "sarah", "m-b", "t1", event.NoteAdded{Text: "observing"}),
	}}); err != nil {
		t.Fatal(err)
	}
	cycle(t, b)
	cycle(t, a) // merge: renewed lease vs B's stale copy
	cycle(t, b)
	sameTrees(t, a, b)

	leases, err := a.store.ReadLeases()
	if err != nil {
		t.Fatal(err)
	}
	want := renewed.UTC().Truncate(time.Second)
	if got := leases[claim]; !got.Equal(want) {
		t.Fatalf("merged lease expiry = %v, want the renewal %v", got, want)
	}
}

// TestLeaseMergeTombstoneRules is the literal-tree table test for the
// leases/ same-path rule (2026-08-04): a released tombstone beats a
// plain lease in either direction, two tombstones keep the earlier
// close, two plain leases keep the later expiry — and both merge
// directions produce the identical tree, so a stand-down's immediate
// closure is never a merge-timing coin flip.
func TestLeaseMergeTombstoneRules(t *testing.T) {
	sharedExp := testBase.Add(30 * time.Minute)
	standDown := testBase.Add(45 * time.Minute)
	laterClose := testBase.Add(50 * time.Minute)
	renewal := testBase.Add(2 * time.Hour)
	shorterRenewal := testBase.Add(90 * time.Minute)

	cases := []struct {
		name         string
		writeA       func(p peer, claim string) error
		writeB       func(p peer, claim string) error
		wantExpiry   time.Time
		wantReleased bool
	}{
		{
			// The plain copy expires LATER than the tombstone — expiry
			// alone would resurrect the lease; the marker must win.
			name:         "A's tombstone beats B's later plain expiry",
			writeA:       func(p peer, c string) error { return p.store.ReleaseLease(c, standDown) },
			writeB:       func(p peer, c string) error { return p.store.WriteLease(c, renewal) },
			wantExpiry:   standDown,
			wantReleased: true,
		},
		{
			name:         "B's tombstone beats A's later plain expiry",
			writeA:       func(p peer, c string) error { return p.store.WriteLease(c, renewal) },
			writeB:       func(p peer, c string) error { return p.store.ReleaseLease(c, standDown) },
			wantExpiry:   standDown,
			wantReleased: true,
		},
		{
			// Cannot happen with honest writers (one daemon owns each
			// lease); resolved fail-safe anyway: the earlier close wins.
			name:         "two tombstones keep the earlier close",
			writeA:       func(p peer, c string) error { return p.store.ReleaseLease(c, laterClose) },
			writeB:       func(p peer, c string) error { return p.store.ReleaseLease(c, standDown) },
			wantExpiry:   standDown,
			wantReleased: true,
		},
		{
			name:         "two plain leases keep the later expiry",
			writeA:       func(p peer, c string) error { return p.store.WriteLease(c, renewal) },
			writeB:       func(p peer, c string) error { return p.store.WriteLease(c, shorterRenewal) },
			wantExpiry:   renewal,
			wantReleased: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, b := newPair(t)
			claim := tick(t, 2)

			// Shared history: the task, the claim, and its lease.
			if err := a.store.AppendBatch(store.Batch{Events: []event.Event{
				evt(t, 1, event.TypeTaskCreated, "brandon", "m-a", "t1", event.TaskCreated{Title: "fix login"}),
				evt(t, 2, event.TypeClaimMade, "brandon/impl-1", "m-a", "t1", event.ClaimMade{}),
			}}); err != nil {
				t.Fatal(err)
			}
			if err := a.store.WriteLease(claim, sharedExp); err != nil {
				t.Fatal(err)
			}
			cycle(t, a)
			cycle(t, b)

			// Divergence: each side overwrites the same lease path.
			if err := tc.writeA(a, claim); err != nil {
				t.Fatal(err)
			}
			if err := tc.writeB(b, claim); err != nil {
				t.Fatal(err)
			}

			t1 := mergeBothWays(t, a, b)

			leasePath := "leases/" + claim + ".json"
			oid, ok := t1[leasePath]
			if !ok {
				t.Fatalf("merged tree lost the lease file %s", leasePath)
			}
			data, err := a.git.CatFile(oid)
			if err != nil {
				t.Fatal(err)
			}
			expiry, released, err := store.DecodeLeaseState(data)
			if err != nil {
				t.Fatalf("decode merged lease %s: %v", data, err)
			}
			if !expiry.Equal(tc.wantExpiry) || released != tc.wantReleased {
				t.Fatalf("merged lease = expiry %v, released %v; want %v, %v",
					expiry, released, tc.wantExpiry, tc.wantReleased)
			}
		})
	}
}

func TestNewerPeerOwnsViews(t *testing.T) {
	a, b := newPair(t)

	if err := a.store.AppendBatch(store.Batch{Events: []event.Event{
		evt(t, 1, event.TypeTaskCreated, "brandon", "m-a", "t1", event.TaskCreated{Title: "fix login"}),
	}, Files: map[string][]byte{
		views.MetaPath: []byte(`{"format":1}` + "\n"),
		"backlog.md":   []byte("OLD RENDERING\n"),
	}}); err != nil {
		t.Fatal(err)
	}
	cycle(t, a)
	cycle(t, b)

	// B impersonates a newer tuhdoo: higher stamp, different views.
	if err := b.store.AppendBatch(store.Batch{Files: map[string][]byte{
		views.MetaPath: []byte(`{"format":99}` + "\n"),
		"backlog.md":   []byte("RENDERED BY THE FUTURE\n"),
	}}); err != nil {
		t.Fatal(err)
	}
	cycle(t, b)

	// A diverges, then merges: the newer peer's views must win wholesale
	// and A must not regenerate over them.
	if err := a.store.AppendBatch(store.Batch{Events: []event.Event{
		evt(t, 2, event.TypeNoteAdded, "brandon", "m-a", "t1", event.NoteAdded{Text: "diverge"}),
	}}); err != nil {
		t.Fatal(err)
	}
	cycle(t, a)

	tree := head(t, a)
	meta, err := a.git.CatFile(tree[views.MetaPath])
	if err != nil {
		t.Fatal(err)
	}
	if got := views.Format(meta); got != 99 {
		t.Fatalf("merged stamp format = %d, want the newer peer's 99", got)
	}
	backlog, err := a.git.CatFile(tree["backlog.md"])
	if err != nil {
		t.Fatal(err)
	}
	if string(backlog) != "RENDERED BY THE FUTURE\n" {
		t.Fatalf("merged backlog = %q, want the newer peer's rendering", backlog)
	}
}
