package tuhdoo

// This test keeps references to the project's private planning material
// out of the public tree. tuhdoo is planned in notes and a ledger that
// live in a private repository, so a comment or doc that cites them (a
// decision code, a private document's name or path, the date something
// was decided, a person's name, a ledger task ID) points the reader at
// something they cannot open. Comments and docs carry the rule and its
// reason in plain words instead. The test runs under `go test ./...`, so
// it gates every pull request and every release.

import (
	"bytes"
	"fmt"
	"go/scanner"
	"go/token"
	"os"
	"os/exec"
	"path"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/brandonbews/tuhdoo/internal/event"
)

// leakRule is one kind of internal reference. The regexp's first
// capture group is the text a hit reports.
type leakRule struct {
	name string
	re   *regexp.Regexp
	// notInGoStrings skips Go string literals, and notInTestdata skips
	// files under a testdata/ directory: test data legitimately carries
	// timestamps and made-up task IDs.
	notInGoStrings bool
	notInTestdata  bool
	// unlessFollowedBy, when set, drops a match whose following text
	// matches it.
	unlessFollowedBy *regexp.Regexp
}

// leakCodeAtom is one decision or backlog code: D1–D11, T1–T8, B1–B99.
const leakCodeAtom = `D(?:1[01]|[1-9])|T[1-8]|B[1-9][0-9]?`

var leakRules = []leakRule{
	// Codes, alone or joined by slashes: "(D6)", "B12 migration",
	// "(T5/T8)". The boundary classes keep out codes that are part of
	// something else: base64 and base32 runs ("+T1/", "QD6"), hex
	// ("0xD1", "#B1d5db"), dotted or hyphenated names, times ("T15:04").
	{name: "code", re: regexp.MustCompile(
		`(?:^|[^\w+/=.#-])((?:` + leakCodeAtom + `)(?:/(?:` + leakCodeAtom + `))*)(?:$|[^\w+/=])`)},
	// Private document numbers used as citations: "002 T7", "`001`",
	// "001-overview.md".
	{name: "docnum", re: regexp.MustCompile("(`00[1-3]`|\\b00[1-3] [DT][0-9]|\\b00[1-3]-[a-z][a-z-]*(?:\\.md)?)")},
	// Names of the sessions decisions were made in.
	{name: "cycle", re: regexp.MustCompile(`(?i)(\bcycle[ -][0-9]+\b|\bgrill)`)},
	// Private paths and document names.
	{name: "path", re: regexp.MustCompile(`(internal-docs|docs/design|agent-rules|plan/roadmap|plan/backlog|open-questions\.md|tuhdoo-internal|CLAUDE\.local|architecture\.md)`)},
	// Phrases that only make sense with the private documents at hand.
	{name: "docword", re: regexp.MustCompile(`(?i)(\bdesign[- ]doc(?:s|'s)?\b|\bdesign record\b|\bfounding (?:decision|principle)|\brevision note\b|\broadmap\b|\bdefinition of done\b|\bDoD\b|\bmock-[ab]\b|\bmockup)`)},
	// People, and who decided what when.
	{name: "name", re: regexp.MustCompile(`(\bBrandon\b|@gmail\.com|\bsteering, 20[0-9]{2})`)},
	// Full task IDs.
	{name: "taskid", re: regexp.MustCompile(`\b((?:tuh|t)-[0-9A-HJKMNP-TV-Z]{26})\b`),
		notInGoStrings: true, notInTestdata: true},
	// Calendar dates, which in prose are nearly always the day something
	// was decided. A date followed by a time of day is a timestamp.
	{name: "date", re: regexp.MustCompile(`\b(20[0-9]{2}-[01][0-9]-[0-3][0-9])\b`),
		notInGoStrings: true, notInTestdata: true,
		unlessFollowedBy: regexp.MustCompile(`^[ T][0-9]{2}:[0-9]{2}`)},
}

