package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

// seedTags applies the given tag layout to the main DB via `kpass edit`. Each
// entry path maps to a comma-joined tag list. Fails the test on any non-zero
// exit.
func (f *fixture) seedTags(t *testing.T, layout map[string]string) {
	t.Helper()
	for path, tags := range layout {
		_, stderr, code := f.runCLI("edit", path, "--tags", tags)
		if code != 0 {
			t.Fatalf("seedTags %s=%s: code=%d stderr=%s", path, tags, code, stderr)
		}
	}
}

// --- tags command -----------------------------------------------------------

func TestTagsListShowsCountsDesc(t *testing.T) {
	f := newFixture(t)
	f.seedTags(t, map[string]string{
		"internet/email":    "work,critical",
		"work/email":        "work",
		"simple":            "personal",
		"otp/sample":        "work,2fa",
		"db-passwords/work": "work,backup",
	})
	stdout, stderr, code := f.runCLI("tags")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 5 {
		t.Fatalf("expected 5 tag lines, got %d: %q", len(lines), stdout)
	}
	// First line must be "work" (count 4, highest).
	if !strings.HasPrefix(lines[0], "work") {
		t.Fatalf("first line should be work, got %q", lines[0])
	}
	if !strings.Contains(lines[0], "4") {
		t.Fatalf("work count should be 4, got %q", lines[0])
	}
}

func TestTagsListSortByName(t *testing.T) {
	f := newFixture(t)
	f.seedTags(t, map[string]string{
		"internet/email": "zulu",
		"work/email":     "alpha",
		"simple":         "mike",
	})
	stdout, _, code := f.runCLI("tags", "--sort", "name")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if !strings.HasPrefix(lines[0], "alpha") || !strings.HasPrefix(lines[1], "mike") || !strings.HasPrefix(lines[2], "zulu") {
		t.Fatalf("sort by name broken: %v", lines)
	}
}

func TestTagsNamesOnly(t *testing.T) {
	f := newFixture(t)
	f.seedTags(t, map[string]string{
		"internet/email": "work,2fa",
	})
	stdout, _, code := f.runCLI("tags", "--names")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	got := strings.Fields(stdout)
	want := map[string]bool{"work": true, "2fa": true}
	if len(got) != 2 {
		t.Fatalf("expected 2 tags, got %v", got)
	}
	for _, g := range got {
		if !want[g] {
			t.Fatalf("unexpected tag %q", g)
		}
	}
}

func TestTagsJSON(t *testing.T) {
	f := newFixture(t)
	f.seedTags(t, map[string]string{
		"internet/email": "work,critical",
		"work/email":     "work",
	})
	stdout, _, code := f.runCLI("tags", "--json")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	var pairs []struct {
		Tag   string `json:"tag"`
		Count int    `json:"count"`
	}
	if err := json.Unmarshal([]byte(stdout), &pairs); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(pairs) != 2 {
		t.Fatalf("expected 2 pairs, got %d", len(pairs))
	}
	if pairs[0].Tag != "work" || pairs[0].Count != 2 {
		t.Fatalf("first pair=%+v", pairs[0])
	}
}

func TestTagsEmpty(t *testing.T) {
	f := newFixture(t)
	stdout, _, code := f.runCLI("tags")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(stdout, "No tags") {
		t.Fatalf("stdout=%q", stdout)
	}
}

// --- tag add ----------------------------------------------------------------

func TestTagAddAttachesToEntries(t *testing.T) {
	f := newFixture(t)
	_, stderr, code := f.runCLI("tag", "add", "work", "internet/email", "work/email")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	db := openSeededDB(t, f.dbPath, "master-password")
	for _, p := range []string{"internet/email", "work/email"} {
		e := findEntryByPath(db, p)
		if e == nil || !strings.Contains(e.Tags, "work") {
			t.Fatalf("%s tags=%q", p, e.Tags)
		}
	}
}

