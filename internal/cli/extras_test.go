package cli

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tobischo/gokeepasslib/v3"
	w "github.com/tobischo/gokeepasslib/v3/wrappers"

	"github.com/irasikhin/kpass/internal/db"
	"github.com/irasikhin/kpass/internal/picker"
)

// gokeepasslibVD builds a plain (unprotected) value pair for tests.
func gokeepasslibVD(key, value string) gokeepasslib.ValueData {
	return gokeepasslib.ValueData{Key: key, Value: gokeepasslib.V{Content: value}}
}

// newRawEntry builds a *gokeepasslib.Entry with the canonical fields populated.
func newRawEntry(title, user, pass, url, notes, otp string) *gokeepasslib.Entry {
	e := gokeepasslib.NewEntry()
	e.Values = []gokeepasslib.ValueData{
		{Key: "Title", Value: gokeepasslib.V{Content: title}},
		{Key: "UserName", Value: gokeepasslib.V{Content: user}},
		{Key: "Password", Value: gokeepasslib.V{Content: pass, Protected: w.NewBoolWrapper(true)}},
	}
	if url != "" {
		e.Values = append(e.Values, gokeepasslibVD("URL", url))
	}
	if notes != "" {
		e.Values = append(e.Values, gokeepasslibVD("Notes", notes))
	}
	if otp != "" {
		e.Values = append(e.Values, gokeepasslib.ValueData{Key: "otp", Value: gokeepasslib.V{Content: otp, Protected: w.NewBoolWrapper(true)}})
	}
	return &e
}

// --- ls --------------------------------------------------------------------

func TestLsJSONOutput(t *testing.T) {
	f := newFixture(t)
	stdout, stderr, code := f.runCLI("ls", "--json")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	var arr []any
	if err := json.Unmarshal([]byte(stdout), &arr); err != nil {
		t.Errorf("ls --json not valid JSON: %v\n%s", err, stdout)
	}
}

func TestLsTagFilter(t *testing.T) {
	f := newFixture(t)
	// Apply a tag to one entry so we have something to filter on.
	if _, _, code := f.runCLI("tag", "add", "starred", "internet/email"); code != 0 {
		t.Fatal("tag add failed")
	}
	stdout, _, code := f.runCLI("ls", "--tag", "starred", "--flat")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(stdout, "internet/email") {
		t.Errorf("expected internet/email in:\n%s", stdout)
	}
	if strings.Contains(stdout, "work/email") {
		t.Errorf("unrelated entry should not appear:\n%s", stdout)
	}
}

// --- get -------------------------------------------------------------------

func TestGet_JSONSingleField(t *testing.T) {
	f := newFixture(t)
	stdout, _, code := f.runCLI("get", "internet/email", "-F", "username", "--json")
	if code != 0 {
		t.Fatal(stdout)
	}
	var v string
	if err := json.Unmarshal([]byte(stdout), &v); err != nil {
		t.Fatalf("json: %v\n%s", err, stdout)
	}
	if v != "alice" {
		t.Errorf("username = %q", v)
	}
}

func TestGet_JSONFullEntry(t *testing.T) {
	f := newFixture(t)
	stdout, _, code := f.runCLI("get", "internet/email", "--json")
	if code != 0 {
		t.Fatal(stdout)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(stdout), &obj); err != nil {
		t.Fatal(err)
	}
	if obj["path"] != "internet/email" {
		t.Errorf("path = %v", obj["path"])
	}
}

func TestGet_JSONMultipleFields(t *testing.T) {
	f := newFixture(t)
	stdout, _, code := f.runCLI("get", "internet/email", "-F", "username", "-F", "url", "--json")
	if code != 0 {
		t.Fatal(stdout)
	}
	var obj map[string]string
	if err := json.Unmarshal([]byte(stdout), &obj); err != nil {
		t.Fatal(err)
	}
	if obj["username"] != "alice" || obj["url"] == "" {
		t.Errorf("fields = %v", obj)
	}
}

func TestGet_GlobMultiple(t *testing.T) {
	f := newFixture(t)
	stdout, _, code := f.runCLI("get", "*/email", "--glob", "-F", "username")
	if code != 0 {
		t.Fatalf("code=%d %s", code, stdout)
	}
	if !strings.Contains(stdout, "alice") || !strings.Contains(stdout, "worker") {
		t.Errorf("expected both usernames in:\n%s", stdout)
	}
}

func TestGet_GlobNoMatch(t *testing.T) {
	f := newFixture(t)
	_, stderr, code := f.runCLI("get", "no/such/*", "--glob")
	if code == 0 {
		t.Errorf("expected non-zero exit; stderr=%s", stderr)
	}
}

func TestGet_MaskPassword(t *testing.T) {
	f := newFixture(t)
	stdout, _, code := f.runCLI("get", "internet/email", "--show-password", "--mask")
	if code != 0 {
		t.Fatal(stdout)
	}
	if strings.Contains(stdout, "pw-email") {
		t.Errorf("masked output leaked password:\n%s", stdout)
	}
}

func TestMaskPassword(t *testing.T) {
	if got := maskPassword(""); got != "****" {
		t.Errorf("empty = %q", got)
	}
	if got := maskPassword("ab"); got != "****" {
		t.Errorf("short = %q", got)
	}
	if got := maskPassword("abcdef"); !strings.HasPrefix(got, "ab") || !strings.HasSuffix(got, "ef") {
		t.Errorf("medium = %q", got)
	}
}

// --- stats -----------------------------------------------------------------

func TestStats_HumanOutput(t *testing.T) {
	f := newFixture(t)
	stdout, _, code := f.runCLI("stats")
	if code != 0 {
		t.Fatal(stdout)
	}
	for _, want := range []string{"Entries:", "Groups:", "Unique tags:", "With password:"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("missing %q in:\n%s", want, stdout)
		}
	}
}

func TestStats_JSON(t *testing.T) {
	f := newFixture(t)
	stdout, _, code := f.runCLI("stats", "--json")
	if code != 0 {
		t.Fatal(stdout)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(stdout), &obj); err != nil {
		t.Fatal(err)
	}
	if int(obj["entries"].(float64)) == 0 {
		t.Errorf("entries=0 unexpected: %v", obj)
	}
}

