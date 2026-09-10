package gitx

// CatFiles / CatFile / BlobOID (T2, 2026-09-10: reads are batched;
// object IDs may be computed locally). Real git for the integration
// tests, synthetic output for the batch parser.

import (
	"bytes"
	"crypto/rand"
	"os/exec"
	"strings"
	"testing"
)

// hashObjectViaGit is git's own answer for a blob's object ID, with
// nothing written: the reference BlobOID must match.
func hashObjectViaGit(t *testing.T, dir string, data []byte) string {
	t.Helper()
	cmd := exec.Command("git", "hash-object", "--stdin")
	cmd.Dir = dir
	cmd.Stdin = bytes.NewReader(data)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git hash-object --stdin: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}

// newSHA256Repo initializes a repository in the SHA-256 object format,
// skipping the test when the local git cannot.
func newSHA256Repo(t *testing.T) *CLI {
	t.Helper()
	setGitEnv(t)
	dir := t.TempDir()
	cmd := exec.Command("git", "init", "--quiet", "-b", "main", "--object-format=sha256")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("this git cannot create a sha256 repository (git init --object-format=sha256: %v: %s)", err, strings.TrimSpace(string(out)))
	}
	g, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return g
}

var blobCases = map[string][]byte{
	"empty":  {},
	"text":   []byte("{\"id\":\"x\",\"type\":\"task.created\"}\n"),
	"binary": {0x00, 0x01, 0xfe, 0xff, '\n', '\r', 0x7f, '\n', 0x00},
	// Content shaped like batch headers: a parser reading by line
	// instead of by size would trip over it.
	"lookalike": []byte("deadbeef missing\n0123456789abcdef0123456789abcdef01234567 blob 3\nabc\n"),
}

// CatFiles returns exact bytes for every requested blob — binary and
// empty included — in one call, keyed by OID, with duplicates read
// once; a missing OID fails naming it; a non-blob fails; CatFile is
// CatFiles for one object.
func TestCatFilesBatch(t *testing.T) {
	g := newRepo(t)
	oids := make(map[string]string, len(blobCases)) // name → OID
	var request []string
	for name, data := range blobCases {
		oid, err := g.HashObject(data)
		if err != nil {
			t.Fatalf("HashObject(%s): %v", name, err)
		}
		oids[name] = oid
		request = append(request, oid)
	}
	request = append(request, oids["text"]) // a duplicate: read once, present once

	got, err := g.CatFiles(request)
	if err != nil {
		t.Fatalf("CatFiles: %v", err)
	}
	if len(got) != len(blobCases) {
		t.Fatalf("CatFiles returned %d blobs, want %d (duplicates collapse)", len(got), len(blobCases))
	}
	for name, want := range blobCases {
		if data, ok := got[oids[name]]; !ok || !bytes.Equal(data, want) {
			t.Errorf("CatFiles[%s] = %q (present %v), want %q", name, data, ok, want)
		}
	}

	// CatFile(oid) equals CatFiles([oid])[oid], bytes for bytes.
	for name := range blobCases {
		one, err := g.CatFile(oids[name])
		if err != nil {
			t.Fatalf("CatFile(%s): %v", name, err)
		}
		batch, err := g.CatFiles([]string{oids[name]})
		if err != nil {
			t.Fatalf("CatFiles([%s]): %v", name, err)
		}
		if !bytes.Equal(one, batch[oids[name]]) {
			t.Errorf("CatFile(%s) = %q, CatFiles = %q", name, one, batch[oids[name]])
		}
	}

	// Empty input: nothing spawned, empty map, no error.
	none, err := g.CatFiles(nil)
	if err != nil || len(none) != 0 {
		t.Errorf("CatFiles(nil) = %v, %v; want empty map, nil", none, err)
	}

	// A missing object is an error naming it — never an absent key.
	missing := strings.Repeat("0", 40)
	if _, err := g.CatFiles([]string{oids["text"], missing, oids["empty"]}); err == nil {
		t.Fatal("CatFiles with a missing OID succeeded, want an error")
	} else if !strings.Contains(err.Error(), missing) {
		t.Errorf("missing-OID error does not name the OID: %v", err)
	}
	if _, err := g.CatFile(missing); err == nil || !strings.Contains(err.Error(), missing) {
		t.Errorf("CatFile(missing) = %v, want an error naming %s", err, missing)
	}

	// The data branch holds blobs only: a tree OID is refused, not
	// returned as bytes.
	tree, err := g.MkTree(nil)
	if err != nil {
		t.Fatalf("MkTree: %v", err)
	}
	if _, err := g.CatFiles([]string{tree}); err == nil || !strings.Contains(err.Error(), "tree") {
		t.Errorf("CatFiles(tree) = %v, want an error saying it is a tree", err)
	}
}

