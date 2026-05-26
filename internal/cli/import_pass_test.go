package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- parser unit tests ------------------------------------------------------

func TestParsePassEntryPasswordOnly(t *testing.T) {
	ie := parsePassEntry("github", []byte("hunter2\n"))
	if ie.Password != "hunter2" {
		t.Fatalf("password=%q", ie.Password)
	}
	if ie.Username != "" || ie.URL != "" || ie.Notes != "" || ie.OTP != "" {
		t.Fatalf("expected only password set, got %+v", ie)
	}
	if ie.Custom != nil {
		t.Fatalf("Custom should be nil when no custom fields, got %v", ie.Custom)
	}
	if ie.Title != "github" {
		t.Fatalf("title=%q", ie.Title)
	}
}

func TestParsePassEntryKeyValueFields(t *testing.T) {
	content := `s3cret
login: alice
url: https://example.com
some random note
another line
`
	ie := parsePassEntry("internet/example", []byte(content))
	if ie.Password != "s3cret" {
		t.Fatalf("password=%q", ie.Password)
	}
	if ie.Username != "alice" {
		t.Fatalf("username=%q", ie.Username)
	}
	if ie.URL != "https://example.com" {
		t.Fatalf("url=%q", ie.URL)
	}
	if ie.Notes != "some random note\nanother line" {
		t.Fatalf("notes=%q", ie.Notes)
	}
	if ie.Title != "example" {
		t.Fatalf("title=%q", ie.Title)
	}
}

func TestParsePassEntryAllUsernameAliases(t *testing.T) {
	cases := map[string]string{
		"user":     "u-line",
		"username": "u-line",
		"login":    "u-line",
	}
	for keyword, want := range cases {
		t.Run(keyword, func(t *testing.T) {
			body := fmt.Sprintf("pw\n%s: %s\n", keyword, want)
			ie := parsePassEntry("x", []byte(body))
			if ie.Username != want {
				t.Fatalf("username=%q for %q", ie.Username, keyword)
			}
			if ie.Notes != "" {
				t.Fatalf("notes should be empty, got %q", ie.Notes)
			}
		})
	}
}

func TestParsePassEntryURLAliases(t *testing.T) {
	for _, k := range []string{"url", "website"} {
		t.Run(k, func(t *testing.T) {
			body := fmt.Sprintf("pw\n%s: https://x\n", k)
			ie := parsePassEntry("x", []byte(body))
			if ie.URL != "https://x" {
				t.Fatalf("url=%q for %q", ie.URL, k)
			}
		})
	}
}

func TestParsePassEntryOtpAuthURI(t *testing.T) {
	content := "pw\notpauth://totp/Demo?secret=ABC123&issuer=Demo\n"
	ie := parsePassEntry("x", []byte(content))
	if ie.OTP != "otpauth://totp/Demo?secret=ABC123&issuer=Demo" {
		t.Fatalf("otp=%q", ie.OTP)
	}
	if ie.Notes != "" {
		t.Fatalf("otp should not leak into notes, got %q", ie.Notes)
	}
}

func TestParsePassEntryEmailFallsBackToUsername(t *testing.T) {
	ie := parsePassEntry("x", []byte("pw\nemail: a@b\n"))
	if ie.Username != "a@b" {
		t.Fatalf("username=%q", ie.Username)
	}
	if _, ok := ie.Custom["email"]; ok {
		t.Fatalf("email should not become custom when username absent")
	}
}

func TestParsePassEntryEmailKeepsCustomWhenUsernameSet(t *testing.T) {
	ie := parsePassEntry("x", []byte("pw\nlogin: alice\nemail: a@b\n"))
	if ie.Username != "alice" {
		t.Fatalf("username=%q", ie.Username)
	}
	if ie.Custom["email"] != "a@b" {
		t.Fatalf("custom email=%q", ie.Custom["email"])
	}
}

func TestParsePassEntryUnknownKeyBecomesCustom(t *testing.T) {
	ie := parsePassEntry("x", []byte("pw\nrecovery: phrase-here\n"))
	if ie.Custom["recovery"] != "phrase-here" {
		t.Fatalf("custom=%v", ie.Custom)
	}
	if ie.Notes != "" {
		t.Fatalf("notes should be empty, got %q", ie.Notes)
	}
}

func TestParsePassEntryNotesPreserveBlankInteriorLines(t *testing.T) {
	body := "pw\n\nfirst note\n\nsecond\n\n"
	ie := parsePassEntry("x", []byte(body))
	if ie.Notes != "first note\n\nsecond" {
		t.Fatalf("notes=%q", ie.Notes)
	}
}