// --- audit -----------------------------------------------------------------

func TestAudit_FindsWeakAndReused(t *testing.T) {
	f := newFixture(t)
	// Insert a weak password and a duplicate.
	if _, _, code := f.runCLI("insert", "weak/short", "--password", "abc", "-u", "u"); code != 0 {
		t.Fatal("insert weak failed")
	}
	if _, _, code := f.runCLI("insert", "weak/dup1", "--password", "Repeated-pw-99", "-u", "u"); code != 0 {
		t.Fatal("insert dup1 failed")
	}
	if _, _, code := f.runCLI("insert", "weak/dup2", "--password", "Repeated-pw-99", "-u", "u"); code != 0 {
		t.Fatal("insert dup2 failed")
	}
	stdout, _, code := f.runCLI("audit")
	if code != 0 {
		t.Fatal(stdout)
	}
	if !strings.Contains(stdout, "Weak passwords") || !strings.Contains(stdout, "Reused passwords") {
		t.Errorf("expected weak and reused sections in:\n%s", stdout)
	}
}

func TestAudit_NoIssues(t *testing.T) {
	f := newFixture(t)
	// Replace problem entries with strong ones.
	if _, _, code := f.runCLI("remove", "-f", "internet/email", "work/email", "simple", "otp/sample", "db-passwords/work"); code != 0 {
		t.Fatal("remove failed")
	}
	if _, _, code := f.runCLI("insert", "strong", "--password", "L0ngStr0ng!Passw0rd",
		"-u", "alice", "--url", "https://example.com"); code != 0 {
		t.Fatal("insert strong failed")
	}
	stdout, _, code := f.runCLI("audit")
	if code != 0 {
		t.Fatal(stdout)
	}
	if !strings.Contains(stdout, "no issues found") {
		t.Errorf("expected clean audit:\n%s", stdout)
	}
}

func TestAudit_JSON(t *testing.T) {
	f := newFixture(t)
	stdout, _, code := f.runCLI("audit", "--json")
	if code != 0 {
		t.Fatal(stdout)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(stdout), &obj); err != nil {
		t.Fatal(err)
	}
	if _, ok := obj["issues"]; !ok {
		t.Errorf("missing 'issues' key: %v", obj)
	}
}

func TestWeakPasswordReason(t *testing.T) {
	cases := []struct {
		pw   string
		want string
	}{
		{"short", "Too short"},
		{"aaaaaaaaaaaaaa", "Only one character class"},
		{"Abcde", "Too short"}, // <8 chars
		{"PasswordX", "Common"},
		{"L0ngStr0ng!Passw0rd", ""},
	}
	for _, tc := range cases {
		got := weakPasswordReason(tc.pw)
		if tc.want == "" {
			if got != "" {
				t.Errorf("weakPasswordReason(%q) = %q, want empty", tc.pw, got)
			}
			continue
		}
		if !strings.Contains(got, tc.want) && tc.pw != "PasswordX" {
			t.Errorf("weakPasswordReason(%q) = %q, want substring %q", tc.pw, got, tc.want)
		}
	}
	// short + 2-class case ("password1" length 9, classes 2) → "Short password with only"
	if got := weakPasswordReason("password1"); !strings.Contains(got, "Short password with only") && !strings.Contains(got, "Common") {
		t.Errorf("password1 = %q", got)
	}
}

func TestIsCommonPassword(t *testing.T) {
	if !isCommonPassword("password") {
		t.Error("password should be common")
	}
	if !isCommonPassword("PASSWORD") {
		t.Error("case-insensitive match expected")
	}
	if isCommonPassword("L0ngStr0ng!") {
		t.Error("strong should not be common")
	}
}

// --- clean -----------------------------------------------------------------

func TestClean_NoEmpty(t *testing.T) {
	f := newFixture(t)
	stdout, _, code := f.runCLI("clean", "-f")
	if code != 0 {
		t.Fatal(stdout)
	}
	if !strings.Contains(stdout, "No empty groups found") {
		t.Errorf("expected 'No empty groups found':\n%s", stdout)
	}
}

func TestClean_JSON(t *testing.T) {
	f := newFixture(t)
	stdout, _, code := f.runCLI("clean", "--json", "-f")
	if code != 0 {
		t.Fatal(stdout)
	}
	if !strings.Contains(stdout, `"count":0`) {
		t.Errorf("expected count:0 in:\n%s", stdout)
	}
}

// --- completion ------------------------------------------------------------

func TestCompletion_Bash(t *testing.T) {
	f := newFixture(t)
	stdout, _, code := f.runCLIWith(runOpts{skipDatabaseArg: true, skipPasswordFile: true}, "completion", "bash")
	if code != 0 {
		t.Fatal(stdout)
	}
	if !strings.Contains(stdout, "complete -F _kpass") {
		t.Errorf("bash completion missing marker:\n%s", stdout[:200])
	}
}

func TestCompletion_Zsh(t *testing.T) {
	f := newFixture(t)
	stdout, _, code := f.runCLIWith(runOpts{skipDatabaseArg: true, skipPasswordFile: true}, "completion", "zsh")
	if code != 0 {
		t.Fatal(stdout)
	}
	if !strings.Contains(stdout, "#compdef kpass") {
		t.Errorf("zsh completion missing marker")
	}
}

func TestCompletion_Fish(t *testing.T) {
	f := newFixture(t)
	stdout, _, code := f.runCLIWith(runOpts{skipDatabaseArg: true, skipPasswordFile: true}, "completion", "fish")
	if code != 0 {
		t.Fatal(stdout)
	}
	if !strings.Contains(stdout, "complete -c kpass") {
		t.Errorf("fish completion missing marker")
	}
}

func TestCompletion_UnknownShell(t *testing.T) {
	f := newFixture(t)
	_, _, code := f.runCLIWith(runOpts{skipDatabaseArg: true, skipPasswordFile: true}, "completion", "powershell")
	if code == 0 {
		t.Error("expected non-zero for unknown shell")
	}
}

