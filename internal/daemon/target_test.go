package daemon

// Tests for the ledger target (target.go): the pure resolution rule,
// table-driven, and the daemon end to end with a pinned target — two
// bare repos, origin and the ledger's home, where origin must never
// receive the data branch whatever happens to the target.

import (
	"bytes"
	"errors"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brandonbews/tuhdoo/internal/gitx"
	"github.com/brandonbews/tuhdoo/internal/store"
)

func TestResolveTarget(t *testing.T) {
	const file = "/repo/.tuhdoo"
	const url = "git@example.com:owner/repo-internal.git"
	origin := ledgerTarget{remote: "origin"}
	fromFile := ledgerTarget{remote: url, source: sourceFile}

	for _, tc := range []struct {
		name       string
		in         targetInputs
		want       ledgerTarget
		wantRecord bool
		wantErr    string // substring; "" means no error
	}{
		{
			name: "neither file nor local config: origin, unpinned",
			in:   targetInputs{filePath: file},
			want: origin,
		},
		{
			name: "local config alone pins",
			in:   targetInputs{filePath: file, localValue: "/srv/ledger.git", localSet: true},
			want: ledgerTarget{remote: "/srv/ledger.git", source: sourceLocalConfig},
		},
		{
			name:       "file pins and is recorded",
			in:         targetInputs{filePath: file, fileFound: true, fileValue: url, fileSet: true},
			want:       fromFile,
			wantRecord: true,
		},
		{
			name: "file pins, already recorded",
			in: targetInputs{filePath: file, fileFound: true, fileValue: url, fileSet: true,
				localValue: url, localSet: true},
			want: fromFile,
		},
		{
			name: "file overrides a stale local value and re-records",
			in: targetInputs{filePath: file, fileFound: true, fileValue: url, fileSet: true,
				localValue: "/old/ledger.git", localSet: true},
			want:       fromFile,
			wantRecord: true,
		},
		{
			name:       "file value is trimmed",
			in:         targetInputs{filePath: file, fileFound: true, fileValue: "  " + url + "\t", fileSet: true},
			want:       fromFile,
			wantRecord: true,
		},
		{
			name:       "file pins even when local config cannot be read",
			in:         targetInputs{filePath: file, fileFound: true, fileValue: url, fileSet: true, localErr: errors.New("boom")},
			want:       fromFile,
			wantRecord: true,
		},
		{
			name:    "file malformed or unreadable: error, no guess",
			in:      targetInputs{filePath: file, fileFound: true, fileErr: errors.New("bad config line 1")},
			wantErr: "bad config line 1",
		},
		{
			name: "file without ledger.remote: error, even with local config set",
			in: targetInputs{filePath: file, fileFound: true,
				localValue: "/srv/ledger.git", localSet: true},
			wantErr: "does not set ledger.remote",
		},
		{
			name:    "file with an empty ledger.remote",
			in:      targetInputs{filePath: file, fileFound: true, fileValue: "  ", fileSet: true},
			wantErr: "empty",
		},
		{
			name:    "file value that git would read as an option",
			in:      targetInputs{filePath: file, fileFound: true, fileValue: "--upload-pack=touch /tmp/x", fileSet: true},
			wantErr: "option",
		},
		{
			name:    "local config set but empty",
			in:      targetInputs{filePath: file, localValue: "", localSet: true},
			wantErr: "empty",
		},
		{
			name:    "local config value that git would read as an option",
			in:      targetInputs{filePath: file, localValue: "-x", localSet: true},
			wantErr: "option",
		},
		{
			name:    "local config unreadable, no file: error, never origin",
			in:      targetInputs{filePath: file, localErr: errors.New("boom")},
			wantErr: "boom",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, record, err := resolveTarget(tc.in)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want one containing %q", err, tc.wantErr)
				}
				if got != (ledgerTarget{}) || record {
					t.Fatalf("an error must come with no target: got %+v, record %v", got, record)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want || record != tc.wantRecord {
				t.Fatalf("got %+v record %v, want %+v record %v", got, record, tc.want, tc.wantRecord)
			}
			if got.pinned() != (got.source != "") {
				t.Fatalf("pinned() disagrees with source for %+v", got)
			}
		})
	}
}

// ---- end to end ----

// pinnedRepo builds base/origin.git (reachable, bare) and a working
// repo at base/clone whose origin it is. It returns the clone's root
// and origin's path; the caller names the ledger's home.
func pinnedRepo(t *testing.T) (root, origin string) {
	t.Helper()
	setGitEnv(t)
	base := shortTempDir(t)
	origin = filepath.Join(base, "origin.git")
	runGit(t, base, "init", "--quiet", "--bare", "-b", "main", origin)
	root = filepath.Join(base, "clone")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "init", "--quiet", "-b", "main")
	runGit(t, root, "remote", "add", "origin", origin)
	return root, origin
}

// newLedgerBare makes an empty bare repo beside root to be the
// ledger's private home.
func newLedgerBare(t *testing.T, root string) string {
	t.Helper()
	ledger := filepath.Join(filepath.Dir(root), "ledger.git")
	runGit(t, root, "init", "--quiet", "--bare", "-b", "main", ledger)
	return ledger
}

