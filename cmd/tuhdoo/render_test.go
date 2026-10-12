package main

import (
	"strings"
	"testing"
	"time"
)

// syncLine is the one-shot surfaces' sync status. The origin default
// reads exactly as it always has; a pinned ledger target also says
// where it came from, and its error says a change takes a restart.
// Errors render on one line whatever git printed.
func TestSyncLine(t *testing.T) {
	const url = "git@github.com:owner/repo-internal.git"
	fetched := time.Date(2026, 10, 12, 9, 30, 0, 0, time.UTC).Format(time.RFC3339)
	for _, tc := range []struct {
		name string
		in   syncJSON
		want string
	}{
		{"local-only", syncJSON{Mode: "local-only", Remote: "origin"}, "local-only"},
		{"origin syncing", syncJSON{Mode: "syncing", Remote: "origin", LastFetch: fetched},
			`syncing with "origin" · last fetch 2026-10-12 09:30 UTC`},
		{"origin error", syncJSON{Mode: "error", Remote: "origin", LastError: "connection refused"},
			`sync error (remote "origin"): connection refused`},
		{"origin starting", syncJSON{Mode: "starting", Remote: "origin"}, "starting"},
		{"pinned by file, syncing", syncJSON{Mode: "syncing", Remote: url, RemoteSource: ".tuhdoo", LastFetch: fetched},
			`syncing with "` + url + `" from .tuhdoo · last fetch 2026-10-12 09:30 UTC`},
		{"pinned by local config, syncing", syncJSON{Mode: "syncing", Remote: "/srv/ledger.git", RemoteSource: "git config tuhdoo.remote"},
			`syncing with "/srv/ledger.git" from git config tuhdoo.remote`},
		{"pinned error, multi-line git output", syncJSON{Mode: "error", Remote: url, RemoteSource: ".tuhdoo",
			LastError: "fatal: Could not read from remote repository.\nPlease make sure you have the correct access rights"},
			`sync error (remote "` + url + `" from .tuhdoo; changing it takes a daemon restart): ` +
				"fatal: Could not read from remote repository. Please make sure you have the correct access rights"},
		{"pinned starting", syncJSON{Mode: "starting", Remote: url, RemoteSource: ".tuhdoo"},
			`starting · remote "` + url + `" from .tuhdoo`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := syncLine(tc.in)
			if got != tc.want {
				t.Fatalf("syncLine = %q\nwant        %q", got, tc.want)
			}
			if strings.Contains(got, "\n") {
				t.Fatalf("syncLine spans lines: %q", got)
			}
		})
	}
}

// The TUI header's sync error stays on its one line with a URL target
// and git's multi-line output.
func TestSyncSegsErrorIsOneLine(t *testing.T) {
	segs := syncSegs(colors{}, syncJSON{Mode: "error", Remote: "git@github.com:o/r-internal.git",
		RemoteSource: ".tuhdoo", LastError: "fatal: no access\nfatal: Could not read from remote repository."}, time.Now())
	if len(segs) != 1 {
		t.Fatalf("segs = %+v, want one", segs)
	}
	want := `sync error (remote "git@github.com:o/r-internal.git"): fatal: no access fatal: Could not read from remote repository.`
	if segs[0].text != want {
		t.Fatalf("header = %q\nwant     %q", segs[0].text, want)
	}
}