func TestParsePassEntryCRLF(t *testing.T) {
	body := "pw\r\nlogin: alice\r\nfree\r\n"
	ie := parsePassEntry("x", []byte(body))
	if ie.Password != "pw" || ie.Username != "alice" || ie.Notes != "free" {
		t.Fatalf("got %+v", ie)
	}
}

func TestParsePassEntryEmpty(t *testing.T) {
	ie := parsePassEntry("x", []byte(""))
	if ie.Password != "" {
		t.Fatalf("password=%q", ie.Password)
	}
}

func TestParsePassEntryFirstLineWinsForPassword(t *testing.T) {
	ie := parsePassEntry("x", []byte("real-pw\npassword: ignored\n"))
	if ie.Password != "real-pw" {
		t.Fatalf("password=%q", ie.Password)
	}
}

func TestParsePassEntryColonInValue(t *testing.T) {
	ie := parsePassEntry("x", []byte("pw\nurl: https://host:8080/path\n"))
	if ie.URL != "https://host:8080/path" {
		t.Fatalf("url=%q", ie.URL)
	}
}

func TestParsePassEntryWhitespaceKeyTreatedAsNotes(t *testing.T) {
	ie := parsePassEntry("x", []byte("pw\nhello world: stuff\n"))
	if ie.Notes != "hello world: stuff" {
		t.Fatalf("notes=%q", ie.Notes)
	}
	if len(ie.Custom) != 0 {
		t.Fatalf("custom should be empty, got %v", ie.Custom)
	}
}

func TestParsePassEntryTitleFromPath(t *testing.T) {
	ie := parsePassEntry("internet/email/alice", []byte("pw"))
	if ie.Title != "alice" {
		t.Fatalf("title=%q", ie.Title)
	}
}

// --- walker unit tests ------------------------------------------------------

// stubStore creates a fake pass tree where each .gpg file contains its own
// "ciphertext" plaintext. The decryptor reads it verbatim.
func stubStore(t *testing.T, layout map[string]string) (string, PassDecryptor) {
	t.Helper()
	root := t.TempDir()
	for rel, content := range layout {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	dec := func(p string) ([]byte, error) { return os.ReadFile(p) }
	return root, dec
}

func TestWalkPassStoreNestedGroups(t *testing.T) {
	root, dec := stubStore(t, map[string]string{
		"top.gpg":                  "pw-top",
		"internet/email/alice.gpg": "pw-alice\nlogin: alice@x\n",
		"internet/email/bob.gpg":   "pw-bob\nlogin: bob@x\n",
		"work/deep/nested/svc.gpg": "pw-svc",
		"work/db/postgres.gpg":     "pw-pg\nurl: postgres://h/db\n",
	})
	got, err := walkPassStore(root, dec)
	if err != nil {
		t.Fatal(err)
	}
	wantPaths := []string{
		"internet/email/alice",
		"internet/email/bob",
		"top",
		"work/db/postgres",
		"work/deep/nested/svc",
	}
	if len(got) != len(wantPaths) {
		t.Fatalf("got %d entries, want %d: %+v", len(got), len(wantPaths), got)
	}
	for i, want := range wantPaths {
		if got[i].Path != want {
			t.Fatalf("entry[%d].Path=%q want %q", i, got[i].Path, want)
		}
	}
	if got[3].URL != "postgres://h/db" {
		t.Fatalf("postgres URL=%q", got[3].URL)
	}
}

func TestWalkPassStoreSkipsHiddenAndNonGPG(t *testing.T) {
	root, dec := stubStore(t, map[string]string{
		".gpg-id":             "key@example.com",
		".git/HEAD":           "ref",
		".extensions/foo.gpg": "ignored",
		"README.md":           "docs",
		"real.gpg":            "pw-real",
		"sub/.hidden/bad.gpg": "no",
		"sub/good.gpg":        "pw-good",
	})
	got, err := walkPassStore(root, dec)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d: %+v", len(got), got)
	}
	if got[0].Path != "real" || got[1].Path != "sub/good" {
		t.Fatalf("paths=%+v", []string{got[0].Path, got[1].Path})
	}
}

func TestWalkPassStoreDecryptorErrorPropagates(t *testing.T) {
	root, _ := stubStore(t, map[string]string{
		"a.gpg": "pw",
	})
	dec := func(p string) ([]byte, error) { return nil, fmt.Errorf("boom") }
	_, err := walkPassStore(root, dec)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected boom error, got %v", err)
	}
}