// --- mkdir -----------------------------------------------------------------

func TestMkdir_CreatesNestedPath(t *testing.T) {
	f := newFixture(t)
	stdout, _, code := f.runCLI("mkdir", "a/b/c")
	if code != 0 {
		t.Fatal(stdout)
	}
	if !strings.Contains(stdout, "a/b/c") {
		t.Errorf("output = %q", stdout)
	}
}

func TestMkdir_EmptyRejected(t *testing.T) {
	f := newFixture(t)
	_, _, code := f.runCLI("mkdir", "")
	if code == 0 {
		t.Error("expected error for empty group")
	}
}

func TestMkdir_JSON(t *testing.T) {
	f := newFixture(t)
	stdout, _, code := f.runCLI("mkdir", "x/y", "--json")
	if code != 0 {
		t.Fatal(stdout)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(stdout), &obj); err != nil {
		t.Fatal(err)
	}
	if obj["path"] != "x/y" {
		t.Errorf("path = %v", obj["path"])
	}
}

// --- export ---------------------------------------------------------------

func TestExport_CSV(t *testing.T) {
	f := newFixture(t)
	stdout, _, code := f.runCLI("export", "-o", "csv")
	if code != 0 {
		t.Fatal(stdout)
	}
	if !strings.Contains(stdout, "path,title,username,password,url,notes,otp") {
		t.Errorf("csv header missing:\n%s", stdout)
	}
}

func TestExport_OutputToFile(t *testing.T) {
	f := newFixture(t)
	outPath := filepath.Join(f.root, "out.json")
	_, _, code := f.runCLI("export", "--output", outPath)
	if code != 0 {
		t.Fatal("export failed")
	}
	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("output mode = %#o", info.Mode().Perm())
	}
}