// writeTuhdoo commits nothing — the daemon reads the worktree — but
// writes the pointer file exactly as the docs show it.
func writeTuhdoo(t *testing.T, root, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, targetFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func pointerFile(remote string) string {
	return "# This repo plans its work with tuhdoo. The ledger lives in a private repo.\n" +
		"[ledger]\n\tremote = " + remote + "\n"
}

// carriesDataBranch reports whether the repository at dir has the data
// branch — a missing ref is an answer here, not a test failure.
func carriesDataBranch(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", "--quiet", store.DefaultRef)
	cmd.Dir = dir
	return cmd.Run() == nil
}

// pinnedLifecycle drives writes, a claim, a sync cycle, and
// confirm_claim through a daemon whose ledger is pinned to ledger, and
// asserts every one of them landed there and none of them on origin —
// before and after the shutdown's final sync.
func pinnedLifecycle(t *testing.T, root, origin, ledger string) *Daemon {
	t.Helper()
	d, c := startDaemonAt(t, root, gateOpts())
	const actor = "brandon/impl-1"
	task := createOne(t, c, "brandon", map[string]any{"title": "private plans"})
	if _, oe := d.opClaimTask(actor, task); oe != nil {
		t.Fatalf("claim: %v", oe)
	}
	mustCycle(t, d)
	if !carriesDataBranch(ledger) {
		t.Fatal("the sync cycle did not publish the data branch to the pinned target")
	}
	if got := runGit(t, ledger, "show", store.DefaultRef+":backlog.md"); !strings.Contains(got, "private plans") {
		t.Fatalf("the target's backlog.md lacks the task:\n%s", got)
	}

	res, oe := d.opConfirmClaim(actor, task)
	if oe != nil || !res.Confirmed {
		t.Fatalf("confirm = %+v, %v; want confirmed through the pinned target", res, oe)
	}
	if log := runGit(t, ledger, "log", "--format=%s", store.DefaultRef); !strings.Contains(log, "tuhdoo: confirm claim") {
		t.Fatalf("the confirmation was not pushed to the pinned target; its log:\n%s", log)
	}

	if carriesDataBranch(origin) {
		t.Fatal("origin received the data branch")
	}
	d.Shutdown("test: final sync")
	if carriesDataBranch(origin) {
		t.Fatal("origin received the data branch on the final sync")
	}
	return d
}

// TestPinnedFileLedgerLivesOnTargetNeverOrigin (acceptance 2): origin is
// bare repo A, .tuhdoo names bare repo B. Writes, the claim, the sync
// cycle, and confirm_claim all land on B; A never receives the data
// branch. The file overrides a stale local value, records itself in
// local config, and status names the target and its source.
func TestPinnedFileLedgerLivesOnTargetNeverOrigin(t *testing.T) {
	root, origin := pinnedRepo(t)
	ledger := newLedgerBare(t, root)
	writeTuhdoo(t, root, pointerFile(ledger))
	runGit(t, root, "config", "tuhdoo.remote", filepath.Join(filepath.Dir(root), "stale.git"))

	d, c := startDaemonAt(t, root, gateOpts())
	sync := snapshotNow(t, c).Sync
	if sync.Remote != ledger || sync.RemoteSource != ".tuhdoo" {
		t.Fatalf("status sync = %+v, want remote %s from .tuhdoo", sync, ledger)
	}
	if got := strings.TrimSpace(runGit(t, root, "config", "--local", "--get", "tuhdoo.remote")); got != ledger {
		t.Fatalf("local config tuhdoo.remote = %q, want the file's %q recorded over the stale value", got, ledger)
	}
	d.Shutdown("test: restart onto the same repo")

	pinnedLifecycle(t, root, origin, ledger)
}

// TestPinnedLocalConfigWithoutFile (acceptance 3, second bullet): no
// .tuhdoo — a worktree on a branch that predates it — but local config
// remembers tuhdoo.remote. That target is used, never origin.
func TestPinnedLocalConfigWithoutFile(t *testing.T) {
	root, origin := pinnedRepo(t)
	ledger := newLedgerBare(t, root)
	runGit(t, root, "config", "tuhdoo.remote", ledger)

	d, c := startDaemonAt(t, root, gateOpts())
	if sync := snapshotNow(t, c).Sync; sync.Remote != ledger || sync.RemoteSource != "git config tuhdoo.remote" {
		t.Fatalf("status sync = %+v, want remote %s from git config tuhdoo.remote", sync, ledger)
	}
	d.Shutdown("test: restart onto the same repo")

	pinnedLifecycle(t, root, origin, ledger)
}

// TestPinnedTargetUnreachableRefusesAndNeverFallsBack (acceptance 3,
// first bullet): the pinned target is a path that does not exist, or a
// remote name nothing configures. The sync cycle errors (status says
// so, naming the target), confirm_claim refuses with nothing written,
// and origin — reachable throughout — never receives the data branch,
// not even from the shutdown's final sync.
func TestPinnedTargetUnreachableRefusesAndNeverFallsBack(t *testing.T) {
	for _, tc := range []struct {
		name   string
		target func(root string) string
	}{
		{"missing path", func(root string) string { return filepath.Join(filepath.Dir(root), "no-such-ledger.git") }},
		{"unconfigured remote name", func(string) string { return "ledger" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, origin := pinnedRepo(t)
			target := tc.target(root)
			writeTuhdoo(t, root, pointerFile(target))
			d, c := startDaemonAt(t, root, gateOpts())

			const actor = "brandon/impl-1"
			task := createOne(t, c, "brandon", map[string]any{"title": "stranded work"})
			if _, oe := d.opClaimTask(actor, task); oe != nil {
				t.Fatalf("claim: %v", oe)
			}
			if err := d.sync.Cycle(); err == nil || errors.Is(err, gitx.ErrNoRemote) {
				t.Fatalf("Cycle = %v, want a fetch error — never the remoteless state", err)
			}
			// The background loop records the same failure in status:
			// its first cycle ran at startup, and the claim poked another.
			sync := snapshotNow(t, c).Sync
			for deadline := time.Now().Add(10 * time.Second); sync.Mode != "error" && time.Now().Before(deadline); {
				time.Sleep(20 * time.Millisecond)
				sync = snapshotNow(t, c).Sync
			}
			if sync.Mode != "error" || sync.LastError == "" || sync.Remote != target || sync.RemoteSource != ".tuhdoo" {
				t.Fatalf("status sync = %+v, want an error naming %q from .tuhdoo", sync, target)
			}

			_, oe := d.opConfirmClaim(actor, task)
			if oe == nil {
				t.Fatal("confirm against an unreachable pinned target must refuse, not confirm locally")
			}
			if oe.code != http.StatusServiceUnavailable || !strings.Contains(oe.msg, target) ||
				!strings.Contains(oe.msg, "restart the daemon") {
				t.Fatalf("refusal = %d %q; want a retryable 503 naming the target and the restart", oe.code, oe.msg)
			}
			if n := len(confirmedEvents(t, flushedEvents(t, d))); n != 0 {
				t.Fatalf("an honest refusal must write nothing; the ledger carries %d confirmations", n)
			}

			d.Shutdown("test: final sync")
			if carriesDataBranch(origin) {
				t.Fatal("origin received the data branch from a daemon pinned elsewhere")
			}
		})
	}
}

// TestMalformedTuhdooFailsStartup: a .tuhdoo the daemon cannot trust —
// no ledger.remote, or unparseable — ends startup before the socket is
// bound, naming the file. The daemon refuses to run rather than guess.
func TestMalformedTuhdooFailsStartup(t *testing.T) {
	for name, body := range map[string]string{
		"no ledger.remote": "[ledger]\n\tbranch = tuhdoo\n",
		"unparseable":      "[ledger\n\tremote = /x.git\n",
	} {
		t.Run(name, func(t *testing.T) {
			root, origin := pinnedRepo(t)
			writeTuhdoo(t, root, body)
			d, err := New(root, gateOpts())
			if err == nil {
				d.Shutdown("test cleanup")
				t.Fatal("New accepted a .tuhdoo without a usable ledger.remote")
			}
			if !strings.Contains(err.Error(), ".tuhdoo") {
				t.Fatalf("startup error %q does not name the file", err)
			}
			gd, _ := gitDirOf(root)
			if _, statErr := os.Stat(filepath.Join(gd, "tuhdoo", "daemon.json")); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("daemon.json exists after a refused startup (%v)", statErr)
			}
			if carriesDataBranch(origin) {
				t.Fatal("origin received the data branch")
			}
		})
	}
}

