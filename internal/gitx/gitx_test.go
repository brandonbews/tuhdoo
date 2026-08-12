package gitx

// Integration tests: every test builds throwaway repositories in
// t.TempDir() and runs the real git binary.

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

var testIdent = Identity{Name: "Test Bot", Email: "bot@example.com"}

// setGitEnv pins identity and isolates the user's git config so tests
// behave identically on any machine. t.Setenv also applies to the
// subprocesses CLI spawns.
func setGitEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_AUTHOR_NAME", testIdent.Name)
	t.Setenv("GIT_AUTHOR_EMAIL", testIdent.Email)
	t.Setenv("GIT_COMMITTER_NAME", testIdent.Name)
	t.Setenv("GIT_COMMITTER_EMAIL", testIdent.Email)
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
}

// runGit runs a raw git command for test setup and verification —
// deliberately not through CLI, so tests can check CLI's work
// independently.
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

func newRepo(t *testing.T) *CLI {
	t.Helper()
	setGitEnv(t)
	dir := t.TempDir()
	runGit(t, dir, "init", "--quiet", "-b", "main")
	g, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return g
}

func newBareRemote(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "--quiet", "--bare")
	return dir
}

// writeCommit stores the given files as a commit (no checkout, no
// index) and returns its OID.
func writeCommit(t *testing.T, g *CLI, files map[string][]byte, msg string, parents ...string) string {
	t.Helper()
	var entries []TreeEntry
	for path, data := range files {
		oid, err := g.HashObject(data)
		if err != nil {
			t.Fatalf("HashObject(%q): %v", path, err)
		}
		entries = append(entries, TreeEntry{Path: path, OID: oid})
	}
	tree, err := g.MkTree(entries)
	if err != nil {
		t.Fatalf("MkTree: %v", err)
	}
	commit, err := g.CommitTree(tree, parents, testIdent, msg)
	if err != nil {
		t.Fatalf("CommitTree: %v", err)
	}
	return commit
}

