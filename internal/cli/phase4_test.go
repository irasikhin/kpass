package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/irasikhin/kpass/internal/config"
	"github.com/irasikhin/kpass/internal/db"
	"github.com/irasikhin/kpass/internal/otp"
)

// --- TOTP -------------------------------------------------------------------

func TestGenerateTotpKnownVector(t *testing.T) {
	uri := "otpauth://totp/Test?secret=GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ&digits=8&algorithm=SHA1&period=30"
	code, err := otp.Generate(uri, time.Unix(59, 0))
	if err != nil {
		t.Fatal(err)
	}
	if code != "94287082" {
		t.Fatalf("got %q want 94287082", code)
	}
}

func TestGetOtpOutputsCode(t *testing.T) {
	f := newFixture(t)
	prev := otp.NowHook
	otp.NowHook = func() time.Time { return time.Unix(59, 0) }
	t.Cleanup(func() { otp.NowHook = prev })

	stdout, stderr, code := f.runCLI("get", "otp/sample", "--field", "otp")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if strings.TrimSpace(stdout) != "94287082" {
		t.Fatalf("stdout=%q", stdout)
	}
}

func TestCopyOtpUsesGeneratedCode(t *testing.T) {
	f := newFixture(t)
	prev := otp.NowHook
	otp.NowHook = func() time.Time { return time.Unix(59, 0) }
	t.Cleanup(func() { otp.NowHook = prev })

	var captured string
	var capturedTimeout int
	ClipboardWriter = func(v string, to int) error {
		captured = v
		capturedTimeout = to
		return nil
	}
	stdout, stderr, code := f.runCLI("copy", "otp/sample", "--field", "otp", "0")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if captured != "94287082" || capturedTimeout != 0 {
		t.Fatalf("captured=%q timeout=%d", captured, capturedTimeout)
	}
	if !strings.Contains(stdout, "Copied otp for otp/sample to clipboard") {
		t.Fatalf("stdout=%q", stdout)
	}
}

// --- Merge ------------------------------------------------------------------

func (f *fixture) mergeFlags() []string {
	return []string{"--source-password-file", f.sourcePasswordFile}
}

func TestMergeSkipPreservesExisting(t *testing.T) {
	f := newFixture(t)
	args := append([]string{"merge", f.sourceDBPath}, f.mergeFlags()...)
	args = append(args, "--on-conflict", "skip")
	stdout, stderr, code := f.runCLI(args...)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "Imported: 1") || !strings.Contains(stdout, "Skipped: 1") {
		t.Fatalf("stdout=%q", stdout)
	}
	mainDB := openSeededDB(t, f.dbPath, "master-password")
	if e := findEntryByPath(mainDB, "internet/email"); e == nil || e.GetContent("UserName") != "alice" || e.GetPassword() != "pw-email" {
		t.Fatalf("internet/email mutated: %+v", e)
	}
	imp := findEntryByPath(mainDB, "shared/chat")
	if imp == nil || imp.GetPassword() != "chat-pass" || imp.GetContent("Notes") != "new-entry" {
		t.Fatalf("import missing: %+v", imp)
	}
}

func TestMergeUsesSelectedTargetDatabase(t *testing.T) {
	f := newFixture(t)
	cfg := writeMultiDBConfig(t, f, multiDBConfigOpts{})
	args := append([]string{"merge", "@work", f.sourceDBPath}, f.mergeFlags()...)
	stdout, stderr, code := f.runCLIWith(runOpts{
		skipDatabaseArg:  true,
		skipPasswordFile: true,
		configFile:       cfg,
	}, args...)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "Imported: 1") || !strings.Contains(stdout, "Renamed: 1") {
		t.Fatalf("stdout=%q", stdout)
	}
	workDB := openSeededDB(t, f.workDBPath, "work-password")
	if e := findEntryByPath(workDB, "shared/chat"); e == nil || e.GetPassword() != "pw-work-chat" {
		t.Fatalf("original chat mutated: %+v", e)
	}
	if e := findEntryByPath(workDB, "shared/chat (2)"); e == nil || e.GetPassword() != "chat-pass" {
		t.Fatalf("renamed chat missing: %+v", e)
	}
}