// setConfigFailsGit refuses every local config write.
type setConfigFailsGit struct{ gitx.Git }

func (setConfigFailsGit) SetLocalConfig(string, string) error {
	return errors.New("config is read-only")
}

// TestRecordFailureIsLoggedNotFatal: when the file's value cannot be
// written to local config, the daemon says so in its log and starts
// anyway — the file still pins this run.
func TestRecordFailureIsLoggedNotFatal(t *testing.T) {
	root, _ := pinnedRepo(t)
	ledger := newLedgerBare(t, root)
	writeTuhdoo(t, root, pointerFile(ledger))
	real, err := gitx.New(root)
	if err != nil {
		t.Fatal(err)
	}
	var logBuf bytes.Buffer
	opts := gateOpts()
	opts.Log = log.New(&logBuf, "", 0)
	opts.git = setConfigFailsGit{real}
	d, err := New(root, opts)
	if err != nil {
		t.Fatalf("New refused to start over a failed record: %v", err)
	}
	// Read before Run: New is synchronous, and nothing else logs yet.
	logged := logBuf.String()
	st := d.sync.Status()
	go d.Run() // Shutdown joins the load Run starts
	d.Shutdown("test cleanup")

	if st.Remote != ledger || st.Source != ".tuhdoo" {
		t.Fatalf("sync status = %+v, want the file's target despite the failed record", st)
	}
	if !strings.Contains(logged, "could not record") {
		t.Fatalf("the failed record was not logged; log:\n%s", logged)
	}
}