func TestTagAddIsIdempotent(t *testing.T) {
	f := newFixture(t)
	f.seedTags(t, map[string]string{"internet/email": "work"})
	stdout, _, code := f.runCLI("tag", "add", "work", "internet/email")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(stdout, "No entries needed updates") {
		t.Fatalf("stdout=%q", stdout)
	}
	db := openSeededDB(t, f.dbPath, "master-password")
	e := findEntryByPath(db, "internet/email")
	if e.Tags != "work" {
		t.Fatalf("tags=%q (should not duplicate)", e.Tags)
	}
}

func TestTagAddCaseInsensitiveDedup(t *testing.T) {
	f := newFixture(t)
	f.seedTags(t, map[string]string{"internet/email": "Work"})
	_, _, code := f.runCLI("tag", "add", "WORK", "internet/email")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	db := openSeededDB(t, f.dbPath, "master-password")
	e := findEntryByPath(db, "internet/email")
	if e.Tags != "Work" {
		t.Fatalf("tags=%q (should keep original casing, no dupe)", e.Tags)
	}
}

func TestTagAddBadEntryErrors(t *testing.T) {
	f := newFixture(t)
	_, stderr, code := f.runCLI("tag", "add", "work", "no-such-entry")
	if code == 0 {
		t.Fatal("expected non-zero exit")
	}
	if stderr == "" {
		t.Fatal("expected error message")
	}
}

// --- tag remove -------------------------------------------------------------

func TestTagRemoveStripsTag(t *testing.T) {
	f := newFixture(t)
	f.seedTags(t, map[string]string{"internet/email": "work,critical"})
	_, _, code := f.runCLI("tag", "remove", "work", "internet/email")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	db := openSeededDB(t, f.dbPath, "master-password")
	e := findEntryByPath(db, "internet/email")
	if e.Tags != "critical" {
		t.Fatalf("tags=%q", e.Tags)
	}
}

func TestTagRemoveAliasRm(t *testing.T) {
	f := newFixture(t)
	f.seedTags(t, map[string]string{"internet/email": "work"})
	_, _, code := f.runCLI("tag", "rm", "work", "internet/email")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	db := openSeededDB(t, f.dbPath, "master-password")
	e := findEntryByPath(db, "internet/email")
	if e.Tags != "" {
		t.Fatalf("tags=%q", e.Tags)
	}
}

func TestTagRemoveAbsentNoop(t *testing.T) {
	f := newFixture(t)
	stdout, _, code := f.runCLI("tag", "remove", "ghost", "internet/email")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(stdout, "No entries had that tag") {
		t.Fatalf("stdout=%q", stdout)
	}
}

// --- tag rename -------------------------------------------------------------

func TestTagRename(t *testing.T) {
	f := newFixture(t)
	f.seedTags(t, map[string]string{
		"internet/email": "deprecated",
		"work/email":     "deprecated,active",
		"simple":         "active",
	})
	_, _, code := f.runCLI("tag", "rename", "deprecated", "legacy")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	db := openSeededDB(t, f.dbPath, "master-password")
	if got := findEntryByPath(db, "internet/email").Tags; got != "legacy" {
		t.Fatalf("internet/email=%q", got)
	}
	if got := findEntryByPath(db, "work/email").Tags; got != "legacy;active" {
		t.Fatalf("work/email=%q", got)
	}
	if got := findEntryByPath(db, "simple").Tags; got != "active" {
		t.Fatalf("simple=%q (unrelated, should not change)", got)
	}
}

func TestTagRenameMergesWhenNewExists(t *testing.T) {
	f := newFixture(t)
	f.seedTags(t, map[string]string{
		"internet/email": "old,new",
	})
	_, _, code := f.runCLI("tag", "rename", "old", "new")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	db := openSeededDB(t, f.dbPath, "master-password")
	if got := findEntryByPath(db, "internet/email").Tags; got != "new" {
		t.Fatalf("tags=%q (rename should merge into existing)", got)
	}
}

func TestTagRenameEmptyNewErrors(t *testing.T) {
	f := newFixture(t)
	f.seedTags(t, map[string]string{"internet/email": "old"})
	_, stderr, code := f.runCLI("tag", "rename", "old", "")
	if code == 0 {
		t.Fatal("expected non-zero exit")
	}
	if !strings.Contains(stderr, "cannot be empty") {
		t.Fatalf("stderr=%q", stderr)
	}
}