func TestMergeDefaultsToRenameOnConflict(t *testing.T) {
	f := newFixture(t)
	args := append([]string{"merge", f.sourceDBPath}, f.mergeFlags()...)
	stdout, stderr, code := f.runCLI(args...)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "Imported: 1") || !strings.Contains(stdout, "Renamed: 1") {
		t.Fatalf("stdout=%q", stdout)
	}
	mainDB := openSeededDB(t, f.dbPath, "master-password")
	if e := findEntryByPath(mainDB, "internet/email"); e.GetPassword() != "pw-email" {
		t.Fatalf("original modified")
	}
	renamed := findEntryByPath(mainDB, "internet/email (2)")
	if renamed == nil || renamed.GetPassword() != "merged-pass" {
		t.Fatalf("renamed missing: %+v", renamed)
	}
	// Custom property
	if v := renamed.GetContent("env"); v != "merged" {
		t.Fatalf("custom prop=%q", v)
	}
	// Attachment
	if len(renamed.Binaries) != 1 || renamed.Binaries[0].Name != "merged.txt" {
		t.Fatalf("attachments=%+v", renamed.Binaries)
	}
}

func TestMergeRenameSuffixOverride(t *testing.T) {
	f := newFixture(t)
	args := append([]string{"merge", f.sourceDBPath}, f.mergeFlags()...)
	args = append(args, "--rename-suffix", "merged")
	stdout, stderr, code := f.runCLI(args...)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "Renamed: 1") {
		t.Fatalf("stdout=%q", stdout)
	}
	mainDB := openSeededDB(t, f.dbPath, "master-password")
	r := findEntryByPath(mainDB, "internet/email (merged)")
	if r == nil || r.GetPassword() != "merged-pass" {
		t.Fatalf("renamed=%+v", r)
	}
}

func TestMergeOverwriteReplaces(t *testing.T) {
	f := newFixture(t)
	args := append([]string{"merge", f.sourceDBPath}, f.mergeFlags()...)
	args = append(args, "--on-conflict", "overwrite")
	stdout, stderr, code := f.runCLI(args...)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "Imported: 1") || !strings.Contains(stdout, "Overwritten: 1") {
		t.Fatalf("stdout=%q", stdout)
	}
	mainDB := openSeededDB(t, f.dbPath, "master-password")
	e := findEntryByPath(mainDB, "internet/email")
	if e.GetContent("UserName") != "merged-user" {
		t.Fatalf("username=%q", e.GetContent("UserName"))
	}
	if e.GetPassword() != "merged-pass" {
		t.Fatalf("password=%q", e.GetPassword())
	}
	if e.GetContent("URL") != "https://merged.example.com" {
		t.Fatalf("url=%q", e.GetContent("URL"))
	}
	if e.GetContent("Notes") != "merged-note" {
		t.Fatalf("notes=%q", e.GetContent("Notes"))
	}
	if e.Tags != "merged;email" {
		t.Fatalf("tags=%q", e.Tags)
	}
	if e.GetContent("env") != "merged" {
		t.Fatalf("custom=%q", e.GetContent("env"))
	}
	if len(e.Binaries) != 1 || e.Binaries[0].Name != "merged.txt" {
		t.Fatalf("attachments=%+v", e.Binaries)
	}
	// Resolve binary content
	mainDBPtr := mainDB
	binID := e.Binaries[0].Value.ID
	bin := mainDBPtr.FindBinary(binID)
	if bin == nil {
		t.Fatal("binary missing")
	}
	data, err := bin.GetContentBytes()
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "merged-attachment" {
		t.Fatalf("binary=%q", data)
	}
}

// --- db sub-CLI -------------------------------------------------------------

