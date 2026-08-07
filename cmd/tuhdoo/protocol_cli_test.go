package main

// `tuhdoo protocol` ships docs/agent-protocol.md inside the binary: one
// canonical text, versioned with the binary the agents actually talk
// to. Two guarantees are pinned here: the embedded bytes cannot drift
// from the file (the doc stays the single source of truth, like
// uninstall_doc_test.go does for the uninstall doc), and the command is
// a pure print — it works in a directory that is not even a git repo,
// with no daemon anywhere.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tuhdoo "github.com/brandonbews/tuhdoo"
)

const protocolDoc = "../../docs/agent-protocol.md"

// The embedded copy and the file are byte-for-byte identical. go:embed
// captures the file at compile time, so an edit to the doc without a
// rebuild — or a build from a tree where the doc went missing — shows
// up here, not in a silently forked protocol.
func TestProtocolEmbedMatchesDoc(t *testing.T) {
	want, err := os.ReadFile(protocolDoc)
	if err != nil {
		t.Fatalf("read %s: %v", protocolDoc, err)
	}
	if len(want) == 0 {
		t.Fatalf("%s is empty — nothing to embed", protocolDoc)
	}
	if tuhdoo.AgentProtocol != string(want) {
		t.Errorf("embedded protocol drifted from %s (embedded %d bytes, file %d bytes)",
			protocolDoc, len(tuhdoo.AgentProtocol), len(want))
	}
}

// The command prints the doc byte-for-byte and exits 0, from a plain
// directory with no .git — proof it needs no repo, no daemon, no
// socket. The descriptions-are-prompts section must survive shipping:
// its heading and all five bold parts appear in the printed output.
func TestProtocolCommandPrintsDocAnywhere(t *testing.T) {
	dir, err := os.MkdirTemp("", "tuhdoo-norepo")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	out, code := runCLI(t, dir, "protocol")
	if code != 0 {
		t.Fatalf("protocol exit %d; output:\n%s", code, out)
	}
	want, err := os.ReadFile(protocolDoc)
	if err != nil {
		t.Fatalf("read %s: %v", protocolDoc, err)
	}
	// Combined output equal to the file also proves stderr was silent.
	if out != string(want) {
		t.Errorf("protocol output is not the doc byte-for-byte (got %d bytes, want %d)",
			len(out), len(want))
	}
	mustContain(t, out, "## Writing tasks: descriptions are prompts",
		"**Context**", "**The ask**", "**Acceptance criteria**",
		"**Pointers**", "**Constraints**")

	// A pure print leaves no trace: no .git, no daemon discovery file,
	// nothing at all in the directory.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("protocol created files in a non-repo dir: %v", names)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); !os.IsNotExist(err) {
		t.Errorf("protocol conjured a .git (stat err: %v)", err)
	}

	// No flags, no arguments: anything extra is rejected loudly.
	out, code = runCLI(t, dir, "protocol", "--raw")
	if code == 0 {
		t.Fatalf("protocol --raw exited 0; output:\n%s", out)
	}
	mustContain(t, out, "usage: tuhdoo protocol")

	// And help advertises the command.
	out, code = runCLI(t, dir, "help")
	if code != 0 {
		t.Fatalf("help exit %d; output:\n%s", code, out)
	}
	if !strings.Contains(out, "protocol") {
		t.Errorf("help does not list the protocol command:\n%s", out)
	}
}