func TestWalkPassStoreSpacesAndUnicodeInPaths(t *testing.T) {
	root, dec := stubStore(t, map[string]string{
		"My Stuff/Личный счёт.gpg":       "pw-1",
		"My Stuff/sub dir/co-worker.gpg": "pw-2",
	})
	got, err := walkPassStore(root, dec)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d entries: %+v", len(got), got)
	}
	if got[0].Path != "My Stuff/sub dir/co-worker" {
		t.Fatalf("got[0].Path=%q", got[0].Path)
	}
	if got[1].Path != "My Stuff/Личный счёт" {
		t.Fatalf("got[1].Path=%q", got[1].Path)
	}
	if got[1].Title != "Личный счёт" {
		t.Fatalf("title=%q", got[1].Title)
	}
}

func TestWalkPassStoreEmpty(t *testing.T) {
	root := t.TempDir()
	got, err := walkPassStore(root, func(p string) ([]byte, error) { return nil, nil })
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected zero entries, got %d", len(got))
	}
}

func TestWalkPassStoreMissingDir(t *testing.T) {
	_, err := walkPassStore(filepath.Join(t.TempDir(), "missing"), nil)
	if err == nil {
		t.Fatal("expected error for missing dir")
	}
}

// --- resolvePassStore -------------------------------------------------------

func TestResolvePassStoreExplicit(t *testing.T) {
	dir := t.TempDir()
	got, err := resolvePassStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != dir {
		t.Fatalf("got=%q want=%q", got, dir)
	}
}

func TestResolvePassStoreFromEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PASSWORD_STORE_DIR", dir)
	got, err := resolvePassStore("")
	if err != nil {
		t.Fatal(err)
	}
	if got != dir {
		t.Fatalf("got=%q want=%q", got, dir)
	}
}

func TestResolvePassStoreMissing(t *testing.T) {
	t.Setenv("PASSWORD_STORE_DIR", filepath.Join(t.TempDir(), "nope"))
	_, err := resolvePassStore("")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolvePassStoreIsFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(f, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := resolvePassStore(f)
	if err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("expected not-a-directory error, got %v", err)
	}
}

// --- end-to-end CLI tests with injected decryptor ---------------------------

// withStubStore builds a pass-style directory and installs a decryptor that
// just reads the file. Returns the store path.
func (f *fixture) withStubStore(t *testing.T, layout map[string]string) string {
	t.Helper()
	store := filepath.Join(f.root, "password-store")
	for rel, content := range layout {
		full := filepath.Join(store, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	PassDecryptorHook = func(p string) ([]byte, error) { return os.ReadFile(p) }
	return store
}

func TestImportPassEndToEndCreatesEntries(t *testing.T) {
	f := newFixture(t)
	store := f.withStubStore(t, map[string]string{
		"new/site.gpg":       "pw-site\nlogin: alice\nurl: https://site.example\nnote line\n",
		"new/sub/nested.gpg": "pw-nested",
		"otpauth-style.gpg":  "pw-otp\notpauth://totp/X?secret=ABC&issuer=I\n",
	})
	_, stderr, code := f.runCLI("import-pass", store, "-f")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}

	db := openSeededDB(t, f.dbPath, "master-password")

	site := findEntryByPath(db, "new/site")
	if site == nil {
		t.Fatal("new/site missing")
	}
	if site.GetPassword() != "pw-site" {
		t.Fatalf("password=%q", site.GetPassword())
	}
	if site.GetContent("UserName") != "alice" {
		t.Fatalf("username=%q", site.GetContent("UserName"))
	}
	if site.GetContent("URL") != "https://site.example" {
		t.Fatalf("url=%q", site.GetContent("URL"))
	}
	if site.GetContent("Notes") != "note line" {
		t.Fatalf("notes=%q", site.GetContent("Notes"))
	}

	nested := findEntryByPath(db, "new/sub/nested")
	if nested == nil {
		t.Fatal("nested entry missing")
	}
	if nested.GetPassword() != "pw-nested" {
		t.Fatalf("nested password=%q", nested.GetPassword())
	}

	otpE := findEntryByPath(db, "otpauth-style")
	if otpE == nil {
		t.Fatal("otp entry missing")
	}
	if otpE.GetContent("otp") != "otpauth://totp/X?secret=ABC&issuer=I" {
		t.Fatalf("otp=%q", otpE.GetContent("otp"))
	}
}

func TestImportPassConflictSkip(t *testing.T) {
	f := newFixture(t)
	store := f.withStubStore(t, map[string]string{
		"internet/email.gpg": "new-password\nlogin: new-user\n",
	})
	stdout, stderr, code := f.runCLI("import-pass", store, "-f", "--on-conflict=skip")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "Skipped:") {
		t.Fatalf("stdout missing Skipped: %s", stdout)
	}
	db := openSeededDB(t, f.dbPath, "master-password")
	e := findEntryByPath(db, "internet/email")
	if e == nil {
		t.Fatal("seeded entry vanished")
	}
	if e.GetPassword() != "pw-email" {
		t.Fatalf("password should be unchanged, got %q", e.GetPassword())
	}
}

