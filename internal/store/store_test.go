package store

// Integration tests: throwaway repositories in t.TempDir(), real git.
// The repo-setup pattern follows internal/gitx/gitx_test.go.

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/brandonbews/tuhdoo/internal/event"
	"github.com/brandonbews/tuhdoo/internal/gitx"
)

var testIdent = gitx.Identity{Name: "Test Bot", Email: "bot@example.com"}

func setGitEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_AUTHOR_NAME", testIdent.Name)
	t.Setenv("GIT_AUTHOR_EMAIL", testIdent.Email)
	t.Setenv("GIT_COMMITTER_NAME", testIdent.Name)
	t.Setenv("GIT_COMMITTER_EMAIL", testIdent.Email)
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
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

// newStore builds a fresh repository and an initialized Store on it.
func newStore(t *testing.T) (*Store, string) {
	t.Helper()
	setGitEnv(t)
	dir := t.TempDir()
	runGit(t, dir, "init", "--quiet", "-b", "main")
	g, err := gitx.New(dir)
	if err != nil {
		t.Fatalf("gitx.New: %v", err)
	}
	s := New(g, "", testIdent)
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return s, dir
}

// newEvent builds a valid event with a fresh ULID.
func newEvent(t *testing.T, n int) event.Event {
	t.Helper()
	id, err := event.NewID(time.Now(), rand.Reader)
	if err != nil {
		t.Fatalf("NewID: %v", err)
	}
	e, err := event.New(id, "task.created", 1, "brandon/impl-2", "m-test", "t-x",
		map[string]int{"n": n})
	if err != nil {
		t.Fatalf("event.New: %v", err)
	}
	return e
}

// commitCount returns the number of commits reachable from the data branch.
func commitCount(t *testing.T, dir string) int {
	t.Helper()
	out := strings.TrimSpace(runGit(t, dir, "rev-list", "--count", DefaultRef))
	n, err := strconv.Atoi(out)
	if err != nil {
		t.Fatalf("rev-list --count returned %q", out)
	}
	return n
}

// treePaths returns path → blob OID at the branch head, optionally
// filtered to a prefix.
func treePaths(t *testing.T, s *Store, prefix string) map[string]string {
	t.Helper()
	head, err := s.git.ReadRef(s.ref)
	if err != nil {
		t.Fatalf("ReadRef: %v", err)
	}
	entries, err := s.git.LsTree(head)
	if err != nil {
		t.Fatalf("LsTree: %v", err)
	}
	m := make(map[string]string)
	for _, e := range entries {
		if strings.HasPrefix(e.Path, prefix) {
			m[e.Path] = e.OID
		}
	}
	return m
}