func TestTagRenameSameErrors(t *testing.T) {
	f := newFixture(t)
	f.seedTags(t, map[string]string{"internet/email": "x"})
	_, stderr, code := f.runCLI("tag", "rename", "X", "x")
	if code == 0 {
		t.Fatal("expected non-zero exit")
	}
	if !strings.Contains(stderr, "are the same") {
		t.Fatalf("stderr=%q", stderr)
	}
}

func TestTagRenameNoOpReports(t *testing.T) {
	f := newFixture(t)
	stdout, _, code := f.runCLI("tag", "rename", "nothing", "something")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(stdout, "No entries had that tag") {
		t.Fatalf("stdout=%q", stdout)
	}
}

// --- ls --tag / --tag-any ---------------------------------------------------

func TestLsTagFilterAND(t *testing.T) {
	f := newFixture(t)
	f.seedTags(t, map[string]string{
		"internet/email": "work,critical",
		"work/email":     "work",
		"simple":         "personal",
	})
	stdout, _, code := f.runCLI("ls", "--flat", "--tag", "work", "--tag", "critical")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	got := strings.Fields(stdout)
	if len(got) != 1 || got[0] != "internet/email" {
		t.Fatalf("got %v", got)
	}
}

func TestLsTagAnyOR(t *testing.T) {
	f := newFixture(t)
	f.seedTags(t, map[string]string{
		"internet/email": "work",
		"work/email":     "personal",
		"simple":         "other",
	})
	stdout, _, code := f.runCLI("ls", "--flat", "--tag-any", "work", "--tag-any", "personal")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	got := strings.Fields(stdout)
	want := map[string]bool{"internet/email": true, "work/email": true}
	if len(got) != 2 {
		t.Fatalf("got %v, want 2 matches", got)
	}
	for _, g := range got {
		if !want[g] {
			t.Fatalf("unexpected %q", g)
		}
	}
}

func TestLsTagFilterEmptyResult(t *testing.T) {
	f := newFixture(t)
	stdout, _, code := f.runCLI("ls", "--flat", "--tag", "nonexistent")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("stdout=%q", stdout)
	}
}

// --- search --tag-any -------------------------------------------------------

func TestSearchTagAnyOR(t *testing.T) {
	f := newFixture(t)
	f.seedTags(t, map[string]string{
		"internet/email": "work",
		"work/email":     "personal",
		"simple":         "other",
	})
	stdout, _, code := f.runCLI("search", "email", "--format", "flat", "--tag-any", "work", "--tag-any", "personal")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	got := strings.Fields(stdout)
	if len(got) != 2 {
		t.Fatalf("got %v, want 2 matches (internet/email + work/email)", got)
	}
}

// --- __complete tags --------------------------------------------------------

func TestCompleteTagsListsAllTags(t *testing.T) {
	f := newFixture(t)
	f.seedTags(t, map[string]string{
		"internet/email": "alpha,bravo",
		"work/email":     "charlie",
	})
	stdout, _, code := f.runCLI("__complete", "tags", "", "", "")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	got := strings.Fields(stdout)
	want := map[string]bool{"alpha": true, "bravo": true, "charlie": true}
	if len(got) != 3 {
		t.Fatalf("got %v want 3", got)
	}
	for _, g := range got {
		if !want[g] {
			t.Fatalf("unexpected %q", g)
		}
	}
}

func TestCompleteTagsPrefixFilter(t *testing.T) {
	f := newFixture(t)
	f.seedTags(t, map[string]string{
		"internet/email": "alpha,beta",
		"work/email":     "gamma",
	})
	// Filter prefix is the 4th positional arg.
	stdout, _, code := f.runCLI("__complete", "tags", "", "", "a")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	got := strings.Fields(stdout)
	if len(got) != 1 || got[0] != "alpha" {
		t.Fatalf("got %v", got)
	}
}

// --- tag filter helper unit -------------------------------------------------

func TestMatchTagFilterEmptyFilter(t *testing.T) {
	// Empty filter matches anything — even an entry with no tags.
	if !matchTagFilter(nil, nil, nil) {
		t.Fatal("nil entry should match empty filter (helper is total)")
	}
}
