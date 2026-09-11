package gitx

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Minimum supported git version (design doc 002, T2).
const minGitMajor, minGitMinor = 2, 40

// CLI implements Git by running the real git binary as a subprocess
// against one repository directory. Fetch and push therefore inherit
// the user's full auth setup (SSH agents, credential helpers) for free.
type CLI struct {
	dir string
	// gitDir is the repository's git directory, absolute, as reported
	// by `rev-parse --git-dir` at New: the private index lives under
	// it (indexPath), which puts it beside the daemon's runtime files
	// and inside the right directory for a linked worktree, whose git
	// dir is not `<root>/.git`.
	gitDir string
	// format is the repository's object format, "sha1" or "sha256",
	// detected once by New (`rev-parse --show-object-format`) so
	// BlobOID hashes the way this repository's git does.
	format string
}

var _ Git = (*CLI)(nil)

// New returns a CLI operating on the git repository at dir. It fails
// with a clear message when git is missing, older than the supported
// floor, or dir is not a repository.
func New(dir string) (*CLI, error) {
	g := &CLI{dir: dir}
	out, _, err := g.run(nil, nil, "version")
	if err != nil {
		return nil, fmt.Errorf("gitx: running git: %w", err)
	}
	major, minor, err := parseGitVersion(strings.TrimSpace(string(out)))
	if err != nil {
		return nil, fmt.Errorf("gitx: %w", err)
	}
	if !gitVersionOK(major, minor) {
		return nil, fmt.Errorf("gitx: git %d.%d or newer is required, found %d.%d — please upgrade git",
			minGitMajor, minGitMinor, major, minor)
	}
	// One rev-parse proves dir is a repository and reports its object
	// format: each flag prints one line, in flag order.
	out, _, err = g.run(nil, nil, "rev-parse", "--git-dir", "--show-object-format")
	if err != nil {
		return nil, fmt.Errorf("gitx: %s is not a git repository: %w", dir, err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 2 {
		return nil, fmt.Errorf("gitx: detect object format: rev-parse printed %q, want a git dir and a format", out)
	}
	// rev-parse prints the git dir relative to the working directory
	// (".git" from a repository root) unless it had to be absolute.
	gitDir := strings.TrimSpace(lines[0])
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(dir, gitDir)
	}
	g.gitDir = filepath.Clean(gitDir)
	g.format = strings.TrimSpace(lines[1])
	if _, err := newObjectHash(g.format); err != nil {
		return nil, fmt.Errorf("gitx: %w", err)
	}
	return g, nil
}

// indexPath is the private index: `<git-dir>/tuhdoo/index`, the
// daemon's runtime directory. Only ReadTree, UpdateIndex, and WriteTree
// ever name it, and only through indexEnv, so no other git subprocess
// can see it — and none of them can see the user's index.
func (g *CLI) indexPath() string {
	return filepath.Join(g.gitDir, "tuhdoo", "index")
}

// indexEnv selects the private index for one subprocess.
func (g *CLI) indexEnv() []string {
	return []string{"GIT_INDEX_FILE=" + g.indexPath()}
}

// zeroOID is the all-zero object name in the repository's format: what
// an `update-index --index-info` removal line carries in place of a
// blob (40 hex digits for SHA-1, 64 for SHA-256).
func (g *CLI) zeroOID() string {
	h, err := newObjectHash(g.format)
	if err != nil {
		panic("gitx: " + err.Error()) // validated by New
	}
	return strings.Repeat("0", h.Size()*2)
}

func (g *CLI) ReadTree(tree string) error {
	idx := g.indexPath()
	if err := os.MkdirAll(filepath.Dir(idx), 0o755); err != nil {
		return fmt.Errorf("gitx: read-tree: create private index dir: %w", err)
	}
	// A lock file here can only be a leftover of a git that died
	// mid-write: the daemon's flock (its caller) is the proof that no
	// other process is writing this index right now.
	if err := os.Remove(idx + ".lock"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("gitx: read-tree: clear stale index lock: %w", err)
	}
	// A single-tree read-tree replaces the index contents wholesale
	// (no merge, no worktree: nothing is checked out), whatever state
	// the file was in — including garbage or absent.
	if _, _, err := g.run(nil, g.indexEnv(), "read-tree", tree); err != nil {
		return fmt.Errorf("gitx: %w", err)
	}
	return nil
}

func (g *CLI) UpdateIndex(entries []TreeEntry) error {
	if err := validateTreePaths(entries); err != nil {
		return fmt.Errorf("gitx: update-index: %w", err)
	}
	if len(entries) == 0 {
		return nil
	}
	// --index-info takes ls-tree-shaped records for additions
	// ("<mode> blob <oid>\t<path>") and mode-0 records with the zero
	// object name for removals; -z makes the records NUL-terminated so
	// no path byte can be mistaken for a terminator.
	var input bytes.Buffer
	for _, e := range entries {
		if e.OID == "" {
			fmt.Fprintf(&input, "0 %s\t%s\x00", g.zeroOID(), e.Path)
			continue
		}
		fmt.Fprintf(&input, "100644 blob %s\t%s\x00", e.OID, e.Path)
	}
	if _, _, err := g.run(input.Bytes(), g.indexEnv(), "update-index", "-z", "--index-info"); err != nil {
		return fmt.Errorf("gitx: %w", err)
	}
	return nil
}

func (g *CLI) WriteTree() (string, error) {
	// git writes the empty tree for an index file that does not exist;
	// for a commit that would mean silently dropping every file the
	// branch holds. The index is seeded by ReadTree before any commit,
	// so its absence here means something removed it underneath us.
	if _, err := os.Stat(g.indexPath()); err != nil {
		return "", fmt.Errorf("gitx: write-tree: private index unusable (reseed with ReadTree): %w", err)
	}
	out, _, err := g.run(nil, g.indexEnv(), "write-tree")
	if err != nil {
		return "", fmt.Errorf("gitx: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// newObjectHash returns a fresh hash for a git object format name.
func newObjectHash(format string) (hash.Hash, error) {
	switch format {
	case "sha1":
		return sha1.New(), nil
	case "sha256":
		return sha256.New(), nil
	}
	return nil, fmt.Errorf("unsupported object format %q (want sha1 or sha256)", format)
}

func gitVersionOK(major, minor int) bool {
	return major > minGitMajor || (major == minGitMajor && minor >= minGitMinor)
}

// parseGitVersion extracts major.minor from `git version` output such
// as "git version 2.50.1 (Apple Git-155)".
func parseGitVersion(s string) (major, minor int, err error) {
	fields := strings.Fields(s)
	if len(fields) < 3 || fields[0] != "git" || fields[1] != "version" {
		return 0, 0, fmt.Errorf("unexpected `git version` output %q", s)
	}
	parts := strings.Split(fields[2], ".")
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("unexpected git version number %q", fields[2])
	}
	major, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("unexpected git version number %q", fields[2])
	}
	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("unexpected git version number %q", fields[2])
	}
	return major, minor, nil
}

// run executes git in the repo directory and returns stdout, stderr,
// and the error (already wrapped with the command name and stderr).
// LC_ALL=C pins the message language: some failures below are
// classified by matching git's error text.
func (g *CLI) run(stdin []byte, extraEnv []string, args ...string) (stdout []byte, stderr string, err error) {
	return g.runCtx(context.Background(), stdin, extraEnv, args...)
}

// runCtx is run with a context: git is killed when the context expires.
// WaitDelay matters for the timeout path — killing git can orphan a
// transport helper (ssh) still holding our stdout/stderr pipes, and
// without the delay cmd.Run would keep waiting on those pipes, undoing
// the bound. In the normal path git exits with its pipes closed, so the
// delay never engages.
func (g *CLI) runCtx(ctx context.Context, stdin []byte, extraEnv []string, args ...string) (stdout []byte, stderr string, err error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.dir
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	cmd.Env = append(cmd.Env, extraEnv...)
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.WaitDelay = 2 * time.Second
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	if err != nil {
		err = fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(errBuf.String()))
	}
	return outBuf.Bytes(), errBuf.String(), err
}