// leakAllowlist holds the legitimate uses the rules flag anyway. A hit
// is allowed when it is in file path and its line contains substring.
// Every entry must allow at least one hit, so none outlives its line.
var leakAllowlist = []struct{ path, substring, why string }{
	{"docs/agent-protocol.md", "with links to design docs or prior tasks",
		"general advice about the adopting project's own design docs"},
	{"docs/agent-protocol.md", "committed files, design docs, task IDs",
		"general advice about the adopting project's own design docs"},
	{"docs/joining.md", "(verified live, 2026-08-21)",
		"the day a claim about Vercel was last checked, so readers can judge how fresh it is"},
	{"docs/joining.md", "(verified live on a public repository, 2026-10-11)",
		"the day a claim about Cloudflare Pages was last checked, so readers can judge how fresh it is"},
	{"docs/recipes/vercel.md", "verified live against a real adopter repo on 2026-08-21",
		"the day the recipe was last checked, so readers can judge how fresh it is"},
	{"npm/smoke.sh", `"protocolVersion":"2025-06-18"`,
		"an MCP protocol version, which is named by its date"},
	{"cmd/tuhdoo/mcp_cmd.go", "2026-07-28",
		"an MCP protocol version, which is named by its date"},
	{"CLAUDE.md", "is a gitignored clone of the private `brandonbews/tuhdoo-internal` repo",
		"tells maintainers where their private notes clone goes"},
	// The README's "Private notes" section is the setup guide for that
	// clone, so it names the repo, the folder and the files it links.
	{"README.md", "tuhdoo-internal", "the private notes repo the setup clones"},
	{"README.md", "internal-docs", "the folder the private notes clone goes in"},
	{"README.md", "CLAUDE.local", "the file the setup script links"},
	{"README.md", "agent-rules", "the file CLAUDE.local.md links to"},
	{".gitignore", "a clone of the private tuhdoo-internal repo",
		"explains the ignore entries below it"},
	{".gitignore", "/internal-docs",
		"keeps the maintainers' private notes clone out of commits"},
	{".gitignore", "/CLAUDE.local.md",
		"keeps a maintainer's local agent instructions out of commits"},
}

// leakSkipped are files never scanned: checksums, the license text,
// the file that names the private ledger repo on purpose, and this file,
// which spells out every pattern.
var leakSkipped = map[string]bool{
	"go.sum":                 true,
	"site/package-lock.json": true,
	"LICENSE":                true,
	".tuhdoo":                true,
	"leak_test.go":           true,
}

const leakHint = "Rewrite each one in plain words: state the rule and its reason, not where, when or by whom it was decided. " +
	"If a use is legitimate, add an entry with a one-line reason to leakAllowlist in leak_test.go."

// leakHit is one rule match in one line of one file.
type leakHit struct {
	path     string
	line     int // 1-based
	rule     string
	text     string
	lineText string // the whole line, which the allowlist is matched against
}

func inTestdata(p string) bool {
	return strings.HasPrefix(p, "testdata/") || strings.Contains(p, "/testdata/")
}

// leakMatch is one rule's hit in a piece of text.
type leakMatch struct {
	rule string
	text string // the matched text
}

// leakMatches applies the rules to one line of text from file p.
// inGoString says the text is part of a Go string literal.
func leakMatches(p string, inGoString bool, text string) []leakMatch {
	var out []leakMatch
	for _, r := range leakRules {
		if (r.notInGoStrings && inGoString) || (r.notInTestdata && inTestdata(p)) {
			continue
		}
		for _, m := range r.re.FindAllStringSubmatchIndex(text, -1) {
			start, end := m[2], m[3]
			if r.unlessFollowedBy != nil && r.unlessFollowedBy.MatchString(text[end:]) {
				continue
			}
			out = append(out, leakMatch{r.name, text[start:end]})
		}
	}
	return out
}

