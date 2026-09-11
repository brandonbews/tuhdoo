package gitx

// The private index (T2, 2026-09-10): ReadTree seeds it from a tree,
// UpdateIndex feeds only the changed paths, WriteTree writes the tree
// out — all against `<git-dir>/tuhdoo/index`, never the user's index.
// Real git throughout; the user's index and working tree are checked
// untouched after every operation.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// lsTreeMap is a raw ls-tree of rev as path → OID, read by git
// directly so CLI's work is checked independently.
func lsTreeMap(t *testing.T, g *CLI, rev string) map[string]string {
	t.Helper()
	out := runGit(t, g.dir, "ls-tree", "-r", "--full-tree", rev)
	m := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		meta, path, _ := strings.Cut(line, "\t")
		m[path] = strings.Fields(meta)[2]
	}
	return m
}

// assertUserIndexUntouched fails if the repository's own index exists
// or the working tree holds anything but .git — the private index must
// never leak into either.
func assertUserIndexUntouched(t *testing.T, g *CLI) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(g.gitDir, "index")); !os.IsNotExist(err) {
		t.Errorf("the user's index %s exists (stat err %v); the private index must not touch it", filepath.Join(g.gitDir, "index"), err)
	}
	entries, err := os.ReadDir(g.dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, de := range entries {
		if de.Name() != ".git" {
			t.Errorf("working directory contains %q; nothing may be checked out", de.Name())
		}
	}
}

// ReadTree seeds, UpdateIndex applies adds, replacements, and mode-0
// removals through the same private index at any depth, and WriteTree
// yields exactly the tree those changes describe.
func TestPrivateIndexRoundTrip(t *testing.T) {
	g := newRepo(t)
	base := map[string][]byte{
		"README.md":                []byte("hello\n"),
		"events/2026/07/29/a.json": []byte(`{"id":"a"}`),
		"events/2026/07/29/b.json": []byte(`{"id":"b"}`),
		"leases/c1.json":           []byte(`{"expires":"2026-07-29T12:00:00Z"}`),
	}
	commit := writeCommit(t, g, base, "base\n")
	want := lsTreeMap(t, g, commit)

	// Seeding from a commit OID (a tree-ish) is what the store does
	// with its head.
	if err := g.ReadTree(commit); err != nil {
		t.Fatalf("ReadTree: %v", err)
	}
	if _, err := os.Stat(g.indexPath()); err != nil {
		t.Fatalf("private index not created at %s: %v", g.indexPath(), err)
	}
	assertUserIndexUntouched(t, g)

	// Nothing changed: the written tree is the seeded tree.
	tree, err := g.WriteTree()
	if err != nil {
		t.Fatalf("WriteTree after seed: %v", err)
	}
	if got := lsTreeMap(t, g, tree); len(got) != len(want) || got["README.md"] != want["README.md"] {
		t.Fatalf("tree after seed = %v, want the seeded %v", got, want)
	}

	newBlob, err := g.HashObject([]byte(`{"id":"c"}`))
	if err != nil {
		t.Fatal(err)
	}
	replaced, err := g.HashObject([]byte("hello again\n"))
	if err != nil {
		t.Fatal(err)
	}
	err = g.UpdateIndex([]TreeEntry{
		{Path: "events/2026/08/01/c.json", OID: newBlob}, // new file, new directory
		{Path: "README.md", OID: replaced},               // replacement
		{Path: "leases/c1.json"},                         // mode-0 removal
		{Path: "never/existed.json"},                     // removing an absent path is a no-op
	})
	if err != nil {
		t.Fatalf("UpdateIndex: %v", err)
	}
	tree, err = g.WriteTree()
	if err != nil {
		t.Fatalf("WriteTree: %v", err)
	}
	got := lsTreeMap(t, g, tree)
	want["events/2026/08/01/c.json"] = newBlob
	want["README.md"] = replaced
	delete(want, "leases/c1.json")
	if len(got) != len(want) {
		t.Fatalf("tree has %d entries %v, want %d %v", len(got), got, len(want), want)
	}
	for path, oid := range want {
		if got[path] != oid {
			t.Errorf("tree[%s] = %s, want %s", path, got[path], oid)
		}
	}
	assertUserIndexUntouched(t, g)

	// Empty input spawns nothing and changes nothing.
	if err := g.UpdateIndex(nil); err != nil {
		t.Fatalf("UpdateIndex(nil): %v", err)
	}
	again, err := g.WriteTree()
	if err != nil || again != tree {
		t.Fatalf("WriteTree after empty update = %s, %v; want %s unchanged", again, err, tree)
	}
}

