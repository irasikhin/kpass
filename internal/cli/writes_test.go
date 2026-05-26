package cli

import (
	"regexp"
	"strings"
	"testing"

	"github.com/tobischo/gokeepasslib/v3"

	"github.com/irasikhin/kpass/internal/editor"
)

// findEntryByPath walks the decoded DB and returns the entry with the given
// display path (split on "/"), or nil.
func findEntryByPath(db *gokeepasslib.Database, path string) *gokeepasslib.Entry {
	parts := strings.Split(path, "/")
	root := &db.Content.Root.Groups[0]
	return findInGroup(root, parts)
}

func findInGroup(g *gokeepasslib.Group, parts []string) *gokeepasslib.Entry {
	if len(parts) == 0 {
		return nil
	}
	if len(parts) == 1 {
		for i := range g.Entries {
			if g.Entries[i].GetTitle() == parts[0] {
				return &g.Entries[i]
			}
		}
		return nil
	}
	for i := range g.Groups {
		if g.Groups[i].Name == parts[0] {
			if e := findInGroup(&g.Groups[i], parts[1:]); e != nil {
				return e
			}
		}
	}
	return nil
}

func TestInsertCreatesEntryAndParentGroups(t *testing.T) {
	f := newFixture(t)
	stdout, stderr, code := f.runCLI("insert", "new/services/github",
		"--username", "bob",
		"--password", "super-secret",
		"--url", "https://github.com",
		"--notes", "note",
	)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if strings.TrimSpace(stdout) != "new/services/github" {
		t.Fatalf("stdout=%q", stdout)
	}
	db := openSeededDB(t, f.dbPath, "master-password")
	e := findEntryByPath(db, "new/services/github")
	if e == nil {
		t.Fatal("entry missing")
	}
	if e.GetContent("UserName") != "bob" {
		t.Fatalf("username=%q", e.GetContent("UserName"))
	}
	if e.GetPassword() != "super-secret" {
		t.Fatalf("password=%q", e.GetPassword())
	}
	if e.GetContent("URL") != "https://github.com" {
		t.Fatalf("url=%q", e.GetContent("URL"))
	}
	if e.GetContent("Notes") != "note" {
		t.Fatalf("notes=%q", e.GetContent("Notes"))
	}
}

func TestInsertForceReplacesExistingEntryFields(t *testing.T) {
	f := newFixture(t)
	stdout, stderr, code := f.runCLI("insert", "internet/email",
		"--username", "forced-user",
		"--password", "forced-secret",
		"--url", "https://forced.example.com",
		"--notes", "forced-note",
		"--otp", "otpauth://totp/Forced?secret=JBSWY3DPEHPK3PXP",
		"--force",
	)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if strings.TrimSpace(stdout) != "internet/email" {
		t.Fatalf("stdout=%q", stdout)
	}
	db := openSeededDB(t, f.dbPath, "master-password")
	e := findEntryByPath(db, "internet/email")
	if e == nil {
		t.Fatal("entry missing")
	}
	if e.GetContent("UserName") != "forced-user" {
		t.Fatalf("username=%q", e.GetContent("UserName"))
	}
	if e.GetPassword() != "forced-secret" {
		t.Fatalf("password=%q", e.GetPassword())
	}
	if e.GetContent("URL") != "https://forced.example.com" {
		t.Fatalf("url=%q", e.GetContent("URL"))
	}
	if e.GetContent("Notes") != "forced-note" {
		t.Fatalf("notes=%q", e.GetContent("Notes"))
	}
	if e.GetContent("otp") != "otpauth://totp/Forced?secret=JBSWY3DPEHPK3PXP" {
		t.Fatalf("otp=%q", e.GetContent("otp"))
	}
}

func TestEditUpdatesFields(t *testing.T) {
	f := newFixture(t)
	stdout, stderr, code := f.runCLI("edit", "internet/email",
		"--username", "alice-updated",
		"--notes", "updated",
		"--password", "rotated-secret",
	)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if strings.TrimSpace(stdout) != "internet/email" {
		t.Fatalf("stdout=%q", stdout)
	}
	db := openSeededDB(t, f.dbPath, "master-password")
	e := findEntryByPath(db, "internet/email")
	if e.GetContent("UserName") != "alice-updated" {
		t.Fatalf("username=%q", e.GetContent("UserName"))
	}
	if e.GetContent("Notes") != "updated" {
		t.Fatalf("notes=%q", e.GetContent("Notes"))
	}
	if e.GetPassword() != "rotated-secret" {
		t.Fatalf("password=%q", e.GetPassword())
	}
}