// scanForLeaks returns the hits in one file, in line order. A Go file is
// tokenized so that the rules check only its comments and string
// literals; any other file is checked line by line. When ledger is not
// nil, every line outside testdata/ is also checked for real task IDs.
func scanForLeaks(p string, src []byte, ledger *ledgerIDs) []leakHit {
	lines := strings.Split(string(src), "\n")
	var hits []leakHit
	add := func(line int, inGoString bool, text string) {
		for _, m := range leakMatches(p, inGoString, text) {
			hits = append(hits, leakHit{p, line, m.rule, m.text, lines[line-1]})
		}
	}
	if strings.HasSuffix(p, ".go") {
		fset := token.NewFileSet()
		file := fset.AddFile(p, -1, len(src))
		var s scanner.Scanner
		s.Init(file, src, nil, scanner.ScanComments)
		for {
			pos, tok, lit := s.Scan()
			if tok == token.EOF {
				break
			}
			if tok != token.COMMENT && tok != token.STRING {
				continue
			}
			// Unadjusted, so a //line directive cannot move the position.
			first := file.PositionFor(pos, false).Line
			for i, part := range strings.Split(lit, "\n") {
				add(first+i, tok == token.STRING, part)
			}
		}
	} else {
		for i, line := range lines {
			add(i+1, false, line)
		}
	}
	if ledger != nil && !inTestdata(p) {
		for i, line := range lines {
			for _, id := range ledger.matches(line) {
				hits = append(hits, leakHit{p, i + 1, "ledger-id", id, line})
			}
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].line < hits[j].line })
	return hits
}

// ledgerIDs recognizes real task IDs: in full, as bare ULIDs, and in
// the short display form. Real IDs cannot be told from made-up ones by
// their shape, so this needs the ledger itself.
type ledgerIDs struct {
	known      map[string]bool
	candidates *regexp.Regexp
}

// newLedgerIDs builds the recognizer from full task IDs ("tuh-<ULID>").
func newLedgerIDs(ids []string) *ledgerIDs {
	known := make(map[string]bool)
	prefixSet := make(map[string]bool)
	for _, id := range ids {
		i := strings.Index(id, "-")
		if i <= 0 {
			continue
		}
		known[id] = true
		known[id[i+1:]] = true
		known[event.ShortID(id)] = true
		prefixSet[regexp.QuoteMeta(id[:i+1])] = true
	}
	var prefixes []string
	for p := range prefixSet {
		prefixes = append(prefixes, p)
	}
	sort.Strings(prefixes)
	pattern := `\b[0-9A-Z]{26}\b`
	if len(prefixes) > 0 {
		pattern = `\b(?:` + strings.Join(prefixes, "|") + `)[0-9A-Za-z]+\b|` + pattern
	}
	return &ledgerIDs{known: known, candidates: regexp.MustCompile(pattern)}
}

// matches returns the real task IDs in one line of text.
func (l *ledgerIDs) matches(text string) []string {
	var out []string
	for _, c := range l.candidates.FindAllString(text, -1) {
		if l.known[c] {
			out = append(out, c)
			continue
		}
		// A real ULID behind some other prefix.
		if _, ulid, ok := strings.Cut(c, "-"); ok && l.known[ulid] {
			out = append(out, ulid)
		}
	}
	return out
}

// readLedgerIDs reads the real task IDs from the data branch, if this
// clone has one (a CI checkout does not). Each task has a page at
// tasks/<id>.md on the branch. It returns nil when there is no ledger.
func readLedgerIDs(t *testing.T) *ledgerIDs {
	t.Helper()
	var ids []string
	found := false
	for _, ref := range []string{"refs/heads/tuhdoo", "refs/tuhdoo/remote"} {
		if exec.Command("git", "rev-parse", "--verify", "-q", ref).Run() != nil {
			continue
		}
		found = true
		out, err := exec.Command("git", "ls-tree", "--name-only", ref, "tasks/").Output()
		if err != nil {
			t.Fatalf("git ls-tree %s tasks/: %v", ref, err)
		}
		for _, name := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if strings.HasSuffix(name, ".md") {
				ids = append(ids, strings.TrimSuffix(path.Base(name), ".md"))
			}
		}
	}
	if !found {
		return nil
	}
	return newLedgerIDs(ids)
}

