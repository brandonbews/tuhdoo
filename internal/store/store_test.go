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
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/brandonbews/tuhdoo/internal/event"
	"github.com/brandonbews/tuhdoo/internal/gitx"
)

var testIdent = gitx.Identity{Name: "Test Bot", Email: "bot@example.com"}

func setGitEnv(t testing.TB) {
	t.Helper()
	t.Setenv("GIT_AUTHOR_NAME", testIdent.Name)
	t.Setenv("GIT_AUTHOR_EMAIL", testIdent.Email)
	t.Setenv("GIT_COMMITTER_NAME", testIdent.Name)
	t.Setenv("GIT_COMMITTER_EMAIL", testIdent.Email)
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
}

func runGit(t testing.TB, dir string, args ...string) string {
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
func newStore(t testing.TB) (*Store, string) {
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
func newEvent(t testing.TB, n int) event.Event {
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
		{"release c1", func() error { return s.ReleaseLease("c1", exp1) }},
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
	// Releasing never deletes: c1's file survives as a tombstone whose
	// expiry is the release instant.
	if len(leases) != 2 {
		t.Fatalf("ReadLeases = %v, want c1 (tombstoned) and c2", leases)
	}
	got, ok := leases["c1"]
	if !ok || !got.Equal(exp1) {
		t.Errorf("released lease c1 expiry = %v, want the release instant %v", got, exp1)
	}
	got, ok = leases["c2"]
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

// Lease bytes round-trip through the encode/decode pair in both shapes,
// and the pre-tombstone format (no released field) still decodes — old
// leases on the data branch stay readable forever.
func TestLeaseEncodingRoundTripsAndReadsOldFormat(t *testing.T) {
	instant := time.Date(2026, 8, 4, 9, 30, 0, 999_999_999, time.UTC)
	truncated := time.Date(2026, 8, 4, 9, 30, 0, 0, time.UTC)

	cases := []struct {
		name         string
		data         []byte
		wantExpiry   time.Time
		wantReleased bool
	}{
		{"plain lease", EncodeLease(instant), truncated, false},
		{"released tombstone", EncodeLeaseTombstone(instant), truncated, true},
		{"old format without released field", []byte(`{"expires":"2026-08-04T09:30:00Z"}`), truncated, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			expiry, released, err := DecodeLeaseState(tc.data)
			if err != nil {
				t.Fatalf("DecodeLeaseState(%s): %v", tc.data, err)
			}
			if !expiry.Equal(tc.wantExpiry) || released != tc.wantReleased {
				t.Errorf("DecodeLeaseState(%s) = %v, %v; want %v, %v",
					tc.data, expiry, released, tc.wantExpiry, tc.wantReleased)
			}
			// The expiry-only reader (replay's path) agrees: a tombstone
			// is just a lease lapsed at its instant.
			expiryOnly, err := DecodeLease(tc.data)
			if err != nil {
				t.Fatalf("DecodeLease(%s): %v", tc.data, err)
			}
			if !expiryOnly.Equal(tc.wantExpiry) {
				t.Errorf("DecodeLease(%s) = %v, want %v", tc.data, expiryOnly, tc.wantExpiry)
			}
		})
	}

	// Plain leases keep their pre-tombstone bytes: the released field is
	// omitted, not written false, so old binaries see nothing new.
	if plain := EncodeLease(instant); bytes.Contains(plain, []byte("released")) {
		t.Errorf("EncodeLease bytes carry a released field: %s", plain)
	}
	if tomb := EncodeLeaseTombstone(instant); !bytes.Contains(tomb, []byte(`"released":true`)) {
		t.Errorf("EncodeLeaseTombstone bytes lack the released marker: %s", tomb)
	}
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

// Files staged with AddFiles ride the next commit alongside events —
// the mechanism views (T6) travel on — and ReadFile reads them back.
func TestFilesRideBatchesAndReadFile(t *testing.T) {
	s, dir := newStore(t)
	b := NewBatcher(s, 50*time.Millisecond)

	b.Add(newEvent(t, 0))
	b.AddFiles(map[string][]byte{"backlog.md": []byte("stale")})
	b.AddFiles(map[string][]byte{"backlog.md": []byte("fresh"), "README.md": []byte("hello")})
	before := commitCount(t, dir)
	if err := b.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if got := commitCount(t, dir); got != before+1 {
		t.Errorf("event + files landed in %d commits, want exactly 1", got-before)
	}

	got, err := s.ReadFile("backlog.md")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "fresh" {
		t.Errorf("backlog.md = %q, want %q (later staging must win)", got, "fresh")
	}
	if got, err := s.ReadFile("README.md"); err != nil || string(got) != "hello" {
		t.Errorf("README.md = %q, %v; want %q, nil", got, err, "hello")
	}
	if got, err := s.ReadFile("absent.md"); err != nil || got != nil {
		t.Errorf("ReadFile(absent) = %q, %v; want nil, nil", got, err)
	}

	// A drained batcher stays drained: flushing again commits nothing.
	if err := b.Flush(); err != nil {
		t.Fatalf("second Flush: %v", err)
	}
	if got := commitCount(t, dir); got != before+1 {
		t.Errorf("drained Flush produced %d extra commits, want 0", got-before-1)
	}

	assertNoWorktreeFiles(t, dir)
}

// countingGit counts CatFile calls, to observe the decode caches: blobs
// are content-addressed, so each one should be read from git only once.
type countingGit struct {
	gitx.Git
	catFiles int
}

func (g *countingGit) CatFile(oid string) ([]byte, error) {
	g.catFiles++
	return g.Git.CatFile(oid)
}

// The decode caches mean repeated loads cost no cat-file subprocesses:
// only blobs never seen before are read, renewed leases are re-read
// exactly once, and the lease cache tracks the live tree instead of
// growing with renewal churn.
func TestLoadReplayInputCachesDecodes(t *testing.T) {
	setGitEnv(t)
	dir := t.TempDir()
	runGit(t, dir, "init", "--quiet", "-b", "main")
	g, err := gitx.New(dir)
	if err != nil {
		t.Fatalf("gitx.New: %v", err)
	}
	cg := &countingGit{Git: g}
	s := New(cg, "", testIdent)
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	evs := []event.Event{newEvent(t, 0), newEvent(t, 1), newEvent(t, 2)}
	if err := s.AppendBatch(Batch{Events: evs}); err != nil {
		t.Fatalf("AppendBatch: %v", err)
	}
	exp1 := time.Now().Add(10 * time.Minute).UTC().Truncate(time.Second)
	if err := s.WriteLease("c1", exp1); err != nil {
		t.Fatalf("WriteLease: %v", err)
	}

	events1, leases1, err := s.LoadReplayInput()
	if err != nil {
		t.Fatalf("LoadReplayInput: %v", err)
	}
	if len(events1) != 3 || len(leases1) != 1 || !leases1["c1"].Equal(exp1) {
		t.Fatalf("first load = %d events, leases %v; want 3 events, c1 -> %v", len(events1), leases1, exp1)
	}
	if cg.catFiles != 4 {
		t.Errorf("first load read %d blobs, want 4 (3 events + 1 lease)", cg.catFiles)
	}

	// Second load with nothing changed: zero blob reads, same results.
	events2, leases2, err := s.LoadReplayInput()
	if err != nil {
		t.Fatalf("second LoadReplayInput: %v", err)
	}
	if cg.catFiles != 4 {
		t.Errorf("unchanged reload read %d extra blobs, want 0", cg.catFiles-4)
	}
	if !reflect.DeepEqual(events1, events2) || !reflect.DeepEqual(leases1, leases2) {
		t.Errorf("cached reload changed results:\nevents %v vs %v\nleases %v vs %v",
			events1, events2, leases1, leases2)
	}

	// A lease renewal writes a new blob: exactly one new read, fresh
	// expiry served, and the superseded blob leaves the cache.
	exp2 := exp1.Add(10 * time.Minute)
	if err := s.WriteLease("c1", exp2); err != nil {
		t.Fatalf("renew WriteLease: %v", err)
	}
	_, leases3, err := s.LoadReplayInput()
	if err != nil {
		t.Fatalf("post-renewal LoadReplayInput: %v", err)
	}
	if cg.catFiles != 5 {
		t.Errorf("renewal reload read %d extra blobs, want 1", cg.catFiles-4)
	}
	if !leases3["c1"].Equal(exp2) {
		t.Errorf("post-renewal lease = %v, want %v", leases3["c1"], exp2)
	}
	if len(s.leaseByOID) != 1 {
		t.Errorf("lease cache holds %d entries after renewal, want 1", len(s.leaseByOID))
	}

	// A new event costs exactly its own read.
	if err := s.AppendBatch(Batch{Events: []event.Event{newEvent(t, 3)}}); err != nil {
		t.Fatalf("AppendBatch: %v", err)
	}
	events4, _, err := s.LoadReplayInput()
	if err != nil {
		t.Fatalf("post-append LoadReplayInput: %v", err)
	}
	if len(events4) != 4 {
		t.Errorf("post-append load = %d events, want 4", len(events4))
	}
	if cg.catFiles != 6 {
		t.Errorf("post-append reload read %d extra blobs, want 1", cg.catFiles-5)
	}

	// Releasing a lease overwrites it with a tombstone blob: one new
	// read, the release instant served as the expiry, and the cache
	// still tracks exactly the one live blob.
	releasedAt := exp1.Add(5 * time.Minute)
	if err := s.ReleaseLease("c1", releasedAt); err != nil {
		t.Fatalf("ReleaseLease: %v", err)
	}
	_, leases5, err := s.LoadReplayInput()
	if err != nil {
		t.Fatalf("post-release LoadReplayInput: %v", err)
	}
	if len(leases5) != 1 || !leases5["c1"].Equal(releasedAt) || len(s.leaseByOID) != 1 || cg.catFiles != 7 {
		t.Errorf("post-release: leases %v, cache %d, blob reads %d; want c1 -> %v, 1, 7",
			leases5, len(s.leaseByOID), cg.catFiles, releasedAt)
	}

	assertNoWorktreeFiles(t, dir)
}