func TestEditWithoutFlagsOpensEditor(t *testing.T) {
	f := newFixture(t)
	prev := editor.SpawnHook
	editor.SpawnHook = func(argv []string) (int, error) {
		path := argv[len(argv)-1]
		body := strings.Join([]string{
			"# edited by test",
			"title: email-updated",
			"username: alice-editor",
			"password: editor-secret",
			"url: https://edited.example.com",
			"otp: otpauth://totp/Test?secret=JBSWY3DPEHPK3PXP",
			"---",
			"first line",
			"second line",
			"",
		}, "\n")
		return 0, writeRaw(path, body)
	}
	t.Cleanup(func() { editor.SpawnHook = prev })

	stdout, stderr, code := f.runCLI("edit", "internet/email", "--editor", "fake-editor")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if strings.TrimSpace(stdout) != "internet/email-updated" {
		t.Fatalf("stdout=%q", stdout)
	}
	db := openSeededDB(t, f.dbPath, "master-password")
	e := findEntryByPath(db, "internet/email-updated")
	if e == nil {
		t.Fatal("entry missing")
	}
	if e.GetContent("UserName") != "alice-editor" {
		t.Fatalf("username=%q", e.GetContent("UserName"))
	}
	if e.GetPassword() != "editor-secret" {
		t.Fatalf("password=%q", e.GetPassword())
	}
	if e.GetContent("URL") != "https://edited.example.com" {
		t.Fatalf("url=%q", e.GetContent("URL"))
	}
	if e.GetContent("otp") != "otpauth://totp/Test?secret=JBSWY3DPEHPK3PXP" {
		t.Fatalf("otp=%q", e.GetContent("otp"))
	}
	if e.GetContent("Notes") != "first line\nsecond line" {
		t.Fatalf("notes=%q", e.GetContent("Notes"))
	}
}

func TestMoveMovesAndRenamesEntry(t *testing.T) {
	f := newFixture(t)
	stdout, stderr, code := f.runCLI("move", "internet/email", "archive/personal/mail")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if strings.TrimSpace(stdout) != "archive/personal/mail" {
		t.Fatalf("stdout=%q", stdout)
	}
	db := openSeededDB(t, f.dbPath, "master-password")
	e := findEntryByPath(db, "archive/personal/mail")
	if e == nil {
		t.Fatal("moved entry missing")
	}
	if e.GetPassword() != "pw-email" {
		t.Fatalf("password=%q", e.GetPassword())
	}
	if e.GetContent("UserName") != "alice" {
		t.Fatalf("username=%q", e.GetContent("UserName"))
	}
}

func TestDuplicateDuplicatesEntry(t *testing.T) {
	f := newFixture(t)
	stdout, stderr, code := f.runCLI("duplicate", "internet/email", "archive/personal/email-copy")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if strings.TrimSpace(stdout) != "archive/personal/email-copy" {
		t.Fatalf("stdout=%q", stdout)
	}
	db := openSeededDB(t, f.dbPath, "master-password")
	orig := findEntryByPath(db, "internet/email")
	copy := findEntryByPath(db, "archive/personal/email-copy")
	if orig == nil || copy == nil {
		t.Fatal("entries missing")
	}
	if copy.GetContent("UserName") != orig.GetContent("UserName") {
		t.Fatalf("username mismatch")
	}
	if copy.GetPassword() != orig.GetPassword() {
		t.Fatalf("password mismatch")
	}
	if copy.GetContent("URL") != orig.GetContent("URL") {
		t.Fatalf("url mismatch")
	}
	if copy.GetContent("Notes") != orig.GetContent("Notes") {
		t.Fatalf("notes mismatch")
	}
}

func TestRemoveForceRemovesEntry(t *testing.T) {
	f := newFixture(t)
	stdout, stderr, code := f.runCLI("remove", "simple", "-f")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "Deleted") {
		t.Fatalf("stdout=%q", stdout)
	}
	db := openSeededDB(t, f.dbPath, "master-password")
	if findEntryByPath(db, "simple") != nil {
		t.Fatal("entry still present")
	}
}