func TestNoInternalReferences(t *testing.T) {
	out, err := exec.Command("git", "rev-parse", "--is-inside-work-tree").Output()
	if err != nil || strings.TrimSpace(string(out)) != "true" {
		t.Skip("not inside a git work tree, so there is no file list to check")
	}
	// Untracked files too, so a new file is checked before it is added.
	out, err = exec.Command("git", "ls-files", "-z", "--cached", "--others", "--exclude-standard").Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	ledger := readLedgerIDs(t)

	var hits []leakHit
	seen := make(map[string]bool)
	for _, p := range strings.Split(string(out), "\x00") {
		if p == "" || seen[p] || leakSkipped[p] {
			continue
		}
		seen[p] = true
		info, err := os.Lstat(p)
		if os.IsNotExist(err) {
			continue // deleted in the work tree but not yet in the index
		}
		if err != nil {
			t.Fatal(err)
		}
		if !info.Mode().IsRegular() {
			continue // a symlink's target is checked under its own path
		}
		src, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		// Git's own test for binary content: a NUL byte near the start.
		if bytes.IndexByte(src[:min(len(src), 8000)], 0) >= 0 {
			continue
		}
		hits = append(hits, scanForLeaks(p, src, ledger)...)
	}

	used := make([]bool, len(leakAllowlist))
	var report strings.Builder
	n := 0
	for _, h := range hits {
		allowed := false
		for i, a := range leakAllowlist {
			if h.path == a.path && strings.Contains(h.lineText, a.substring) {
				used[i] = true
				allowed = true
			}
		}
		if !allowed {
			fmt.Fprintf(&report, "%s:%d: %s: %s\n", h.path, h.line, h.rule, h.text)
			n++
		}
	}
	if n > 0 {
		t.Errorf("%d references to private planning material:\n%s%s", n, report.String(), leakHint)
	}
	for i, a := range leakAllowlist {
		if !used[i] {
			t.Errorf("leakAllowlist entry allows nothing, remove it: %s %q", a.path, a.substring)
		}
	}
}