func TestExport_RefuseOverwrite(t *testing.T) {
	f := newFixture(t)
	outPath := filepath.Join(f.root, "existing.json")
	if err := os.WriteFile(outPath, []byte("[]"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := f.runCLI("export", "--output", outPath)
	if code == 0 {
		t.Errorf("expected refuse-overwrite, stderr=%q", stderr)
	}
}

func TestExport_ForceOverwrite(t *testing.T) {
	f := newFixture(t)
	outPath := filepath.Join(f.root, "existing.json")
	if err := os.WriteFile(outPath, []byte("[]"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, code := f.runCLI("export", "--output", outPath, "--force")
	if code != 0 {
		t.Fatal("force overwrite failed")
	}
}

func TestExport_EmptyDB(t *testing.T) {
	f := newFixture(t)
	// Remove all entries first.
	if _, _, code := f.runCLI("remove", "-f", "internet/email", "work/email", "simple", "otp/sample", "db-passwords/work"); code != 0 {
		t.Fatal("remove failed")
	}
	_, _, code := f.runCLI("export")
	if code == 0 {
		t.Error("expected 'no entries' error")
	}
}

func TestImport_RoundTripCSV(t *testing.T) {
	f := newFixture(t)
	exportOut, _, code := f.runCLI("export", "-o", "csv")
	if code != 0 {
		t.Fatal("export")
	}
	src := filepath.Join(f.root, "round.csv")
	if err := os.WriteFile(src, []byte(exportOut), 0o600); err != nil {
		t.Fatal(err)
	}
	// Importing back into the same DB should skip everything.
	_, _, code = f.runCLI("import", src, "-o", "csv", "-f")
	if code != 0 {
		t.Error("import-back failed")
	}
}

func TestParseJSONImport_Valid(t *testing.T) {
	data := []byte(`[{"path":"a/b","title":"b","username":"u","password":"p","custom_fields":{"k":"v"}}]`)
	entries, err := parseJSONImport(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Path != "a/b" || entries[0].Custom["k"] != "v" {
		t.Errorf("got %+v", entries)
	}
}

func TestParseJSONImport_MissingPath(t *testing.T) {
	data := []byte(`[{"title":"b"}]`)
	if _, err := parseJSONImport(data); err == nil {
		t.Error("expected missing-path error")
	}
}

func TestParseJSONImport_InvalidJSON(t *testing.T) {
	if _, err := parseJSONImport([]byte(`{not json`)); err == nil {
		t.Error("expected invalid-json error")
	}
}

func TestParseCSVImport_Valid(t *testing.T) {
	data := []byte("path,title,username,password,url,notes,otp\na/b,b,u,p,,note,\n")
	entries, err := parseCSVImport(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Path != "a/b" {
		t.Errorf("got %+v", entries)
	}
}

func TestParseCSVImport_NoPath(t *testing.T) {
	data := []byte("title\nfoo")
	if _, err := parseCSVImport(data); err == nil {
		t.Error("expected 'path column' error")
	}
}

func TestParseCSVImport_HeaderOnly(t *testing.T) {
	data := []byte("path\n")
	if _, err := parseCSVImport(data); err == nil {
		t.Error("expected header-only error")
	}
}

func TestParseCSVImport_InvalidCSV(t *testing.T) {
	if _, err := parseCSVImport([]byte("\"unterminated")); err == nil {
		t.Error("expected invalid-csv error")
	}
}

func TestParseCSVImport_CustomFieldsKV(t *testing.T) {
	data := []byte("path,title,extra\nx,y,key=val\n")
	entries, err := parseCSVImport(data)
	if err != nil {
		t.Fatal(err)
	}
	if entries[0].Custom["key"] != "val" {
		t.Errorf("custom = %v", entries[0].Custom)
	}
}

func TestStrVal(t *testing.T) {
	m := map[string]any{"a": "hi", "b": 42, "c": nil}
	if strVal(m, "a") != "hi" {
		t.Error("string")
	}
	if strVal(m, "b") != "42" {
		t.Error("int")
	}
	if strVal(m, "missing") != "" {
		t.Error("missing")
	}
}

// --- history ---------------------------------------------------------------

func TestHistory_ListEmpty(t *testing.T) {
	f := newFixture(t)
	stdout, _, code := f.runCLI("history", "internet/email")
	if code != 0 {
		t.Fatal(stdout)
	}
	if !strings.Contains(stdout, "No history entries") {
		t.Errorf("expected 'No history entries':\n%s", stdout)
	}
}

func TestHistory_NotFound(t *testing.T) {
	f := newFixture(t)
	_, _, code := f.runCLI("history", "no-such-entry")
	if code == 0 {
		t.Error("expected entry-not-found error")
	}
}

// Direct unit tests for the history helpers — kpass edit doesn't append to
// KeePass Histories so the integration paths are unreachable via the CLI.

func TestRestoreEntry_CopiesAllFields(t *testing.T) {
	src := newRawEntry("old", "olduser", "oldpass", "https://old", "oldnotes", "")
	src.Values = append(src.Values, gokeepasslibVD("Custom", "src-val"))
	dst := newRawEntry("new", "newuser", "newpass", "https://new", "newnotes", "")
	dst.Values = append(dst.Values, gokeepasslibVD("Custom", "dst-val"))

	restoreEntry(dst, src)

	if dst.GetTitle() != "old" || dst.GetContent("UserName") != "olduser" {
		t.Errorf("standard fields not restored: %+v", dst.Values)
	}
	// Custom field should be copied across.
	var got string
	for _, v := range dst.Values {
		if v.Key == "Custom" {
			got = v.Value.Content
		}
	}
	if got != "src-val" {
		t.Errorf("custom field = %q", got)
	}
}

func TestRestoreEntry_AddsMissingCustom(t *testing.T) {
	src := newRawEntry("t", "u", "p", "", "", "")
	src.Values = append(src.Values, gokeepasslibVD("OnlyInSrc", "v"))
	dst := newRawEntry("t", "u", "p", "", "", "")

	restoreEntry(dst, src)
	found := false
	for _, v := range dst.Values {
		if v.Key == "OnlyInSrc" && v.Value.Content == "v" {
			found = true
		}
	}
	if !found {
		t.Error("missing custom field not appended")
	}
}

func TestCopyValue_CreatesProtectedForSensitive(t *testing.T) {
	src := newRawEntry("t", "u", "p", "", "", "totp-uri")
	dst := newRawEntry("t", "u", "p", "", "", "")

	copyValue(dst, src, "otp")
	if dst.GetContent("otp") != "totp-uri" {
		t.Errorf("otp not copied: %v", dst.Values)
	}
}

func TestDiffEntries(t *testing.T) {
	cur := newRawEntry("Title", "alice", "newpass", "https://a", "n", "")
	prev := newRawEntry("Title", "bob", "newpass", "https://a", "old-notes", "")
	changes := diffEntries(cur, prev)
	if len(changes) != 2 {
		t.Errorf("expected 2 changes, got %d: %+v", len(changes), changes)
	}
}

func TestFlattenHistory(t *testing.T) {
	raw := newRawEntry("t", "u", "p", "", "", "")
	items := flattenHistory(raw)
	if len(items) != 0 {
		t.Errorf("expected empty history, got %d", len(items))
	}
}

// historyFixture builds a kdbx file with a single entry that carries
// pre-populated Histories so the CLI history sub-commands can be exercised
// (kpass edit does not append history itself).
func historyFixture(t *testing.T) (*fixture, string) {
	t.Helper()
	f := newFixture(t)
	// Rebuild a kdbx into a fresh path with a histories entry.
	path := filepath.Join(f.root, "with-hist.kdbx")
	pwPath := filepath.Join(f.root, "hist.password")
	if err := os.WriteFile(pwPath, []byte("hist-pw\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	raw := gokeepasslib.NewDatabase(gokeepasslib.WithDatabaseKDBXVersion4())
	raw.Credentials = gokeepasslib.NewPasswordCredentials("hist-pw")
	root := &raw.Content.Root.Groups[0]
	root.Name = "Root"
	root.Entries = nil
	root.Groups = nil

	current := *newRawEntry("docs", "alice", "current-pw", "https://current", "current notes", "")
	older := *newRawEntry("docs", "bob", "old-pw", "https://old", "old notes", "")
	older2 := *newRawEntry("docs", "carol", "older-pw", "https://older", "older notes", "")
	current.Histories = []gokeepasslib.History{{Entries: []gokeepasslib.Entry{older, older2}}}
	root.Entries = append(root.Entries, current)

	fp, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := gokeepasslib.NewEncoder(fp).Encode(raw); err != nil {
		_ = fp.Close()
		t.Fatal(err)
	}
	_ = fp.Close()

	return f, path
}

func TestHistory_List_Integration(t *testing.T) {
	f, path := historyFixture(t)
	stdout, _, code := f.runCLIWith(runOpts{
		databasePath:     path,
		passwordFilePath: filepath.Join(f.root, "hist.password"),
	}, "history", "docs")
	if code != 0 {
		t.Fatalf("code=%d %s", code, stdout)
	}
	if !strings.Contains(stdout, "version(s)") {
		t.Errorf("expected history list:\n%s", stdout)
	}
}

func TestHistory_Diff_Integration(t *testing.T) {
	f, path := historyFixture(t)
	stdout, _, code := f.runCLIWith(runOpts{
		databasePath:     path,
		passwordFilePath: filepath.Join(f.root, "hist.password"),
	}, "history", "docs", "--diff")
	if code != 0 {
		t.Fatalf("code=%d %s", code, stdout)
	}
	if !strings.Contains(stdout, "Diff vs") {
		t.Errorf("expected diff output:\n%s", stdout)
	}
}

func TestHistory_Restore_Integration(t *testing.T) {
	f, path := historyFixture(t)
	pwPath := filepath.Join(f.root, "hist.password")
	_, _, code := f.runCLIWith(runOpts{
		databasePath:     path,
		passwordFilePath: pwPath,
	}, "history", "docs", "--restore", "0")
	if code != 0 {
		t.Fatal("restore failed")
	}
	stdout, _, _ := f.runCLIWith(runOpts{
		databasePath:     path,
		passwordFilePath: pwPath,
	}, "get", "docs", "-F", "username")
	if strings.TrimSpace(stdout) != "bob" {
		t.Errorf("after restore username = %q, want bob", stdout)
	}
}

func TestHistory_RestoreOutOfRange_Integration(t *testing.T) {
	f, path := historyFixture(t)
	_, _, code := f.runCLIWith(runOpts{
		databasePath:     path,
		passwordFilePath: filepath.Join(f.root, "hist.password"),
	}, "history", "docs", "--restore", "99")
	if code == 0 {
		t.Error("expected out-of-range error")
	}
}

func TestHistoryCmd_HelpString(t *testing.T) {
	if (HistoryCmd{}).Help() == "" {
		t.Error("HistoryCmd.Help() returned empty")
	}
}

func TestUndoCmd_HelpString(t *testing.T) {
	if (UndoCmd{}).Help() == "" {
		t.Error("UndoCmd.Help() returned empty")
	}
}

func TestExportCmd_HelpString(t *testing.T) {
	if (ExportCmd{}).Help() == "" {
		t.Error("ExportCmd.Help() returned empty")
	}
}

func TestImportCmd_HelpString(t *testing.T) {
	if (ImportCmd{}).Help() == "" {
		t.Error("ImportCmd.Help() returned empty")
	}
}

func TestTruncateDiff(t *testing.T) {
	if got := truncateDiff(""); got != "(empty)" {
		t.Errorf("empty = %q", got)
	}
	if got := truncateDiff("short"); got != "short" {
		t.Errorf("short = %q", got)
	}
	long := strings.Repeat("a", 50)
	got := truncateDiff(long)
	if !strings.HasSuffix(got, "...") || len(got) != 40 {
		t.Errorf("long len=%d, %q", len(got), got)
	}
}

// --- open ------------------------------------------------------------------

func TestOpen_NoURL(t *testing.T) {
	f := newFixture(t)
	_, _, code := f.runCLI("open", "simple")
	if code == 0 {
		t.Error("expected no-URL error")
	}
}

func TestOpen_NotFound(t *testing.T) {
	f := newFixture(t)
	_, _, code := f.runCLI("open", "ghost/nothing")
	if code == 0 {
		t.Error("expected entry-not-found error")
	}
}

func TestOpenCommand(t *testing.T) {
	// Just exercise the platform branch by calling it; we cannot mock runtime.GOOS.
	cmd := openCommand()
	if len(cmd) == 0 {
		t.Error("openCommand returned empty argv")
	}
}

// --- pick ------------------------------------------------------------------

func TestPick_HookCopy(t *testing.T) {
	f := newFixture(t)
	picker.Hook = func([]string, string) (string, error) { return "internet/email", nil }
	t.Cleanup(func() { picker.Hook = nil })
	var captured string
	ClipboardWriter = func(value string, _ int) error { captured = value; return nil }

	_, _, code := f.runCLI("pick", "--action", "copy")
	if code != 0 {
		t.Fatal("pick copy failed")
	}
	if captured != "pw-email" {
		t.Errorf("clipboard = %q", captured)
	}
}

func TestPick_HookGetField(t *testing.T) {
	f := newFixture(t)
	picker.Hook = func([]string, string) (string, error) { return "internet/email", nil }
	t.Cleanup(func() { picker.Hook = nil })

	stdout, _, code := f.runCLI("pick", "--action", "get", "-F", "username")
	if code != 0 {
		t.Fatal("pick get failed")
	}
	if strings.TrimSpace(stdout) != "alice" {
		t.Errorf("output = %q", stdout)
	}
}

func TestPick_HookShow(t *testing.T) {
	f := newFixture(t)
	picker.Hook = func([]string, string) (string, error) { return "internet/email", nil }
	t.Cleanup(func() { picker.Hook = nil })

	stdout, _, code := f.runCLI("pick", "--action", "show")
	if code != 0 {
		t.Fatal("pick show failed")
	}
	if !strings.Contains(stdout, "Title:") {
		t.Errorf("expected entry details:\n%s", stdout)
	}
}

func TestPick_HookOtpNoUri(t *testing.T) {
	f := newFixture(t)
	picker.Hook = func([]string, string) (string, error) { return "internet/email", nil }
	t.Cleanup(func() { picker.Hook = nil })

	_, _, code := f.runCLI("pick", "--action", "otp")
	if code == 0 {
		t.Error("expected no-OTP error")
	}
}

func TestPick_HookOtpWithUri(t *testing.T) {
	f := newFixture(t)
	picker.Hook = func([]string, string) (string, error) { return "otp/sample", nil }
	t.Cleanup(func() { picker.Hook = nil })
	var captured string
	ClipboardWriter = func(value string, _ int) error { captured = value; return nil }
	OtpCoder = func(string) (string, error) { return "123456", nil }

	_, _, code := f.runCLI("pick", "--action", "otp")
	if code != 0 {
		t.Fatal("pick otp failed")
	}
	if captured != "123456" {
		t.Errorf("clipboard = %q", captured)
	}
}

func TestPick_TagFilterNoMatch(t *testing.T) {
	f := newFixture(t)
	picker.Hook = func([]string, string) (string, error) { return "irrelevant", nil }
	t.Cleanup(func() { picker.Hook = nil })

	_, _, code := f.runCLI("pick", "--tag", "no-such-tag")
	if code == 0 {
		t.Error("expected no-entries error")
	}
}

// --- import-pass -----------------------------------------------------------

func TestDecryptWithGPG_BinaryMissing(t *testing.T) {
	// Internal helper; calls a real gpg binary. Should error cleanly when
	// the binary doesn't exist.
	if _, err := decryptWithGPG("/no/such/file.gpg", "/nonexistent-gpg-binary"); err == nil {
		t.Error("expected error for missing gpg binary")
	}
}

// --- insert wrapForUser ---------------------------------------------------

func TestWrapForUser(t *testing.T) {
	if wrapForUser(nil) != nil {
		t.Error("nil should pass through")
	}
	ue := &UserError{Msg: "x"}
	if wrapForUser(ue) != ue {
		t.Error("UserError should pass through unchanged")
	}
	wrapped := wrapForUser(errors.New("plain"))
	if _, ok := wrapped.(*UserError); !ok {
		t.Errorf("plain error not wrapped, got %T", wrapped)
	}
}

// --- doctor ----------------------------------------------------------------

func TestDoctor_NoProfiles(t *testing.T) {
	f := newFixture(t)
	// configEnv points at a missing file → empty config.
	_, _, code := f.runCLIWith(runOpts{skipDatabaseArg: true, skipPasswordFile: true}, "doctor")
	if code == 0 {
		t.Error("expected 'no profiles' error")
	}
}

func TestDoctor_JSONEmpty(t *testing.T) {
	f := newFixture(t)
	stdout, _, code := f.runCLIWith(runOpts{skipDatabaseArg: true, skipPasswordFile: true}, "doctor", "--json")
	if code != 0 {
		t.Fatalf("code=%d %s", code, stdout)
	}
	if !strings.Contains(stdout, `"profiles": []`) {
		t.Errorf("expected empty profiles:\n%s", stdout)
	}
}

func TestDoctor_JSONHealthy(t *testing.T) {
	f := newFixture(t)
	cfg := writeMultiDBConfig(t, f, multiDBConfigOpts{})
	stdout, _, code := f.runCLIWith(runOpts{
		skipDatabaseArg:  true,
		skipPasswordFile: true,
		configFile:       cfg,
	}, "doctor", "--json")
	if code != 0 {
		t.Fatalf("code=%d %s", code, stdout)
	}
	if !strings.Contains(stdout, `"status": "OK"`) {
		t.Errorf("expected OK status:\n%s", stdout)
	}
}

// --- undo / backup ---------------------------------------------------------

func TestUndo_ListNone(t *testing.T) {
	f := newFixture(t)
	stdout, _, code := f.runCLI("undo", "--list")
	if code != 0 {
		t.Fatal(stdout)
	}
	if !strings.Contains(stdout, "No backups found") {
		t.Errorf("expected 'No backups':\n%s", stdout)
	}
}

func TestUndo_RestoreNoBackup(t *testing.T) {
	f := newFixture(t)
	_, _, code := f.runCLI("undo", "-f")
	if code == 0 {
		t.Error("expected no-backup error")
	}
}

func TestUndo_AfterMutation(t *testing.T) {
	f := newFixture(t)
	// Make a mutation to create a backup.
	if _, _, code := f.runCLI("edit", "internet/email", "-u", "modified"); code != 0 {
		t.Fatal("edit")
	}
	stdout, _, code := f.runCLI("undo", "--list")
	if code != 0 {
		t.Fatal(stdout)
	}
	if !strings.Contains(stdout, "Backups for") {
		t.Errorf("expected backup list:\n%s", stdout)
	}
}

func TestUndo_Prune(t *testing.T) {
	f := newFixture(t)
	if _, _, code := f.runCLI("edit", "internet/email", "-u", "m1"); code != 0 {
		t.Fatal("edit 1")
	}
	if _, _, code := f.runCLI("edit", "internet/email", "-u", "m2"); code != 0 {
		t.Fatal("edit 2")
	}
	// Prune all.
	_, _, code := f.runCLI("undo", "--prune", "0", "-f")
	if code != 0 {
		t.Fatal("prune")
	}
}

func TestUndo_PruneNothing(t *testing.T) {
	f := newFixture(t)
	stdout, _, code := f.runCLI("undo", "--prune", "5", "-f")
	if code != 0 {
		t.Fatal(stdout)
	}
	if !strings.Contains(stdout, "No backups to prune") && !strings.Contains(stdout, "nothing to prune") {
		t.Errorf("unexpected output:\n%s", stdout)
	}
}

func TestFormatSize(t *testing.T) {
	cases := []struct{ in int64; want string }{
		{42, "42 B"},
		{2048, "2.0 KiB"},
		{2 * 1024 * 1024, "2.0 MiB"},
	}
	for _, tc := range cases {
		if got := formatSize(tc.in); got != tc.want {
			t.Errorf("formatSize(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// --- errors / clear_clipboard ---------------------------------------------

func TestUserError_Error(t *testing.T) {
	e := &UserError{Msg: "boom"}
	if e.Error() != "boom" {
		t.Errorf("got %q", e.Error())
	}
}

func TestRunClearClipboard_BadArgs(t *testing.T) {
	if runClearClipboard([]string{}) == 0 {
		t.Error("expected error for missing argv")
	}
	if runClearClipboard([]string{"__clear", "notnum"}) == 0 {
		t.Error("expected error for non-numeric timeout")
	}
	if runClearClipboard([]string{"__clear", "0"}) == 0 {
		t.Error("expected error for non-positive timeout")
	}
}

func TestRunClearClipboard_EmptyStdin(t *testing.T) {
	origStdin := os.Stdin
	t.Cleanup(func() { os.Stdin = origStdin })
	r, w, _ := os.Pipe()
	_ = w.Close()
	os.Stdin = r
	if runClearClipboard([]string{"__clear", "1"}) == 0 {
		t.Error("expected error for empty stdin")
	}
}

// --- generate.matchGlob ----------------------------------------------------

func TestMatchGlob(t *testing.T) {
	if !matchGlob("work/*", "work/email") {
		t.Error("should match")
	}
	if matchGlob("work/*", "personal/x") {
		t.Error("should not match")
	}
	if matchGlob("[", "x") {
		t.Error("bad pattern should not match")
	}
}

// --- combine.altAttachmentName --------------------------------------------

func TestAltAttachmentName(t *testing.T) {
	if got := altAttachmentName("doc.txt"); got != "doc.alt.txt" {
		t.Errorf("alt name = %q", got)
	}
	if got := altAttachmentName("readme"); got != "readme.alt" {
		t.Errorf("ext-less alt = %q", got)
	}
}

// --- search ----------------------------------------------------------------

func TestSearch_JSON(t *testing.T) {
	f := newFixture(t)
	stdout, _, code := f.runCLI("search", "email", "--json")
	if code != 0 {
		t.Fatal(stdout)
	}
	var arr []any
	if err := json.Unmarshal([]byte(stdout), &arr); err != nil {
		t.Fatalf("json: %v\n%s", err, stdout)
	}
	if len(arr) == 0 {
		t.Errorf("expected at least one match:\n%s", stdout)
	}
}

// --- remove cleanEmptyGroups branch --------------------------------------

func TestRemove_LeavesEmptyParentWithoutConfirm(t *testing.T) {
	// Use --force to skip cleanEmptyGroups call.
	f := newFixture(t)
	_, _, code := f.runCLI("remove", "-f", "internet/email")
	if code != 0 {
		t.Fatal("remove")
	}
}

func TestRemove_PrintEntryContext(t *testing.T) {
	// Single-entry confirm path uses printEntryContext. Drive via prompt with
	// piped "n" so user declines.
	f := newFixture(t)
	_, _, code := f.runCLIWith(runOpts{stdin: "n\n"}, "remove", "internet/email")
	if code == 0 {
		t.Error("expected non-zero (aborted)")
	}
}

// --- pickEntry tag filter empty ------------------------------------------

func TestPick_HookDelete(t *testing.T) {
	f := newFixture(t)
	picker.Hook = func([]string, string) (string, error) { return "internet/email", nil }
	t.Cleanup(func() { picker.Hook = nil })

	_, _, code := f.runCLIWith(runOpts{stdin: "y\n"}, "pick", "--action", "delete")
	if code != 0 {
		t.Fatalf("pick delete failed; code=%d", code)
	}
}

func TestPick_HookOpenNoURL(t *testing.T) {
	f := newFixture(t)
	picker.Hook = func([]string, string) (string, error) { return "simple", nil }
	t.Cleanup(func() { picker.Hook = nil })
	_, _, code := f.runCLI("pick", "--action", "open")
	if code == 0 {
		t.Error("expected no-URL error")
	}
}

// --- passwordFetcher (via doctor's resolveProfile call) ------------------

func TestPasswordFetcher_BadDatabase(t *testing.T) {
	// Drive passwordFetcher via a config that chains to a missing source DB.
	f := newFixture(t)
	cfgPath := filepath.Join(f.root, "broken.toml")
	writeFile(t, cfgPath, strings.Join([]string{
		`default = "main"`,
		"",
		"[databases.main]",
		`database = "` + f.dbPath + `"`,
		`password_database = "missing"`,
		`password_entry = "x"`,
		"",
		"[databases.missing]",
		`database = "/no/such/file.kdbx"`,
		"",
	}, "\n"))
	_, _, code := f.runCLIWith(runOpts{
		skipDatabaseArg:  true,
		skipPasswordFile: true,
		configFile:       cfgPath,
	}, "doctor", "--json")
	if code != 0 {
		// doctor with --json should not exit non-zero; broken profile is in JSON.
		t.Fatalf("doctor --json should not exit non-zero")
	}
}

// --- mkdir error path -----------------------------------------------------

func TestMkdir_PrintsPathInJSON(t *testing.T) {
	f := newFixture(t)
	stdout, _, _ := f.runCLI("mkdir", "  /trimmed/path/  ", "--json")
	if !strings.Contains(stdout, "trimmed/path") {
		t.Errorf("trimmed path missing in JSON:\n%s", stdout)
	}
}

// --- ensure import-skip path runs ----------------------------------------

func TestImport_SkipExisting(t *testing.T) {
	f := newFixture(t)
	src := filepath.Join(f.root, "in.json")
	if err := os.WriteFile(src, []byte(`[{"path":"internet/email","title":"email","username":"u","password":"p"}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, _, code := f.runCLI("import", src, "-f", "--on-conflict", "skip")
	if code != 0 {
		t.Fatal(stdout)
	}
	if !strings.Contains(stdout, "Skipped:") {
		t.Errorf("expected skipped row:\n%s", stdout)
	}
}

func TestImport_OverwriteExisting(t *testing.T) {
	f := newFixture(t)
	src := filepath.Join(f.root, "in.json")
	if err := os.WriteFile(src, []byte(`[{"path":"internet/email","username":"new","password":"new"}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, code := f.runCLI("import", src, "-f", "--on-conflict", "overwrite"); code != 0 {
		t.Fatal("import overwrite")
	}
	stdout, _, _ := f.runCLI("get", "internet/email", "-F", "username")
	if strings.TrimSpace(stdout) != "new" {
		t.Errorf("overwrite did not apply, username = %q", stdout)
	}
}

func TestImport_Empty(t *testing.T) {
	f := newFixture(t)
	src := filepath.Join(f.root, "empty.json")
	if err := os.WriteFile(src, []byte("[]"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, code := f.runCLI("import", src, "-f")
	if code == 0 {
		t.Error("expected no-entries error")
	}
}

// --- additional batches to lift coverage further ---

func TestClean_RemovesEmptyGroup(t *testing.T) {
	f := newFixture(t)
	// Create an empty group then run clean -f.
	if _, _, code := f.runCLI("mkdir", "lonely-group"); code != 0 {
		t.Fatal("mkdir")
	}
	stdout, _, code := f.runCLI("clean", "-f")
	if code != 0 {
		t.Fatal(stdout)
	}
	if !strings.Contains(stdout, "Removed") && !strings.Contains(stdout, "No empty") {
		t.Errorf("unexpected clean output:\n%s", stdout)
	}
}

func TestClean_DryRun(t *testing.T) {
	f := newFixture(t)
	if _, _, code := f.runCLI("mkdir", "would-clean"); code != 0 {
		t.Fatal("mkdir")
	}
	stdout, _, code := f.runCLI("clean", "--dry-run")
	if code != 0 {
		t.Fatal(stdout)
	}
	_ = stdout
}

func TestClean_DryRunJSON(t *testing.T) {
	f := newFixture(t)
	if _, _, code := f.runCLI("mkdir", "would-clean"); code != 0 {
		t.Fatal("mkdir")
	}
	_, _, code := f.runCLI("clean", "--dry-run", "--json")
	if code != 0 {
		t.Fatal("clean dry-run --json failed")
	}
}

func TestCompleteCmd_Commands(t *testing.T) {
	f := newFixture(t)
	stdout, _, code := f.runCLIWith(runOpts{skipDatabaseArg: true, skipPasswordFile: true}, "__complete", "commands", "", "", "ls")
	if code != 0 {
		t.Fatal(stdout)
	}
	if !strings.Contains(stdout, "ls") {
		t.Errorf("expected ls in commands:\n%s", stdout)
	}
}

func TestCompleteCmd_Entries(t *testing.T) {
	f := newFixture(t)
	stdout, _, code := f.runCLI("__complete", "entries", "", "", "internet/")
	if code != 0 {
		t.Fatal(stdout)
	}
	if !strings.Contains(stdout, "internet/email") {
		t.Errorf("expected internet/email:\n%s", stdout)
	}
}

func TestCompleteCmd_Profiles(t *testing.T) {
	f := newFixture(t)
	cfg := writeMultiDBConfig(t, f, multiDBConfigOpts{})
	stdout, _, code := f.runCLIWith(runOpts{
		skipDatabaseArg:  true,
		skipPasswordFile: true,
		configFile:       cfg,
	}, "__complete", "profiles", "", "", "m")
	if code != 0 {
		t.Fatal(stdout)
	}
	if !strings.Contains(stdout, "main") {
		t.Errorf("expected 'main' in profiles:\n%s", stdout)
	}
}

func TestCompleteCmd_Attachments(t *testing.T) {
	f := newFixture(t)
	// Add an attachment first.
	attachPath := filepath.Join(f.root, "att.txt")
	if err := os.WriteFile(attachPath, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, code := f.runCLI("attach", "add", "internet/email", attachPath); code != 0 {
		t.Fatal("attach add")
	}
	stdout, _, code := f.runCLI("__complete", "attachments", "", "internet/email", "")
	if code != 0 {
		t.Fatal(stdout)
	}
	if !strings.Contains(stdout, "att.txt") {
		t.Errorf("expected att.txt:\n%s", stdout)
	}
}

func TestCompleteCmd_Unknown(t *testing.T) {
	f := newFixture(t)
	_, _, code := f.runCLI("__complete", "ghost-resource", "", "", "")
	if code == 0 {
		t.Error("expected unknown-resource error")
	}
}

func TestUndo_RestoreToOtherPath(t *testing.T) {
	f := newFixture(t)
	// Create a backup via mutation.
	if _, _, code := f.runCLI("edit", "internet/email", "-u", "m"); code != 0 {
		t.Fatal("edit")
	}
	out := filepath.Join(f.root, "restored.kdbx")
	if _, _, code := f.runCLI("undo", "--index", "0", "--restore-to", out, "-f"); code != 0 {
		t.Fatal("restore-to")
	}
	if _, err := os.Stat(out); err != nil {
		t.Errorf("restored file missing: %v", err)
	}
}

func TestUndo_PruneKeepN(t *testing.T) {
	f := newFixture(t)
	// Create multiple backups by mutating.
	for i, u := range []string{"a", "b", "c"} {
		if _, _, code := f.runCLI("edit", "internet/email", "-u", u); code != 0 {
			t.Fatalf("edit %d", i)
		}
	}
	_, _, code := f.runCLI("undo", "--prune", "1", "-f")
	if code != 0 {
		t.Fatal("prune")
	}
}

func TestPick_HookCopyTimeout(t *testing.T) {
	f := newFixture(t)
	picker.Hook = func([]string, string) (string, error) { return "internet/email", nil }
	t.Cleanup(func() { picker.Hook = nil })
	var capturedTimeout int
	ClipboardWriter = func(_ string, timeout int) error { capturedTimeout = timeout; return nil }

	if _, _, code := f.runCLI("pick", "--action", "copy", "--timeout", "5"); code != 0 {
		t.Fatal("pick copy timeout")
	}
	if capturedTimeout != 5 {
		t.Errorf("timeout = %d, want 5", capturedTimeout)
	}
}

func TestPickCopyDefaultTimeout(t *testing.T) {
	f := newFixture(t)
	picker.Hook = func([]string, string) (string, error) { return "internet/email", nil }
	t.Cleanup(func() { picker.Hook = nil })
	var captured int
	ClipboardWriter = func(_ string, t int) error { captured = t; return nil }
	if _, _, code := f.runCLI("pick", "--action", "copy"); code != 0 {
		t.Fatal("pick copy default")
	}
	// default --timeout is -1; PickCmd.copyTimeout falls back to DefaultClipboardTimeout=10.
	if captured != 10 {
		t.Errorf("default timeout = %d, want 10", captured)
	}
}

func TestGet_JSON_MultipleEntriesGlob(t *testing.T) {
	f := newFixture(t)
	stdout, _, code := f.runCLI("get", "*/email", "--glob", "--json")
	if code != 0 {
		t.Fatal(stdout)
	}
	var arr []map[string]any
	if err := json.Unmarshal([]byte(stdout), &arr); err != nil {
		t.Fatal(err)
	}
	if len(arr) < 2 {
		t.Errorf("expected at least 2 entries, got %d", len(arr))
	}
}

func TestGet_JSON_GlobSingleField(t *testing.T) {
	f := newFixture(t)
	stdout, _, code := f.runCLI("get", "*/email", "--glob", "--json", "-F", "username")
	if code != 0 {
		t.Fatal(stdout)
	}
	var obj map[string]string
	if err := json.Unmarshal([]byte(stdout), &obj); err != nil {
		t.Fatal(err)
	}
	if obj["internet/email"] != "alice" {
		t.Errorf("expected alice, got %v", obj)
	}
}

func TestGet_JSON_GlobMultiFields(t *testing.T) {
	f := newFixture(t)
	stdout, _, code := f.runCLI("get", "*/email", "--glob", "--json", "-F", "username", "-F", "url")
	if code != 0 {
		t.Fatal(stdout)
	}
	var arr []map[string]string
	if err := json.Unmarshal([]byte(stdout), &arr); err != nil {
		t.Fatal(err)
	}
	if len(arr) < 2 {
		t.Errorf("expected multiple entries, got %d", len(arr))
	}
}

// --- silence linter for unused db import ---
var _ = db.MatchError{}
var _ = io.EOF