// The batch parser over synthetic output: exact sizes, binary bodies,
// the empty blob, and every malformation is an error rather than a
// misattributed or silently absent blob.
func TestParseCatFileBatch(t *testing.T) {
	a := strings.Repeat("a", 40)
	b := strings.Repeat("b", 40)
	cases := []struct {
		name    string
		out     string
		oids    []string
		want    map[string]string
		wantErr string // substring; "" means success
	}{
		{"empty blob", a + " blob 0\n\n", []string{a}, map[string]string{a: ""}, ""},
		{"binary body with newlines and NULs", a + " blob 5\n\x00\n\xff\n\x00\n", []string{a}, map[string]string{a: "\x00\n\xff\n\x00"}, ""},
		{"two records in request order", a + " blob 2\nhi\n" + b + " blob 0\n\n", []string{a, b}, map[string]string{a: "hi", b: ""}, ""},
		{"body that looks like headers", a + " blob 29\ndeadbeef missing\n" + b[:6] + " blob\n\n", []string{a}, map[string]string{a: "deadbeef missing\n" + b[:6] + " blob\n"}, ""},
		{"no request, no output", "", nil, map[string]string{}, ""},
		{"missing object", a + " missing\n", []string{a}, nil, a},
		{"missing object among others", a + " blob 1\nx\n" + b + " missing\n", []string{a, b}, nil, b},
		{"output ends before the record", "", []string{a}, nil, "ended early"},
		{"body shorter than declared", a + " blob 5\nab", []string{a}, nil, "ended early"},
		{"body not newline-terminated", a + " blob 2\nab", []string{a}, nil, "newline"},
		{"record for the wrong object", b + " blob 0\n\n", []string{a}, nil, "expected"},
		{"not a blob", a + " tree 0\n\n", []string{a}, nil, "tree"},
		{"garbage header", "what?\n", []string{a}, nil, "cannot parse"},
		{"output after the last record", a + " blob 0\n\n" + b + " blob 0\n\n", []string{a}, nil, "unexpected output"},
	}
	for _, c := range cases {
		got, err := parseCatFileBatch([]byte(c.out), c.oids)
		if c.wantErr != "" {
			if err == nil {
				t.Errorf("%s: parsed %q without error, want one mentioning %q", c.name, got, c.wantErr)
			} else if !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("%s: error %q does not mention %q", c.name, err, c.wantErr)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if len(got) != len(c.want) {
			t.Errorf("%s: got %d blobs, want %d", c.name, len(got), len(c.want))
		}
		for oid, want := range c.want {
			if string(got[oid]) != want {
				t.Errorf("%s: blob %s = %q, want %q", c.name, oid, got[oid], want)
			}
		}
	}
}

// BlobOID matches `git hash-object --stdin` for the same bytes, in a
// SHA-1 repository and in a SHA-256 one — nothing is written either way.
func TestBlobOIDMatchesGit(t *testing.T) {
	big := make([]byte, 100*1024)
	if _, err := rand.Read(big); err != nil {
		t.Fatal(err)
	}
	data := map[string][]byte{"big-random": big}
	for name, d := range blobCases {
		data[name] = d
	}

	repos := []struct {
		format string
		repo   func(*testing.T) *CLI
		hexLen int
	}{
		{"sha1", newRepo, 40},
		{"sha256", newSHA256Repo, 64},
	}
	for _, r := range repos {
		t.Run(r.format, func(t *testing.T) {
			g := r.repo(t)
			if g.format != r.format {
				t.Fatalf("New detected object format %q, want %q", g.format, r.format)
			}
			for name, d := range data {
				want := hashObjectViaGit(t, g.dir, d)
				got := g.BlobOID(d)
				if got != want {
					t.Errorf("BlobOID(%s) = %s, want %s (git hash-object)", name, got, want)
				}
				if len(got) != r.hexLen {
					t.Errorf("BlobOID(%s) is %d hex chars, want %d", name, len(got), r.hexLen)
				}
			}
			// Still no object written: git has nothing to show for it.
			cmd := exec.Command("git", "cat-file", "-e", g.BlobOID([]byte("never written")))
			cmd.Dir = g.dir
			if err := cmd.Run(); err == nil {
				t.Error("BlobOID wrote an object; it must only compute the ID")
			}
		})
	}
}