func TestGenerateCreatesEntryAndRespects(t *testing.T) {
	f := newFixture(t)
	stdout, stderr, code := f.runCLI("generate", "new/services/generated",
		"--username", "gen-user",
		"--url", "https://generated.example.com",
		"--notes", "generated-note",
		"--otp", "otpauth://totp/Generated?secret=JBSWY3DPEHPK3PXP",
		"-L", "32", "--lower", "--digits",
	)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if !strings.HasPrefix(strings.TrimSpace(stdout), "new/services/generated") {
		t.Fatalf("stdout=%q", stdout)
	}
	db := openSeededDB(t, f.dbPath, "master-password")
	e := findEntryByPath(db, "new/services/generated")
	if e == nil {
		t.Fatal("entry missing")
	}
	if e.GetContent("UserName") != "gen-user" {
		t.Fatalf("username=%q", e.GetContent("UserName"))
	}
	if e.GetContent("URL") != "https://generated.example.com" {
		t.Fatalf("url=%q", e.GetContent("URL"))
	}
	if e.GetContent("Notes") != "generated-note" {
		t.Fatalf("notes=%q", e.GetContent("Notes"))
	}
	if e.GetContent("otp") != "otpauth://totp/Generated?secret=JBSWY3DPEHPK3PXP" {
		t.Fatalf("otp=%q", e.GetContent("otp"))
	}
	password := e.GetPassword()
	if len(password) != 32 {
		t.Fatalf("password length=%d", len(password))
	}
	if !regexp.MustCompile(`[a-z]`).MatchString(password) {
		t.Fatalf("no lowercase: %q", password)
	}
	if !regexp.MustCompile(`[0-9]`).MatchString(password) {
		t.Fatalf("no digits: %q", password)
	}
}

func TestGenerateRejectsOverwriteWithoutForce(t *testing.T) {
	f := newFixture(t)
	dbBefore := openSeededDB(t, f.dbPath, "master-password")
	oldPw := findEntryByPath(dbBefore, "internet/email").GetPassword()

	stdout, stderr, code := f.runCLI("generate", "internet/email", "-L", "32", "--lower", "--digits")
	if code != 1 {
		t.Fatalf("code=%d", code)
	}
	if stdout != "" {
		t.Fatalf("stdout=%q", stdout)
	}
	if !strings.Contains(stderr, "Entry already exists: internet/email. Use --force to replace its password.") {
		t.Fatalf("stderr=%q", stderr)
	}
	db := openSeededDB(t, f.dbPath, "master-password")
	if findEntryByPath(db, "internet/email").GetPassword() != oldPw {
		t.Fatalf("password changed")
	}
}

func TestGenerateUpdatesExistingEntryWithForce(t *testing.T) {
	f := newFixture(t)
	dbBefore := openSeededDB(t, f.dbPath, "master-password")
	oldPw := findEntryByPath(dbBefore, "internet/email").GetPassword()

	stdout, stderr, code := f.runCLI("generate", "internet/email",
		"--force", "--username", "generated-user", "--notes", "generated-note",
		"-L", "32", "--lower", "--digits",
	)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if !strings.HasPrefix(strings.TrimSpace(stdout), "internet/email") {
		t.Fatalf("stdout=%q", stdout)
	}
	db := openSeededDB(t, f.dbPath, "master-password")
	e := findEntryByPath(db, "internet/email")
	if e.GetPassword() == oldPw {
		t.Fatalf("password not changed")
	}
	if e.GetContent("UserName") != "generated-user" {
		t.Fatalf("username=%q", e.GetContent("UserName"))
	}
	if e.GetContent("Notes") != "generated-note" {
		t.Fatalf("notes=%q", e.GetContent("Notes"))
	}
	if e.GetContent("URL") != "https://mail.example.com" {
		t.Fatalf("url=%q", e.GetContent("URL"))
	}
	if len(e.GetPassword()) != 32 {
		t.Fatalf("password length=%d", len(e.GetPassword()))
	}
}

func TestGenerateAcceptsDatabaseSelector(t *testing.T) {
	f := newFixture(t)
	cfg := writeMultiDBConfig(t, f, multiDBConfigOpts{})
	stdout, stderr, code := f.runCLIWith(runOpts{
		skipDatabaseArg:  true,
		skipPasswordFile: true,
		configFile:       cfg,
	}, "generate", "@work", "generated/work-secret")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if !strings.HasPrefix(strings.TrimSpace(stdout), "generated/work-secret") {
		t.Fatalf("stdout=%q", stdout)
	}
}

// writeRaw is a tiny helper that exists separately so the editor test can
// keep its inline fake.
func writeRaw(path, contents string) error {
	return writeFileErr(path, contents)
}