// ReadTree recovers the index from every broken state a crashed daemon
// can leave: a stale index.lock, a garbage index file, a missing
// runtime directory — and seeding replaces the contents wholesale, so
// entries from an earlier, unrelated seed never survive into the next
// tree.
func TestReadTreeRecoversAndReplaces(t *testing.T) {
	g := newRepo(t)
	one := writeCommit(t, g, map[string][]byte{"one.md": []byte("1")}, "one\n")
	two := writeCommit(t, g, map[string][]byte{"sub/two.md": []byte("2")}, "two\n")

	// Missing runtime directory: created.
	if err := os.RemoveAll(filepath.Dir(g.indexPath())); err != nil {
		t.Fatal(err)
	}
	if err := g.ReadTree(one); err != nil {
		t.Fatalf("ReadTree into a missing dir: %v", err)
	}

	// Stale lock: cleared, not fatal.
	if err := os.WriteFile(g.indexPath()+".lock", nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := g.ReadTree(two); err != nil {
		t.Fatalf("ReadTree with a stale index.lock: %v", err)
	}
	if _, err := os.Stat(g.indexPath() + ".lock"); !os.IsNotExist(err) {
		t.Fatalf("stale index.lock still present after ReadTree (stat err %v)", err)
	}
	// Wholesale replacement: one.md from the first seed is gone.
	tree, err := g.WriteTree()
	if err != nil {
		t.Fatal(err)
	}
	if got := lsTreeMap(t, g, tree); len(got) != 1 || got["sub/two.md"] == "" {
		t.Fatalf("tree after reseed = %v, want only sub/two.md", got)
	}

	// Garbage index file: overwritten by the seed.
	if err := os.WriteFile(g.indexPath(), []byte("not an index"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := g.ReadTree(one); err != nil {
		t.Fatalf("ReadTree over a garbage index: %v", err)
	}
	tree, err = g.WriteTree()
	if err != nil {
		t.Fatal(err)
	}
	if got := lsTreeMap(t, g, tree); len(got) != 1 || got["one.md"] == "" {
		t.Fatalf("tree after reseed over garbage = %v, want only one.md", got)
	}
	assertUserIndexUntouched(t, g)
}

// WriteTree refuses a missing index instead of returning git's silent
// empty tree — the one answer that would drop every file on the branch.
func TestWriteTreeRefusesMissingIndex(t *testing.T) {
	g := newRepo(t)
	if _, err := g.WriteTree(); err == nil || !strings.Contains(err.Error(), "reseed") {
		t.Fatalf("WriteTree with no index = %v, want a refusal telling the caller to reseed", err)
	}
	commit := writeCommit(t, g, map[string][]byte{"a": []byte("1")}, "a\n")
	if err := g.ReadTree(commit); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(g.indexPath()); err != nil {
		t.Fatal(err)
	}
	if _, err := g.WriteTree(); err == nil {
		t.Fatal("WriteTree after the index vanished succeeded; want an error, not the empty tree")
	}
}

// UpdateIndex validates paths exactly as MkTree does.
func TestUpdateIndexRejectsBadPaths(t *testing.T) {
	g := newRepo(t)
	if err := g.ReadTree(writeCommit(t, g, map[string][]byte{"a": []byte("1")}, "a\n")); err != nil {
		t.Fatal(err)
	}
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
		{{Path: "gone/../x"}}, // removals are validated too
	}
	for _, entries := range bad {
		if err := g.UpdateIndex(entries); err == nil {
			t.Errorf("UpdateIndex(%v) succeeded, want error", entries)
		}
	}
}

// The removal record carries the zero object name in the repository's
// format: 64 hex zeros in a SHA-256 repository, not SHA-1's 40.
func TestUpdateIndexRemovalInSHA256Repo(t *testing.T) {
	g := newSHA256Repo(t)
	commit := writeCommit(t, g, map[string][]byte{"keep": []byte("k"), "drop": []byte("d")}, "two\n")
	if err := g.ReadTree(commit); err != nil {
		t.Fatal(err)
	}
	if err := g.UpdateIndex([]TreeEntry{{Path: "drop"}}); err != nil {
		t.Fatalf("UpdateIndex removal: %v", err)
	}
	tree, err := g.WriteTree()
	if err != nil {
		t.Fatal(err)
	}
	if got := lsTreeMap(t, g, tree); len(got) != 1 || got["keep"] == "" {
		t.Fatalf("tree after removal = %v, want only keep", got)
	}
}

// New resolves the git dir rev-parse reports — relative ".git" from a
// repository root — to an absolute path, so the private index lands in
// `<git-dir>/tuhdoo/index` whatever the process's working directory.
func TestIndexPathUnderGitDir(t *testing.T) {
	g := newRepo(t)
	want := filepath.Join(g.dir, ".git", "tuhdoo", "index")
	if got := g.indexPath(); got != want {
		t.Fatalf("indexPath = %s, want %s", got, want)
	}
	if !filepath.IsAbs(g.gitDir) {
		t.Fatalf("gitDir %q is not absolute", g.gitDir)
	}
}
