package gitx

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Minimum supported git version (design doc 002, T2).
const minGitMajor, minGitMinor = 2, 40

// CLI implements Git by running the real git binary as a subprocess
// against one repository directory. Fetch and push therefore inherit
// the user's full auth setup (SSH agents, credential helpers) for free.
type CLI struct {
	dir string
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
	if _, _, err := g.run(nil, nil, "rev-parse", "--git-dir"); err != nil {
		return nil, fmt.Errorf("gitx: %s is not a git repository: %w", dir, err)
	}
	return g, nil
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
	cmd := exec.Command("git", args...)
	cmd.Dir = g.dir
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	cmd.Env = append(cmd.Env, extraEnv...)
	cmd.Stdin = bytes.NewReader(stdin)
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
	out, _, err := g.run(nil, nil, "cat-file", "blob", oid)
	if err != nil {
		return nil, fmt.Errorf("gitx: %w", err)
	}
	return out, nil
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
		// The data branch holds blobs only; anything else (submodule,
		// symlink) means the tree is not ours — fail, don't skip.
		if fields[1] != "blob" {
			return nil, fmt.Errorf("gitx: ls-tree: unexpected %s object at %q", fields[1], path)
		}
		entries = append(entries, TreeEntry{Path: path, OID: fields[2]})
	}
	return entries, nil
}

func (g *CLI) Fetch(remote, refspec string) error {
	_, _, err := g.run(nil, nil, "fetch", "--quiet", remote, refspec)
	if err != nil {
		return fmt.Errorf("gitx: %w", err)
	}
	return nil
}

func (g *CLI) Push(remote, refspec string) error {
	// --porcelain gives a stable per-ref status line on stdout, which
	// is what classifies a rejection. --no-follow-tags/--no-signed pin
	// behavior the user's config could otherwise change.
	stdout, _, err := g.run(nil, nil, "push", "--porcelain", "--no-follow-tags", "--no-signed", remote, refspec)
	if err != nil {
		// "fetch first" is git's wording for the same situation when
		// the remote ref is entirely unknown locally.
		s := string(stdout)
		if strings.Contains(s, "non-fast-forward") || strings.Contains(s, "fetch first") {
			return fmt.Errorf("gitx: push %s %s: %w", remote, refspec, ErrNonFastForward)
		}
		return fmt.Errorf("gitx: %w", err)
	}
	return nil
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