func TestDbLsListsProfiles(t *testing.T) {
	f := newFixture(t)
	cfg := writeMultiDBConfig(t, f, multiDBConfigOpts{})
	stdout, stderr, code := f.runCLIWith(runOpts{
		skipDatabaseArg:  true,
		skipPasswordFile: true,
		configFile:       cfg,
	}, "db", "ls")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "Default: main") {
		t.Fatalf("stdout=%q", stdout)
	}
	if !strings.Contains(stdout, "* main:") {
		t.Fatalf("stdout=%q", stdout)
	}
	if !strings.Contains(stdout, "  work:") {
		t.Fatalf("stdout=%q", stdout)
	}
}

func TestDbDefaultWithoutNameShowsCurrent(t *testing.T) {
	f := newFixture(t)
	cfg := writeMultiDBConfig(t, f, multiDBConfigOpts{})
	stdout, stderr, code := f.runCLIWith(runOpts{
		skipDatabaseArg:  true,
		skipPasswordFile: true,
		configFile:       cfg,
	}, "db", "default")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if strings.TrimSpace(stdout) != "main" {
		t.Fatalf("stdout=%q", stdout)
	}
}

func TestDbAddCreatesNewConfig(t *testing.T) {
	f := newFixture(t)
	cfgPath := f.root + "/managed-kpass.toml"
	stdout, stderr, code := f.runCLIWith(runOpts{
		skipDatabaseArg:  true,
		skipPasswordFile: true,
		configFile:       cfgPath,
	}, "db", "add", "main", f.dbPath,
		"--password-file", f.passwordFile,
		"--default",
	)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "Added database profile main") {
		t.Fatalf("stdout=%q", stdout)
	}
	fc, _, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if fc.DefaultDatabase != "main" {
		t.Fatalf("default=%q", fc.DefaultDatabase)
	}
	if fc.Databases["main"].PasswordFile != f.passwordFile {
		t.Fatalf("password file mismatch")
	}
}

func TestDbDefaultSwitches(t *testing.T) {
	f := newFixture(t)
	cfg := writeMultiDBConfig(t, f, multiDBConfigOpts{})
	stdout, stderr, code := f.runCLIWith(runOpts{
		skipDatabaseArg:  true,
		skipPasswordFile: true,
		configFile:       cfg,
	}, "db", "default", "work")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "Default database profile is now work") {
		t.Fatalf("stdout=%q", stdout)
	}
	fc, _, err := config.Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if fc.DefaultDatabase != "work" {
		t.Fatalf("default=%q", fc.DefaultDatabase)
	}
}

func TestDbRmSwitchesDefault(t *testing.T) {
	f := newFixture(t)
	cfg := writeMultiDBConfig(t, f, multiDBConfigOpts{})
	stdout, stderr, code := f.runCLIWith(runOpts{
		skipDatabaseArg:  true,
		skipPasswordFile: true,
		configFile:       cfg,
	}, "db", "rm", "main")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "Removed database profile main") {
		t.Fatalf("stdout=%q", stdout)
	}
	if !strings.Contains(stdout, "Default database profile is now work") {
		t.Fatalf("stdout=%q", stdout)
	}
	fc, _, err := config.Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if fc.DefaultDatabase != "work" {
		t.Fatalf("default=%q", fc.DefaultDatabase)
	}
	if _, ok := fc.Databases["main"]; ok {
		t.Fatalf("main still present")
	}
}

// --- KeyboardInterrupt ------------------------------------------------------

func TestKeyboardInterruptReturnsCleanExit(t *testing.T) {
	f := newFixture(t)
	prev := db.OpenHook
	db.OpenHook = func(cfg config.Config) (*db.DB, error) {
		return nil, context.Canceled
	}
	t.Cleanup(func() { db.OpenHook = prev })

	stdout, stderr, code := f.runCLI("ls")
	if code != 130 {
		t.Fatalf("code=%d", code)
	}
	if stdout != "" {
		t.Fatalf("stdout=%q", stdout)
	}
	if strings.TrimSpace(stderr) != "Interrupted." {
		t.Fatalf("stderr=%q", stderr)
	}
}