func (g *CLI) HashObject(data []byte) (string, error) {
	out, _, err := g.run(data, nil, "hash-object", "-w", "--stdin")
	if err != nil {
		return "", fmt.Errorf("gitx: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// BlobOID computes, in memory, the ID `git hash-object` would print for
// data: the object format's hash over the header "blob <len>\0" followed
// by the bytes. Nothing is written — see HashObject for that.
func (g *CLI) BlobOID(data []byte) string {
	return blobOID(g.format, data)
}

// blobOID is BlobOID for an explicit object format name. The format was
// validated by New, so an unknown name here is a programming error.
func blobOID(format string, data []byte) string {
	h, err := newObjectHash(format)
	if err != nil {
		panic("gitx: " + err.Error())
	}
	fmt.Fprintf(h, "blob %d\x00", len(data))
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

func (g *CLI) MkTree(entries []TreeEntry) (string, error) {
	if err := validateTreePaths(entries); err != nil {
		return "", fmt.Errorf("gitx: mktree: %w", err)
	}
	return g.mkTreeLevel(entries)
}

func validateTreePaths(entries []TreeEntry) error {
	seen := make(map[string]bool, len(entries))
	for _, e := range entries {
		if seen[e.Path] {
			return fmt.Errorf("duplicate path %q", e.Path)
		}
		seen[e.Path] = true
		if e.Path == "" || strings.HasPrefix(e.Path, "/") {
			return fmt.Errorf("invalid path %q", e.Path)
		}
		for _, seg := range strings.Split(e.Path, "/") {
			if seg == "" || seg == "." || seg == ".." {
				return fmt.Errorf("invalid path %q", e.Path)
			}
		}
	}
	return nil
}

// mkTreeLevel builds the tree for one directory level: blobs directly
// here, plus one recursive call per subdirectory. Entry order in the
// mktree input does not matter — mktree sorts entries into canonical
// tree order itself. Mode is always 100644: the data branch holds plain
// files only, never executables or symlinks.
func (g *CLI) mkTreeLevel(entries []TreeEntry) (string, error) {
	var input bytes.Buffer
	blobNames := make(map[string]bool)
	var subNames []string // deterministic iteration; a plain map would randomize subprocess input
	subs := make(map[string][]TreeEntry)
	for _, e := range entries {
		head, rest, nested := strings.Cut(e.Path, "/")
		if !nested {
			blobNames[head] = true
			fmt.Fprintf(&input, "100644 blob %s\t%s\x00", e.OID, e.Path)
			continue
		}
		if subs[head] == nil {
			subNames = append(subNames, head)
		}
		subs[head] = append(subs[head], TreeEntry{Path: rest, OID: e.OID})
	}
	for _, name := range subNames {
		if blobNames[name] {
			return "", fmt.Errorf("gitx: mktree: %q is both a file and a directory", name)
		}
		oid, err := g.mkTreeLevel(subs[name])
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&input, "040000 tree %s\t%s\x00", oid, name)
	}
	out, _, err := g.run(input.Bytes(), nil, "mktree", "-z")
	if err != nil {
		return "", fmt.Errorf("gitx: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (g *CLI) CommitTree(treeOID string, parentOIDs []string, ident Identity, message string) (string, error) {
	// App-level merges write two-parent commits; nothing needs more.
	if len(parentOIDs) > 2 {
		return "", fmt.Errorf("gitx: commit-tree: at most 2 parents are supported, got %d", len(parentOIDs))
	}
	if ident.Name == "" || ident.Email == "" {
		return "", errors.New("gitx: commit-tree: identity name and email are required")
	}
	// --no-gpg-sign: commit-tree honors the user's commit.gpgsign
	// config, which could block a daemon on a passphrase prompt.
	args := []string{"commit-tree", "--no-gpg-sign", treeOID}
	for _, p := range parentOIDs {
		args = append(args, "-p", p)
	}
	env := []string{
		"GIT_AUTHOR_NAME=" + ident.Name,
		"GIT_AUTHOR_EMAIL=" + ident.Email,
		"GIT_COMMITTER_NAME=" + ident.Name,
		"GIT_COMMITTER_EMAIL=" + ident.Email,
	}
	// The message goes in on stdin so git stores it byte-for-byte.
	out, _, err := g.run([]byte(message), env, args...)
	if err != nil {
		return "", fmt.Errorf("gitx: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (g *CLI) UpdateRef(ref, newOID, oldOID string) error {
	// `update-ref <ref> <new> <old>` is git's native compare-and-swap;
	// an empty <old> means "must not exist yet".
	_, stderr, err := g.run(nil, nil, "update-ref", ref, newOID, oldOID)
	if err != nil {
		if isCASFailure(stderr) {
			return fmt.Errorf("gitx: update-ref %s: %w: %s", ref, ErrRefCASFailed, strings.TrimSpace(stderr))
		}
		return fmt.Errorf("gitx: %w", err)
	}
	return nil
}

// isCASFailure recognizes the three messages git prints when the
// old-value check of update-ref fails: value mismatch, ref exists but
// was expected absent, ref missing but was expected present.
func isCASFailure(stderr string) bool {
	return strings.Contains(stderr, "but expected") ||
		strings.Contains(stderr, "reference already exists") ||
		strings.Contains(stderr, "unable to resolve reference")
}

func (g *CLI) ReadRef(ref string) (string, error) {
	// With --quiet, rev-parse --verify exits with code 1 and prints
	// nothing when the ref is missing, so no message matching is needed
	// to classify "not found".
	out, _, err := g.run(nil, nil, "rev-parse", "--verify", "--quiet", "--end-of-options", ref)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return "", fmt.Errorf("gitx: read ref %q: %w", ref, ErrRefNotFound)
		}
		return "", fmt.Errorf("gitx: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (g *CLI) CatFile(oid string) ([]byte, error) {
	blobs, err := g.CatFiles([]string{oid})
	if err != nil {
		return nil, err
	}
	return blobs[oid], nil
}

// CatFiles reads every requested blob through one `git cat-file --batch`
// process: the deduplicated OIDs go in on stdin, one per line, and the
// records come back on stdout as "<oid> <type> <size>\n<bytes>\n" — or
// "<oid> missing\n" for an object the repository lacks, which is an
// error here, never a skipped blob. The whole output is collected
// first and then parsed by parseCatFileBatch; git exits on its own
// once stdin is consumed, so there is no pipe to manage.
func (g *CLI) CatFiles(oids []string) (map[string][]byte, error) {
	unique := dedupe(oids)
	if len(unique) == 0 {
		return map[string][]byte{}, nil
	}
	var input bytes.Buffer
	for _, oid := range unique {
		input.WriteString(oid)
		input.WriteByte('\n')
	}
	out, _, err := g.run(input.Bytes(), nil, "cat-file", "--batch")
	if err != nil {
		return nil, fmt.Errorf("gitx: %w", err)
	}
	blobs, err := parseCatFileBatch(out, unique)
	if err != nil {
		return nil, fmt.Errorf("gitx: cat-file --batch: %w", err)
	}
	return blobs, nil
}

// dedupe returns oids without repeats, first occurrence order kept, so
// the batch input has one line per object and the output one record.
func dedupe(oids []string) []string {
	seen := make(map[string]bool, len(oids))
	var out []string
	for _, oid := range oids {
		if seen[oid] {
			continue
		}
		seen[oid] = true
		out = append(out, oid)
	}
	return out
}

// parseCatFileBatch decodes `cat-file --batch` output for the objects
// requested, in request order — git answers in input order, one record
// per line of input. Bodies are read by their declared size (io.ReadFull)
// rather than by line, so a blob may hold newlines, NULs, or bytes that
// look like a header; an empty blob is a zero-byte body. Every record's
// object name must match the request it answers: a drift between the
// two would attribute bytes to the wrong OID, which is worse than
// failing.
func parseCatFileBatch(out []byte, oids []string) (map[string][]byte, error) {
	r := bufio.NewReader(bytes.NewReader(out))
	blobs := make(map[string][]byte, len(oids))
	for _, want := range oids {
		header, err := r.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("no record for %s: output ended early", want)
		}
		fields := strings.Fields(header)
		if len(fields) == 2 && fields[1] == "missing" {
			return nil, fmt.Errorf("object %s missing from the repository", fields[0])
		}
		if len(fields) != 3 {
			return nil, fmt.Errorf("cannot parse record header %q (expected it for %s)", strings.TrimSuffix(header, "\n"), want)
		}
		got, typ := fields[0], fields[1]
		if got != want {
			return nil, fmt.Errorf("record for %s arrived where %s was expected", got, want)
		}
		if typ != "blob" {
			return nil, fmt.Errorf("object %s is a %s, not a blob", got, typ)
		}
		size, err := strconv.Atoi(fields[2])
		if err != nil || size < 0 {
			return nil, fmt.Errorf("record for %s declares size %q", got, fields[2])
		}
		body := make([]byte, size)
		if _, err := io.ReadFull(r, body); err != nil {
			return nil, fmt.Errorf("blob %s: declared %d bytes, output ended early", got, size)
		}
		// Each body is followed by exactly one newline that is not part
		// of the blob.
		if nl, err := r.ReadByte(); err != nil || nl != '\n' {
			return nil, fmt.Errorf("blob %s: record not newline-terminated after %d bytes", got, size)
		}
		blobs[got] = body
	}
	if rest, _ := r.Peek(1); len(rest) != 0 {
		return nil, fmt.Errorf("unexpected output after the last of %d records", len(oids))
	}
	return blobs, nil
}

func (g *CLI) LsTree(rev string) ([]TreeEntry, error) {
	out, _, err := g.run(nil, nil, "ls-tree", "-r", "-z", "--full-tree", rev)
	if err != nil {
		return nil, fmt.Errorf("gitx: %w", err)
	}
	var entries []TreeEntry
	// One record per entry: "<mode> <type> <oid>\t<path>", NUL-ended.
	for _, rec := range strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00") {
		if rec == "" { // an empty tree lists nothing
			continue
		}
		meta, path, ok := strings.Cut(rec, "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 3 {
			return nil, fmt.Errorf("gitx: ls-tree: cannot parse entry %q", rec)
		}
		// The data branch holds regular blobs only; anything else means
		// the tree is not ours — fail, don't skip. The type field alone
		// cannot enforce that: ls-tree types symlink (120000) entries as
		// "blob" too, so the mode is the check that catches them.
		if fields[1] != "blob" {
			return nil, fmt.Errorf("gitx: ls-tree: unexpected %s object at %q", fields[1], path)
		}
		if fields[0] != "100644" && fields[0] != "100755" {
			return nil, fmt.Errorf("gitx: ls-tree: unexpected mode %s at %q", fields[0], path)
		}
		entries = append(entries, TreeEntry{Path: path, OID: fields[2]})
	}
	return entries, nil
}

func (g *CLI) Fetch(remote, refspec string) error {
	return g.fetch(context.Background(), remote, refspec)
}

func (g *CLI) FetchTimeout(remote, refspec string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return g.fetch(ctx, remote, refspec)
}

func (g *CLI) fetch(ctx context.Context, remote, refspec string) error {
	_, stderr, err := g.runCtx(ctx, nil, nil, "fetch", "--quiet", remote, refspec)
	if err != nil {
		// The deadline check comes first: a killed git prints nothing
		// classifiable, and callers deserve a message naming the bound.
		if ctx.Err() != nil {
			return fmt.Errorf("gitx: fetch %s %s: timed out: %w", remote, refspec, ctx.Err())
		}
		if strings.Contains(stderr, "couldn't find remote ref") {
			return fmt.Errorf("gitx: fetch %s %s: %w", remote, refspec, ErrRemoteRefMissing)
		}
		return fmt.Errorf("gitx: %w", err)
	}
	return nil
}

// IsAncestor reports whether commit a is an ancestor of (or equal to)
// commit b.
func (g *CLI) IsAncestor(a, b string) (bool, error) {
	_, _, err := g.run(nil, nil, "merge-base", "--is-ancestor", a, b)
	if err == nil {
		return true, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("gitx: %w", err)
}

func (g *CLI) Push(remote, refspec string) error {
	// --porcelain gives a stable per-ref status line on stdout, which
	// is what classifies a rejection. --no-follow-tags/--no-signed pin
	// behavior the user's config could otherwise change.
	stdout, stderr, err := g.run(nil, nil, "push", "--porcelain", "--no-follow-tags", "--no-signed", remote, refspec)
	if err != nil {
		if pushRejectionIsContention(string(stdout), string(stderr)) {
			return fmt.Errorf("gitx: push %s %s: %w", remote, refspec, ErrNonFastForward)
		}
		return fmt.Errorf("gitx: %w", err)
	}
	return nil
}

// pushRejectionIsContention classifies a failed push's output: true means
// another writer got to the ref first and the caller should fetch, merge,
// and retry (ErrNonFastForward); false means some other failure.
func pushRejectionIsContention(stdout, stderr string) bool {
	// The per-ref --porcelain status on stdout: "non-fast-forward" for a
	// stale local history; "fetch first" is git's wording for the same
	// situation when the remote ref is entirely unknown locally.
	if strings.Contains(stdout, "non-fast-forward") || strings.Contains(stdout, "fetch first") {
		return true
	}
	// Two pushes landing at the same instant can lose the remote's own
	// ref-update race instead: the remote relays "cannot lock ref '…':
	// is at X but expected Y" on stderr. Contention all the same — not
	// counting it undercounts Status.Collisions (T8) and skips the retry
	// loop for a cycle (observed in the collision-harness storm).
	return strings.Contains(stderr, "cannot lock ref")
}

func (g *CLI) RemoteURL(remote string) (string, error) {
	out, stderr, err := g.run(nil, nil, "remote", "get-url", remote)
	if err != nil {
		if strings.Contains(stderr, "No such remote") {
			return "", fmt.Errorf("gitx: remote %q: %w", remote, ErrNoRemote)
		}
		return "", fmt.Errorf("gitx: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