func TestWriteAndReadBackNeverCheckedOut(t *testing.T) {
	g := newRepo(t)

	files := map[string][]byte{
		"README.md":                []byte("tuhdoo data branch\n"),
		"events/2026/07/29/x.json": []byte(`{"id":"x","type":"task.created"}`),
		"events/2026/07/29/y.json": {0x00, 0x01, 0xfe, 0xff, '\n', '\r', 0x7f}, // binary-safety check
		"events/2026/08/01/z.json": []byte(`{"id":"z"}`),
		"views/backlog.md":         []byte("# Backlog\n"),
	}
	commit := writeCommit(t, g, files, "tuhdoo: initial events\n")
	if err := g.UpdateRef("refs/heads/tuhdoo", commit, ""); err != nil {
		t.Fatalf("UpdateRef create: %v", err)
	}

	got, err := g.ReadRef("refs/heads/tuhdoo")
	if err != nil {
		t.Fatalf("ReadRef: %v", err)
	}
	if got != commit {
		t.Fatalf("ReadRef = %s, want %s", got, commit)
	}

	entries, err := g.LsTree(commit)
	if err != nil {
		t.Fatalf("LsTree: %v", err)
	}
	if len(entries) != len(files) {
		t.Fatalf("LsTree returned %d entries, want %d: %v", len(entries), len(files), entries)
	}
	for _, e := range entries {
		want, ok := files[e.Path]
		if !ok {
			t.Fatalf("LsTree listed unexpected path %q", e.Path)
		}
		data, err := g.CatFile(e.OID)
		if err != nil {
			t.Fatalf("CatFile(%q): %v", e.Path, err)
		}
		if !bytes.Equal(data, want) {
			t.Errorf("CatFile(%q) = %q, want %q", e.Path, data, want)
		}
	}

	// The branch was never checked out: the working directory must
	// still contain nothing but .git, and main must still be unborn.
	dirEntries, err := os.ReadDir(g.dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, de := range dirEntries {
		if de.Name() != ".git" {
			t.Errorf("working directory contains %q; the data branch must never be checked out", de.Name())
		}
	}
	if _, err := g.ReadRef("refs/heads/main"); !errors.Is(err, ErrRefNotFound) {
		t.Errorf("refs/heads/main should not exist, got err = %v", err)
	}
}

func TestMkTreeEmptyIsValidRoot(t *testing.T) {
	g := newRepo(t)
	tree, err := g.MkTree(nil)
	if err != nil {
		t.Fatalf("MkTree(nil): %v", err)
	}
	entries, err := g.LsTree(tree)
	if err != nil {
		t.Fatalf("LsTree: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("empty tree lists %v", entries)
	}
	if _, err := g.CommitTree(tree, nil, testIdent, "root\n"); err != nil {
		t.Fatalf("CommitTree on empty tree: %v", err)
	}
}

// The data branch holds blobs only: an entry of any other object kind
// means the tree is not ours, and LsTree must fail, not skip. A
// gitlink (submodule) entry is the shape that pins the arm — git's
// ls-tree types symlinks (120000) as blobs, so a gitlink's "commit" is
// the one non-blob kind a recursive listing can surface.
func TestLsTreeRejectsNonBlobEntries(t *testing.T) {
	g := newRepo(t)
	// Plant the gitlink with raw mktree; --missing admits the dangling
	// commit OID, exactly as a real submodule pointer arrives.
	cmd := exec.Command("git", "mktree", "--missing")
	cmd.Dir = g.dir
	cmd.Stdin = strings.NewReader("160000 commit 0123456789012345678901234567890123456789\tsub\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("mktree: %v\n%s", err, out)
	}
	tree := strings.TrimSpace(string(out))
	if _, err := g.LsTree(tree); err == nil || !strings.Contains(err.Error(), "commit") {
		t.Fatalf("LsTree over a gitlink entry = %v, want the fail-don't-skip rejection naming the object kind", err)
	}
}

func TestMkTreeRejectsBadPaths(t *testing.T) {
	g := newRepo(t)
	oid, err := g.HashObject([]byte("x"))
	if err != nil {
		t.Fatal(err)
	}
	bad := [][]TreeEntry{
		{{Path: "", OID: oid}},
		{{Path: "/abs", OID: oid}},
		{{Path: "a//b", OID: oid}},
		{{Path: "a/", OID: oid}},
		{{Path: "../escape", OID: oid}},
		{{Path: "a/./b", OID: oid}},
		{{Path: "dup", OID: oid}, {Path: "dup", OID: oid}},
		{{Path: "a", OID: oid}, {Path: "a/b", OID: oid}}, // file and directory
	}
	for _, entries := range bad {
		if _, err := g.MkTree(entries); err == nil {
			t.Errorf("MkTree(%v) succeeded, want error", entries)
		}
	}
}

func TestUpdateRefCAS(t *testing.T) {
	g := newRepo(t)
	const ref = "refs/heads/tuhdoo"

	c1 := writeCommit(t, g, map[string][]byte{"a": []byte("1")}, "c1\n")
	c2 := writeCommit(t, g, map[string][]byte{"a": []byte("2")}, "c2\n", c1)

	if err := g.UpdateRef(ref, c1, ""); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Stale expected value: ref is at c1, we claim it is at c2.
	err := g.UpdateRef(ref, c2, c2)
	if !errors.Is(err, ErrRefCASFailed) {
		t.Fatalf("stale old value: err = %v, want ErrRefCASFailed", err)
	}

	// "Must not exist" on a ref that does exist.
	err = g.UpdateRef(ref, c2, "")
	if !errors.Is(err, ErrRefCASFailed) {
		t.Fatalf("create-over-existing: err = %v, want ErrRefCASFailed", err)
	}

	// Expected value on a ref that does not exist.
	err = g.UpdateRef("refs/heads/absent", c2, c1)
	if !errors.Is(err, ErrRefCASFailed) {
		t.Fatalf("update-of-missing: err = %v, want ErrRefCASFailed", err)
	}

	// The failures above must not have moved the ref.
	if got, _ := g.ReadRef(ref); got != c1 {
		t.Fatalf("ref moved to %s despite CAS failures, want %s", got, c1)
	}

	// Correct expected value succeeds.
	if err := g.UpdateRef(ref, c2, c1); err != nil {
		t.Fatalf("valid CAS update: %v", err)
	}
	if got, _ := g.ReadRef(ref); got != c2 {
		t.Fatalf("ReadRef = %s, want %s", got, c2)
	}
}

func TestReadRefNotFound(t *testing.T) {
	g := newRepo(t)
	_, err := g.ReadRef("refs/heads/nope")
	if !errors.Is(err, ErrRefNotFound) {
		t.Fatalf("err = %v, want ErrRefNotFound", err)
	}
}

func TestCommitTreeTwoParents(t *testing.T) {
	g := newRepo(t)

	c1 := writeCommit(t, g, map[string][]byte{"a.json": []byte("a")}, "side one\n")
	c2 := writeCommit(t, g, map[string][]byte{"b.json": []byte("b")}, "side two\n")
	merge := writeCommit(t, g,
		map[string][]byte{"a.json": []byte("a"), "b.json": []byte("b")},
		"tuhdoo: merge\n", c1, c2)

	// Verify the parent list with raw git, independent of CLI.
	raw := runGit(t, g.dir, "cat-file", "-p", merge)
	var parents []string
	for _, line := range strings.Split(raw, "\n") {
		if oid, ok := strings.CutPrefix(line, "parent "); ok {
			parents = append(parents, oid)
		}
	}
	if len(parents) != 2 || parents[0] != c1 || parents[1] != c2 {
		t.Fatalf("merge commit parents = %v, want [%s %s]", parents, c1, c2)
	}
	logOut := runGit(t, g.dir, "log", "--format=%H", merge)
	for _, want := range []string{merge, c1, c2} {
		if !strings.Contains(logOut, want) {
			t.Errorf("git log from merge is missing %s:\n%s", want, logOut)
		}
	}

	if _, err := g.CommitTree(c1, []string{c1, c2, c1}, testIdent, "x\n"); err == nil {
		t.Error("CommitTree with 3 parents succeeded, want error")
	}
}

func TestFetchAndPush(t *testing.T) {
	const ref = "refs/heads/tuhdoo"
	const spec = ref + ":" + ref

	a := newRepo(t)
	remote := newBareRemote(t)
	runGit(t, a.dir, "remote", "add", "origin", remote)

	c1 := writeCommit(t, a, map[string][]byte{"events/x.json": []byte(`{"id":"x"}`)}, "c1\n")
	if err := a.UpdateRef(ref, c1, ""); err != nil {
		t.Fatal(err)
	}
	if err := a.Push("origin", spec); err != nil {
		t.Fatalf("Push: %v", err)
	}
	if got := strings.TrimSpace(runGit(t, remote, "rev-parse", ref)); got != c1 {
		t.Fatalf("remote ref = %s, want %s", got, c1)
	}

	// A second machine fetches and sees identical bytes.
	b := newRepo(t)
	runGit(t, b.dir, "remote", "add", "origin", remote)
	if err := b.Fetch("origin", spec); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if got, err := b.ReadRef(ref); err != nil || got != c1 {
		t.Fatalf("ReadRef after fetch = %s, %v; want %s", got, err, c1)
	}
	entries, err := b.LsTree(c1)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Path != "events/x.json" {
		t.Fatalf("fetched tree = %v", entries)
	}
	data, err := b.CatFile(entries[0].OID)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"id":"x"}` {
		t.Fatalf("fetched blob = %q", data)
	}

	// b advances the remote…
	c2 := writeCommit(t, b, map[string][]byte{"events/x.json": []byte(`{"id":"x"}`), "events/y.json": []byte(`{"id":"y"}`)}, "c2\n", c1)
	if err := b.UpdateRef(ref, c2, c1); err != nil {
		t.Fatal(err)
	}
	if err := b.Push("origin", spec); err != nil {
		t.Fatalf("fast-forward push from b: %v", err)
	}

	// …so a's divergent commit must now be rejected as non-fast-forward.
	c3 := writeCommit(t, a, map[string][]byte{"events/x.json": []byte(`{"id":"x"}`), "events/z.json": []byte(`{"id":"z"}`)}, "c3\n", c1)
	if err := a.UpdateRef(ref, c3, c1); err != nil {
		t.Fatal(err)
	}
	err = a.Push("origin", spec)
	if !errors.Is(err, ErrNonFastForward) {
		t.Fatalf("divergent push: err = %v, want ErrNonFastForward", err)
	}
}

// TestFetchTimeoutClassifiesLikeFetch: the bounded variant preserves
// Fetch's error classification when the remote answers promptly.
func TestFetchTimeoutClassifiesLikeFetch(t *testing.T) {
	const spec = "refs/heads/tuhdoo:refs/tuhdoo/remote"

	g := newRepo(t)
	remote := newBareRemote(t)
	runGit(t, g.dir, "remote", "add", "origin", remote)

	err := g.FetchTimeout("origin", spec, 30*time.Second)
	if !errors.Is(err, ErrRemoteRefMissing) {
		t.Fatalf("fetch of a branch the remote lacks: err = %v, want ErrRemoteRefMissing", err)
	}

	c1 := writeCommit(t, g, map[string][]byte{"events/x.json": []byte(`{"id":"x"}`)}, "c1\n")
	if err := g.UpdateRef("refs/heads/tuhdoo", c1, ""); err != nil {
		t.Fatal(err)
	}
	if err := g.Push("origin", "refs/heads/tuhdoo:refs/heads/tuhdoo"); err != nil {
		t.Fatal(err)
	}
	if err := g.FetchTimeout("origin", spec, 30*time.Second); err != nil {
		t.Fatalf("FetchTimeout: %v", err)
	}
	if got, err := g.ReadRef("refs/tuhdoo/remote"); err != nil || got != c1 {
		t.Fatalf("tracking ref after fetch = %s, %v; want %s", got, err, c1)
	}
}

// TestFetchTimeoutBoundsAHangingRemote: a transport that never answers
// (an ssh command that just sleeps) must not stall the caller past the
// bound — this is the daemon-startup guarantee behind clone-join.
func TestFetchTimeoutBoundsAHangingRemote(t *testing.T) {
	g := newRepo(t)
	runGit(t, g.dir, "remote", "add", "origin", "ssh://tuhdoo-test.invalid/repo.git")
	// git appends the host and remote command; the sh -c wrapper eats
	// them as ignored positional args and just hangs.
	t.Setenv("GIT_SSH_COMMAND", "sh -c 'sleep 60' hang-transport")

	start := time.Now()
	err := g.FetchTimeout("origin", "refs/heads/tuhdoo:refs/tuhdoo/remote", 500*time.Millisecond)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("fetch through a hanging transport succeeded, want a timeout error")
	}
	// Generous ceiling: the bound plus WaitDelay's pipe cleanup, with
	// slack for a loaded CI box — the point is seconds, not minutes.
	if elapsed > 10*time.Second {
		t.Fatalf("fetch not bounded: took %s", elapsed)
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("timeout error should say so, got: %v", err)
	}
}

func TestRemoteURL(t *testing.T) {
	g := newRepo(t)

	url, err := g.RemoteURL("origin")
	if !errors.Is(err, ErrNoRemote) {
		t.Fatalf("no remote: err = %v, want ErrNoRemote", err)
	}
	if url != "" {
		t.Fatalf("no remote: url = %q, want empty", url)
	}

	remote := newBareRemote(t)
	runGit(t, g.dir, "remote", "add", "origin", remote)
	url, err = g.RemoteURL("origin")
	if err != nil {
		t.Fatalf("RemoteURL: %v", err)
	}
	if url != remote {
		t.Fatalf("RemoteURL = %q, want %q", url, remote)
	}
}

func TestNewRejectsNonRepo(t *testing.T) {
	setGitEnv(t)
	if _, err := New(t.TempDir()); err == nil {
		t.Fatal("New on a plain directory succeeded, want error")
	}
}

func TestGitVersionFloor(t *testing.T) {
	cases := []struct {
		major, minor int
		ok           bool
	}{
		{1, 99, false},
		{2, 39, false},
		{2, 40, true},
		{2, 50, true},
		{3, 0, true},
	}
	for _, c := range cases {
		if got := gitVersionOK(c.major, c.minor); got != c.ok {
			t.Errorf("gitVersionOK(%d, %d) = %v, want %v", c.major, c.minor, got, c.ok)
		}
	}
}

func TestParseGitVersion(t *testing.T) {
	cases := []struct {
		in           string
		major, minor int
		wantErr      bool
	}{
		{"git version 2.50.1 (Apple Git-155)", 2, 50, false},
		{"git version 2.40.0", 2, 40, false},
		{"git version 2.43.0.windows.1", 2, 43, false},
		{"nonsense", 0, 0, true},
		{"git version x.y", 0, 0, true},
	}
	for _, c := range cases {
		major, minor, err := parseGitVersion(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("parseGitVersion(%q) succeeded, want error", c.in)
			}
			continue
		}
		if err != nil || major != c.major || minor != c.minor {
			t.Errorf("parseGitVersion(%q) = %d, %d, %v; want %d, %d", c.in, major, minor, err, c.major, c.minor)
		}
	}
}

func TestPushRejectionClassification(t *testing.T) {
	cases := []struct {
		name           string
		stdout, stderr string
		contention     bool
	}{
		{
			name:       "porcelain non-fast-forward",
			stdout:     "To /tmp/remote.git\n!\trefs/heads/tuhdoo:refs/heads/tuhdoo\t[rejected] (non-fast-forward)\nDone\n",
			contention: true,
		},
		{
			name:       "porcelain fetch first",
			stdout:     "To /tmp/remote.git\n!\trefs/heads/tuhdoo:refs/heads/tuhdoo\t[rejected] (fetch first)\nDone\n",
			contention: true,
		},
		{
			// The remote's internal ref-update race, lost: relayed on
			// stderr, no rejection wording on stdout at all. The literal
			// shape from the collision-harness storm (2026-08-03).
			name:       "remote ref-lock race",
			stderr:     "remote: error: cannot lock ref 'refs/heads/tuhdoo': is at 4bb2416435a37fcbdafe2ad3ea75e1c31f7f0b39 but expected d2f36340d15d29afa8c1fc4dcf1a26d21008eeaa\nTo /tmp/remote.git\n !\trefs/heads/tuhdoo:refs/heads/tuhdoo\t[remote rejected] (failed to update ref)\n",
			contention: true,
		},
		{
			name:       "genuine failure stays a failure",
			stderr:     "fatal: '/nope/remote.git' does not appear to be a git repository\n",
			contention: false,
		},
	}
	for _, c := range cases {
		if got := pushRejectionIsContention(c.stdout, c.stderr); got != c.contention {
			t.Errorf("%s: pushRejectionIsContention = %v, want %v", c.name, got, c.contention)
		}
	}
}