func TestImportPassConflictOverwrite(t *testing.T) {
	f := newFixture(t)
	store := f.withStubStore(t, map[string]string{
		"internet/email.gpg": "new-password\nlogin: new-user\n",
	})
	_, stderr, code := f.runCLI("import-pass", store, "-f", "--on-conflict=overwrite")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	db := openSeededDB(t, f.dbPath, "master-password")
	e := findEntryByPath(db, "internet/email")
	if e == nil {
		t.Fatal("entry missing")
	}
	if e.GetPassword() != "new-password" {
		t.Fatalf("password=%q", e.GetPassword())
	}
	if e.GetContent("UserName") != "new-user" {
		t.Fatalf("username=%q", e.GetContent("UserName"))
	}
}

func TestImportPassConflictRename(t *testing.T) {
	f := newFixture(t)
	store := f.withStubStore(t, map[string]string{
		"internet/email.gpg": "rename-pw",
	})
	_, stderr, code := f.runCLI("import-pass", store, "-f", "--on-conflict=rename")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	db := openSeededDB(t, f.dbPath, "master-password")
	orig := findEntryByPath(db, "internet/email")
	if orig == nil || orig.GetPassword() != "pw-email" {
		t.Fatal("original entry mutated")
	}
	renamed := findEntryByPath(db, "internet/email (1)")
	if renamed == nil {
		t.Fatal("renamed entry missing")
	}
	if renamed.GetPassword() != "rename-pw" {
		t.Fatalf("renamed password=%q", renamed.GetPassword())
	}
}

func TestImportPassConflictError(t *testing.T) {
	f := newFixture(t)
	store := f.withStubStore(t, map[string]string{
		"internet/email.gpg": "x",
	})
	_, stderr, code := f.runCLI("import-pass", store, "-f", "--on-conflict=error")
	if code == 0 {
		t.Fatal("expected non-zero exit on conflict")
	}
	if !strings.Contains(stderr, "already exists") {
		t.Fatalf("stderr=%q", stderr)
	}
}

func TestImportPassEmptyStore(t *testing.T) {
	f := newFixture(t)
	store := f.withStubStore(t, map[string]string{})
	// Need the empty dir to actually exist:
	if err := os.MkdirAll(store, 0o700); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := f.runCLI("import-pass", store, "-f")
	if code == 0 {
		t.Fatal("expected non-zero exit on empty store")
	}
	if !strings.Contains(stderr, "No entries") {
		t.Fatalf("stderr=%q", stderr)
	}
}

func TestImportPassAbortByPromptDeniesWrite(t *testing.T) {
	f := newFixture(t)
	store := f.withStubStore(t, map[string]string{
		"new/site.gpg": "pw",
	})
	_, stderr, code := f.runCLIWith(runOpts{stdin: "n\n"}, "import-pass", store)
	if code == 0 {
		t.Fatal("expected non-zero exit when user denies prompt")
	}
	if !strings.Contains(stderr, "Aborted") {
		t.Fatalf("stderr=%q", stderr)
	}
	db := openSeededDB(t, f.dbPath, "master-password")
	if e := findEntryByPath(db, "new/site"); e != nil {
		t.Fatal("entry should not have been written")
	}
}

func TestImportPassCustomFieldsPersisted(t *testing.T) {
	f := newFixture(t)
	store := f.withStubStore(t, map[string]string{
		"app.gpg": "pw\nlogin: alice\nrecovery: phrase-here\nenv: prod\n",
	})
	_, stderr, code := f.runCLI("import-pass", store, "-f")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	db := openSeededDB(t, f.dbPath, "master-password")
	e := findEntryByPath(db, "app")
	if e == nil {
		t.Fatal("entry missing")
	}
	if e.GetContent("recovery") != "phrase-here" {
		t.Fatalf("recovery=%q", e.GetContent("recovery"))
	}
	if e.GetContent("env") != "prod" {
		t.Fatalf("env=%q", e.GetContent("env"))
	}
}

func TestImportPassMissingStoreErrors(t *testing.T) {
	f := newFixture(t)
	PassDecryptorHook = func(p string) ([]byte, error) { return os.ReadFile(p) }
	missing := filepath.Join(f.root, "no-such-store")
	_, stderr, code := f.runCLI("import-pass", missing, "-f")
	if code == 0 {
		t.Fatal("expected non-zero exit")
	}
	if !strings.Contains(stderr, "not found") {
		t.Fatalf("stderr=%q", stderr)
	}
}