// assertNoWorktreeFiles fails if the branch machinery ever materialized
// files in the working directory (the branch must never be checked out).
func assertNoWorktreeFiles(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, de := range entries {
		if de.Name() != ".git" {
			t.Errorf("working directory contains %q; the data branch must never be checked out", de.Name())
		}
	}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestInitCreatesOrphanRoot(t *testing.T) {
	s, dir := newStore(t)

	head, err := s.git.ReadRef(DefaultRef)
	if err != nil {
		t.Fatalf("ReadRef after Init: %v", err)
	}

	// Parentless root on the empty tree, verified with raw git.
	raw := runGit(t, dir, "cat-file", "-p", head)
	if strings.Contains(raw, "parent ") {
		t.Errorf("root commit has a parent:\n%s", raw)
	}
	if entries, err := s.git.LsTree(head); err != nil || len(entries) != 0 {
		t.Errorf("root tree entries = %v, %v; want empty", entries, err)
	}

	// Double Init is a no-op: the head must not move.
	if err := s.Init(); err != nil {
		t.Fatalf("second Init: %v", err)
	}
	if got, _ := s.git.ReadRef(DefaultRef); got != head {
		t.Errorf("second Init moved head from %s to %s", head, got)
	}

	assertNoWorktreeFiles(t, dir)
}

// raceGit simulates a concurrent init: the first ReadRef claims the
// branch is missing even though another writer has already created it.
type raceGit struct {
	gitx.Git
	calls int
}

func (r *raceGit) ReadRef(ref string) (string, error) {
	r.calls++
	if r.calls == 1 {
		return "", gitx.ErrRefNotFound
	}
	return r.Git.ReadRef(ref)
}

func TestInitToleratesConcurrentCreation(t *testing.T) {
	s, _ := newStore(t) // branch already exists
	head, err := s.git.ReadRef(DefaultRef)
	if err != nil {
		t.Fatal(err)
	}

	racer := New(&raceGit{Git: s.git}, "", testIdent)
	// Init sees "missing", builds a root, loses the UpdateRef CAS to the
	// existing branch — and must treat that as success.
	if err := racer.Init(); err != nil {
		t.Fatalf("Init after losing the creation race: %v", err)
	}
	if got, _ := s.git.ReadRef(DefaultRef); got != head {
		t.Errorf("losing Init moved head from %s to %s", head, got)
	}
}

func TestBatcherDebounceCombinesCommits(t *testing.T) {
	s, dir := newStore(t)
	b := NewBatcher(s, 50*time.Millisecond)

	const n = 10
	events := make([]event.Event, n)
	for i := range events {
		events[i] = newEvent(t, i)
		b.Add(events[i])
	}

	waitFor(t, "debounced flush", func() bool {
		return len(treePaths(t, s, "events/")) == n
	})
	if err := b.LastError(); err != nil {
		t.Fatalf("LastError after flush: %v", err)
	}

	// Strictly fewer commits than events: root + flush commits < root + n.
	if got := commitCount(t, dir); got-1 >= n {
		t.Errorf("%d events produced %d commits; debounce must combine them", n, got-1)
	}

	// Every event landed at its date-sharded path.
	paths := treePaths(t, s, "events/")
	for _, e := range events {
		path, err := event.Path(e.ID)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := paths[path]; !ok {
			t.Errorf("event %s missing at %s", e.ID, path)
		}
	}

	// Flush commits eagerly, without waiting out the quiet interval.
	before := commitCount(t, dir)
	b.Add(newEvent(t, n))
	if err := b.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if got := commitCount(t, dir); got != before+1 {
		t.Errorf("Flush produced %d commits, want exactly 1", got-before)
	}
	// And a drained Batcher's timer must not commit again.
	time.Sleep(120 * time.Millisecond)
	if got := commitCount(t, dir); got != before+1 {
		t.Errorf("timer fired after Flush: %d extra commits", got-before-1)
	}

	assertNoWorktreeFiles(t, dir)
}

func TestLoadEventsRoundTrip(t *testing.T) {
	s, dir := newStore(t)

	const n = 5
	written := make(map[string][]byte, n) // id → stored bytes
	var batch Batch
	for i := 0; i < n; i++ {
		e := newEvent(t, i)
		enc, err := event.Encode(e)
		if err != nil {
			t.Fatal(err)
		}
		written[e.ID] = enc
		batch.Events = append(batch.Events, e)
	}
	if err := s.AppendBatch(batch); err != nil {
		t.Fatalf("AppendBatch: %v", err)
	}

	loaded, err := s.LoadEvents()
	if err != nil {
		t.Fatalf("LoadEvents: %v", err)
	}
	if len(loaded) != n {
		t.Fatalf("LoadEvents returned %d events, want %d", len(loaded), n)
	}
	seen := make(map[string]bool, n)
	for _, e := range loaded {
		want, ok := written[e.ID]
		if !ok {
			t.Errorf("LoadEvents returned unknown id %s", e.ID)
			continue
		}
		if seen[e.ID] {
			t.Errorf("LoadEvents returned id %s twice", e.ID)
		}
		seen[e.ID] = true
		got, err := event.Encode(e)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("id %s: re-encoded bytes differ\ngot  %s\nwant %s", e.ID, got, want)
		}
	}
	if len(seen) != n {
		t.Errorf("LoadEvents id set has %d of %d written ids", len(seen), n)
	}

	assertNoWorktreeFiles(t, dir)
}

