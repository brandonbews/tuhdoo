package main

// Repository discovery for the CLI. The few git queries here shell out
// to git directly instead of going through internal/gitx: gitx's
// interface is deliberately scoped to data-branch object access (T2)
// and has no toplevel/remote-enumeration calls, and the CLI performs no
// object writes of its own — branch creation happens on the daemon's
// startup path.

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/brandonbews/tuhdoo/internal/store"
)

// repo is the surrounding git repository as the CLI sees it.
type repo struct {
	root   string // worktree root (what the daemon takes as its root)
	gitDir string // resolved .git dir; daemon.json and friends live under it
}

func openRepo() (*repo, error) {
	root, err := repoRoot()
	if err != nil {
		return nil, err
	}
	gd, err := gitDirOf(root)
	if err != nil {
		return nil, err
	}
	return &repo{root: root, gitDir: gd}, nil
}

// repoRoot resolves the worktree root of the repository containing the
// current directory.
func repoRoot() (string, error) {
	out, err := gitOutput("", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("not a git repository — run tuhdoo inside one (%v)", err)
	}
	return strings.TrimSpace(out), nil
}

// runtimeDir is <git-dir>/tuhdoo: daemon.json, daemon.sock, daemon.log.
func (r *repo) runtimeDir() string {
	return filepath.Join(r.gitDir, "tuhdoo")
}

// headShort returns the data branch head OID in short form.
func (r *repo) headShort() (string, error) {
	out, err := gitOutput(r.root, "rev-parse", "--short", store.DefaultRef)
	if err != nil {
		return "", fmt.Errorf("read %s: %v", store.DefaultRef, err)
	}
	return strings.TrimSpace(out), nil
}

// branchName is the short name of the data branch.
func branchName() string {
	return strings.TrimPrefix(store.DefaultRef, "refs/heads/")
}

// gitDirOf locates the .git directory for root, handling the
// linked-worktree form where ".git" is a file holding "gitdir: <path>".
// Mirrors the daemon's own resolution so both sides agree on where
// daemon.json lives.
func gitDirOf(root string) (string, error) {
	p := filepath.Join(root, ".git")
	fi, err := os.Stat(p)
	if err != nil {
		return "", fmt.Errorf("%s is not a git repository: %w", root, err)
	}
	if fi.IsDir() {
		return p, nil
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", p, err)
	}
	target, ok := strings.CutPrefix(strings.TrimSpace(string(b)), "gitdir:")
	if !ok {
		return "", fmt.Errorf("%s is neither a directory nor a gitdir pointer", p)
	}
	target = strings.TrimSpace(target)
	if !filepath.IsAbs(target) {
		target = filepath.Join(root, target)
	}
	return filepath.Clean(target), nil
}

// gitEmailLocalPart derives the human half of a D7 principal from git
// identity: the local part of user.email (brandonbews@gmail.com →
// brandonbews). This is the one documented auto-derivation rule,
// shared by every surface that acts as a human without an explicit
// --as (the mcp shim, the TUI).
func gitEmailLocalPart(dir string) (string, error) {
	out, err := gitOutput(dir, "config", "user.email")
	if err != nil {
		return "", fmt.Errorf("git user.email is not set (%v)", err)
	}
	local, _, _ := strings.Cut(strings.TrimSpace(out), "@")
	if local == "" {
		return "", errors.New("git user.email has an empty local part")
	}
	return local, nil
}

// gitOutput runs one read-only git query. An empty dir inherits the
// current directory.
func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", args[0], msg)
	}
	return out.String(), nil
}