// The rules flag what they should and pass what they should, so a
// regexp edit cannot quietly widen or narrow them.
func TestLeakRules(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		inGoString bool
		text       string
		want       []string // nil: the line passes
	}{
		// Must flag.
		{"code in parentheses", "x.go", false, "the winner rule (D6) applies", []string{"code: D6"}},
		{"code after a doc number", "x.md", false, "see 002 T7", []string{"code: T7", "docnum: 002 T7"}},
		{"backlog number", "x.md", false, "the B12 migration", []string{"code: B12"}},
		{"codes joined by a slash", "x.md", false, "both hold (T5/T8).", []string{"code: T5/T8"}},
		{"backlog number joined to a code", ".github/workflows/x.yml", false, "# filter guidance (B1/T2)", []string{"code: B1/T2"}},
		{"slash-joined codes at line start", "x.md", false, "B1/T2 says so", []string{"code: B1/T2"}},
		{"code in a Go string", "x.go", true, "claims follow D6", []string{"code: D6"}},
		{"doc file name", "x.md", false, "see 001-overview.md", []string{"docnum: 001-overview.md"}},
		{"session name", "x.md", false, "settled in the edge grill", []string{"cycle: grill"}},
		{"session number", "x.md", false, "as Cycle 2 amended it", []string{"cycle: Cycle 2"}},
		{"private path", "x.md", false, "internal-docs/notes.md", []string{"path: internal-docs"}},
		{"doc idiom", "x.md", false, "per the design doc", []string{"docword: design doc"}},
		{"person", "x.md", false, "decided by Brandon", []string{"name: Brandon"}},
		{"decision date", "x.go", false, "(steering, 2026-09-11):", []string{"name: steering, 2026", "date: 2026-09-11"}},
		{"bare decision date", "x.md", false, "since 2026-09-11):", []string{"date: 2026-09-11"}},
		{"task ID in prose", "x.md", false, "see tuh-01KYT63MB28Z535SMJC9B0QX4R", []string{"taskid: tuh-01KYT63MB28Z535SMJC9B0QX4R"}},

		// Must pass.
		{"base64 run", "x.json", false, `"sig":"AbC+T1/xyz="`, nil},
		{"base64 run with slash-joined codes", "x.json", false, `"sig":"AbC+T1/T8x"`, nil},
		{"base32 run", "x.go", false, "id ends in QD6", nil},
		{"time of day after a code letter", "x.go", false, "layout 2006-01-02T15:04:05Z07:00", nil},
		{"hex literal", "x.go", false, "mask 0xD1", nil},
		{"hex color", "x.css", false, "color: #B1d5db;", nil},
		{"short hex color", "x.css", false, "color: #B12;", nil},
		{"dotted name", "x.go", false, "cfg.T1 is set", nil},
		{"hyphenated name", "x.go", false, "the pre-T2 layout", nil},
		{"identifier", "x.go", false, "see TestD6Winner", nil},
		{"event schema versions", "x.go", false, "upcast v1→v2", nil},
		{"API path", "x.md", false, "GET /v0/snapshot", nil},
		{"timestamp", "x.md", false, "at 2026-07-29 12:17 UTC", nil},
		{"date in a Go string", "x.go", true, `"2026-09-11"`, nil},
		{"task ID in a Go string", "x.go", true, `"tuh-01KYT63MB28Z535SMJC9B0QX4R"`, nil},
		{"date in testdata", "internal/views/testdata/golden/README.md", false, "created 2026-09-11", nil},
		{"task ID in testdata", "internal/views/testdata/golden/README.md", false, "tuh-01KYT63MB28Z535SMJC9B0QX4R", nil},
		{"fixture principal", "x.go", true, "brandon/impl-1", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for _, m := range leakMatches(tc.path, tc.inGoString, tc.text) {
				got = append(got, m.rule+": "+m.text)
			}
			if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
				t.Errorf("leakMatches(%q) = %q, want %q", tc.text, got, tc.want)
			}
		})
	}
}

// The ledger check finds real IDs in all three forms and nothing else.
func TestLedgerIDs(t *testing.T) {
	l := newLedgerIDs([]string{
		"tuh-01KYT63MB28Z535SMJC9B0QX4R",
		"t-01KYT63MB28Z535SMJCA63WV7K",
	})
	tests := []struct {
		text string
		want []string
	}{
		{"see tuh-01KYT63MB28Z535SMJC9B0QX4R.", []string{"tuh-01KYT63MB28Z535SMJC9B0QX4R"}},
		{"events/2026/07/30/01KYT63MB28Z535SMJC9B0QX4R.json", []string{"01KYT63MB28Z535SMJC9B0QX4R"}},
		{"fixed in tuh-qx4r and t-wv7k", []string{"tuh-qx4r", "t-wv7k"}},
		{"branch tuh-qx4r/some-slug", []string{"tuh-qx4r"}},
		{"tuh-01KYT63MB28Z535SMJCA63WV7K", []string{"01KYT63MB28Z535SMJCA63WV7K"}},
		{"t-qx4r is another task's tail", nil},
		{"tuh-zzzz and 01KYT63MB28Z535SMJC9B0ZZZZ", nil},
		{"the tuh-qx4rx branch", nil},
	}
	for _, tc := range tests {
		got := l.matches(tc.text)
		if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
			t.Errorf("matches(%q) = %q, want %q", tc.text, got, tc.want)
		}
	}
}