func TestLeasesDoNotTouchEvents(t *testing.T) {
	s, dir := newStore(t)

	// Seed some events so there is an events/ subtree to protect.
	if err := s.AppendBatch(Batch{Events: []event.Event{newEvent(t, 0), newEvent(t, 1)}}); err != nil {
		t.Fatal(err)
	}
	eventsBefore := treePaths(t, s, "events/")
	if len(eventsBefore) != 2 {
		t.Fatalf("seed events = %v", eventsBefore)
	}

	exp1 := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	exp2 := time.Date(2026, 7, 29, 12, 15, 0, 0, time.UTC)

	steps := []struct {
		name string
		op   func() error
	}{
		{"write c1", func() error { return s.WriteLease("c1", exp1) }},
		{"overwrite c1", func() error { return s.WriteLease("c1", exp2) }},
		{"write c2", func() error { return s.WriteLease("c2", exp2) }},
		{"delete c1", func() error { return s.DeleteLease("c1") }},
	}
	for _, step := range steps {
		if err := step.op(); err != nil {
			t.Fatalf("%s: %v", step.name, err)
		}
		if got := treePaths(t, s, "events/"); fmt.Sprint(got) != fmt.Sprint(eventsBefore) {
			t.Fatalf("%s changed events/:\nbefore %v\nafter  %v", step.name, eventsBefore, got)
		}
	}

	leases, err := s.ReadLeases()
	if err != nil {
		t.Fatalf("ReadLeases: %v", err)
	}
	if len(leases) != 1 {
		t.Fatalf("ReadLeases = %v, want only c2", leases)
	}
	got, ok := leases["c2"]
	if !ok || !got.Equal(exp2) {
		t.Errorf("lease c2 expiry = %v, want %v", got, exp2)
	}

	// The lease file itself stores second-precision RFC3339 UTC.
	if err := s.WriteLease("c3", time.Date(2026, 7, 29, 13, 0, 0, 999_999_999, time.UTC)); err != nil {
		t.Fatal(err)
	}
	leases, err = s.ReadLeases()
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 7, 29, 13, 0, 0, 0, time.UTC)
	if !leases["c3"].Equal(want) {
		t.Errorf("lease c3 expiry = %v, want truncated %v", leases["c3"], want)
	}

	assertNoWorktreeFiles(t, dir)
}

func TestAppendBatchRetriesWhenRefMoves(t *testing.T) {
	s, _ := newStore(t)

	// Simulate the sync loop moving the ref between our read and CAS:
	// fail the first UpdateRef as a CAS loss.
	rg := &casLoseOnceGit{Git: s.git}
	racy := New(rg, "", testIdent)
	if err := racy.AppendBatch(Batch{Events: []event.Event{newEvent(t, 0)}}); err != nil {
		t.Fatalf("AppendBatch with one CAS loss: %v", err)
	}
	if rg.updates != 2 {
		t.Errorf("UpdateRef called %d times, want 2 (loss then retry)", rg.updates)
	}
	if got := treePaths(t, s, "events/"); len(got) != 1 {
		t.Errorf("events after retried append = %v, want 1", got)
	}
}

type casLoseOnceGit struct {
	gitx.Git
	updates int
}

func (g *casLoseOnceGit) UpdateRef(ref, newOID, oldOID string) error {
	g.updates++
	if g.updates == 1 {
		return gitx.ErrRefCASFailed
	}
	return g.Git.UpdateRef(ref, newOID, oldOID)
}

func TestAppendBatchRequiresInit(t *testing.T) {
	setGitEnv(t)
	dir := t.TempDir()
	runGit(t, dir, "init", "--quiet", "-b", "main")
	g, err := gitx.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	s := New(g, "", testIdent)
	err = s.AppendBatch(Batch{Events: []event.Event{newEvent(t, 0)}})
	if !errors.Is(err, gitx.ErrRefNotFound) {
		t.Fatalf("AppendBatch without Init: err = %v, want ErrRefNotFound", err)
	}
}
