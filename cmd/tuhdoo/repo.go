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

	"github.com/brandonbews/tuhdoo/internal/daemon"
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

// principalConfigKey is the git config key that overrides the derived
// human principal (T4, revised 2026-07-30). Set once per clone with
// `git config tuhdoo.principal <name>` (or per machine with --global;
// ordinary git config precedence applies), it replaces the user.email
// derivation everywhere derivation is used — the mcp shim and the
// TUI's steer mode. --as still beats it: callers only derive when no
// explicit principal was given.
const principalConfigKey = "tuhdoo.principal"

// gitEmailLocalPart derives the human half of a D7 principal from git
// identity. The name records the default rule — the local part of
// user.email (brandonbews@gmail.com → brandonbews) — but a
// tuhdoo.principal config entry, when set, overrides it (handy when
// user.email is a GitHub noreply address and the local part is
// `4099114+brandonbews`). This is the one documented auto-derivation
// path, shared by every surface that acts as a human without an
// explicit --as (the mcp shim, the TUI).
func gitEmailLocalPart(dir string) (string, error) {
	if p, ok, err := principalOverride(dir); err != nil {
		return "", err
	} else if ok {
		return p, nil
	}
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

// principalOverride reads the tuhdoo.principal config entry. ok
// reports whether the key is set at all; an unset key is the normal
// no-override case. A key that is set but invalid is a loud error,
// never a silent fall-through to email derivation — a principal the
// human wrote down and got wrong must not quietly become a different
// one. The override is the human half only, so it must be a root
// principal (no "/"); agents get their half minted by the daemon.
func principalOverride(dir string) (string, bool, error) {
	out, err := gitOutput(dir, "config", "--get", principalConfigKey)
	if err != nil {
		// `git config --get` exits non-zero when the key is unset.
		// A genuinely broken git surfaces on the user.email query
		// that follows, with its own message.
		return "", false, nil
	}
	p := strings.TrimSpace(out)
	if p == "" {
		return "", false, fmt.Errorf("git config %s is set but empty — set a principal or unset it", principalConfigKey)
	}
	if strings.Contains(p, "/") {
		return "", false, fmt.Errorf("git config %s must be a root (human) principal, not an agent: got %q", principalConfigKey, p)
	}
	if err := daemon.ValidateActor(p); err != nil {
		return "", false, fmt.Errorf("git config %s: %v", principalConfigKey, err)
	}
	return p, true, nil
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
