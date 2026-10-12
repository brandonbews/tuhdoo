package daemon

// Where the data branch syncs. By default that is origin, the code
// repo's own remote (002 T2). A repo can instead name its ledger's home
// — typically a private `<repo>-internal` repo beside a public code
// repo (decided by Brandon, 2026-10-12) — with a committed `.tuhdoo`
// file at the worktree root, in git-config format like .gitmodules:
//
//	[ledger]
//		remote = git@github.com:owner/repo-internal.git
//
// The value is anything git fetch and git push accept: a URL, a local
// path, or a remote name. A named target is pinned (syncer.Options):
// never origin, never remoteless. The daemon resolves the target once,
// at startup; changing it means restarting the daemon.
//
// resolveTarget is the pure rule, table-tested; readTargetInputs and
// resolveTargetAt are the thin I/O around it.

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/brandonbews/tuhdoo/internal/gitx"
)

const (
	// targetFile is the pointer file, at the worktree root, and
	// targetFileKey the one key read from it.
	targetFile    = ".tuhdoo"
	targetFileKey = "ledger.remote"
	// targetConfigKey is the clone's memory of the target, in local
	// config (shared by every linked worktree): written from the file
	// at startup, read when the file is absent — a worktree on an old
	// branch or tag that predates the file. Also a per-clone setting
	// for anyone who sets it by hand.
	targetConfigKey = "tuhdoo.remote"
)

// Sources of a pinned target, as status surfaces show them.
const (
	sourceFile        = targetFile
	sourceLocalConfig = "git config " + targetConfigKey
)

// defaultRemote is the unpinned target: today's behaviour exactly,
// including local-only mode when no origin is configured (T2).
const defaultRemote = "origin"

// ledgerTarget is a resolved sync target. source is sourceFile or
// sourceLocalConfig for a pinned target, "" for the origin default.
type ledgerTarget struct {
	remote string
	source string
}

func (t ledgerTarget) pinned() bool { return t.source != "" }

// targetInputs is everything resolveTarget decides from, gathered by
// readTargetInputs. Pure data: the rule never touches git or disk.
type targetInputs struct {
	filePath  string // where .tuhdoo is, for messages
	fileFound bool   // .tuhdoo exists at the worktree root
	fileErr   error  // it exists, but could not be read or parsed
	fileValue string // its ledger.remote
	fileSet   bool   // ledger.remote is present in it

	localValue string // tuhdoo.remote in local config
	localSet   bool   // tuhdoo.remote is set
	localErr   error  // reading local config failed
}

// resolveTarget picks the sync target, in order:
//
//   - .tuhdoo present → its ledger.remote, pinned. record is true when
//     local config does not already hold that value, so the caller
//     writes it there for later starts on branches without the file.
//     A .tuhdoo that cannot be read, or that lacks a usable
//     ledger.remote, is an error: the file says the ledger lives
//     elsewhere, so falling back to anything would be a guess.
//   - no .tuhdoo, tuhdoo.remote set → that value, pinned.
//   - neither → origin, unpinned.
//
// A pinned value must be non-empty and must not start with "-": git
// would read that as an option, and the file is repository content.
func resolveTarget(in targetInputs) (target ledgerTarget, record bool, err error) {
	if in.fileFound {
		if in.fileErr != nil {
			return ledgerTarget{}, false, fmt.Errorf("%s names where this repo's ledger lives, but it cannot be read or parsed (%v); fix it and run tuhdoo again — tuhdoo will not guess the ledger's home", in.filePath, in.fileErr)
		}
		if !in.fileSet {
			return ledgerTarget{}, false, fmt.Errorf("%s does not set %s; add it (a URL, path, or remote name) and run tuhdoo again — tuhdoo will not guess the ledger's home", in.filePath, targetFileKey)
		}
		value, err := checkTargetValue(in.fileValue)
		if err != nil {
			return ledgerTarget{}, false, fmt.Errorf("%s: %s %v", in.filePath, targetFileKey, err)
		}
		record = !in.localSet || strings.TrimSpace(in.localValue) != value
		return ledgerTarget{remote: value, source: sourceFile}, record, nil
	}
	if in.localErr != nil {
		return ledgerTarget{}, false, fmt.Errorf("reading git config %s: %v", targetConfigKey, in.localErr)
	}
	if in.localSet {
		value, err := checkTargetValue(in.localValue)
		if err != nil {
			return ledgerTarget{}, false, fmt.Errorf("git config %s %v; set it or unset it", targetConfigKey, err)
		}
		return ledgerTarget{remote: value, source: sourceLocalConfig}, false, nil
	}
	return ledgerTarget{remote: defaultRemote}, false, nil
}

// checkTargetValue trims a pinned target and refuses the two values
// git cannot be handed safely: nothing, and an option.
func checkTargetValue(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", errors.New("is set but empty")
	}
	if strings.HasPrefix(value, "-") {
		return "", fmt.Errorf("is %q, which git would read as an option; refusing it", value)
	}
	return value, nil
}

// readTargetInputs gathers resolveTarget's inputs: whether .tuhdoo
// exists at root and what it says, and the local config value. The
// file's bytes are read first so an unreadable file (or a directory)
// is reported as such — git config itself reads one as "key not set".
func readTargetInputs(g gitx.Git, root string) targetInputs {
	in := targetInputs{filePath: filepath.Join(root, targetFile)}
	if _, err := os.ReadFile(in.filePath); err == nil {
		in.fileFound = true
		in.fileValue, in.fileSet, in.fileErr = g.ReadConfig(in.filePath, targetFileKey)
	} else if !errors.Is(err, os.ErrNotExist) {
		in.fileFound = true
		in.fileErr = err
	}
	in.localValue, in.localSet, in.localErr = g.ReadConfig("", targetConfigKey)
	return in
}

// resolveTargetAt is daemon startup's one call: read, resolve, and —
// when the file pins a value local config does not hold yet — record
// it, once. A failed write is logged, not fatal: the file still pins
// this run, and the next start tries again.
func resolveTargetAt(g gitx.Git, root string, logger *log.Logger) (ledgerTarget, error) {
	target, record, err := resolveTarget(readTargetInputs(g, root))
	if err != nil {
		return ledgerTarget{}, fmt.Errorf("daemon: ledger target: %w", err)
	}
	if record {
		if err := g.SetLocalConfig(targetConfigKey, target.remote); err != nil {
			logger.Printf("daemon: could not record the ledger target in git config %s (%v); %s still pins this run", targetConfigKey, err, targetFile)
		}
	}
	if target.pinned() {
		logger.Printf("daemon: ledger target %q (from %s); changing it takes a daemon restart", target.remote, target.source)
	}
	return target, nil
}
