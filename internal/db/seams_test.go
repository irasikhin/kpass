package db

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tobischo/gokeepasslib/v3"
	w "github.com/tobischo/gokeepasslib/v3/wrappers"

	"github.com/irasikhin/kpass/internal/cache"
	"github.com/irasikhin/kpass/internal/config"
)

// resetSeams snapshots and restores all package-level seams + hooks.
func resetSeams(t *testing.T) {
	t.Helper()
	origCT := osCreateTempFn
	origR := osRenameFn
	origOF := osOpenFileFn
	origOp := osOpenFn
	origSt := osStatFn
	origCh := osChmodFileFn
	origCp := ioCopyFn
	origEnc := encodeFn
	origLock := lockProtectedFn
	origUnlock := unlockProtectedFn
	origOpenHook := OpenHook
	origPrompter := PasswordPrompter
	t.Cleanup(func() {
		osCreateTempFn = origCT
		osRenameFn = origR
		osOpenFileFn = origOF
		osOpenFn = origOp
		osStatFn = origSt
		osChmodFileFn = origCh
		ioCopyFn = origCp
		encodeFn = origEnc
		lockProtectedFn = origLock
		unlockProtectedFn = origUnlock
		OpenHook = origOpenHook
		PasswordPrompter = origPrompter
	})
}

func TestCreate_EncodeError(t *testing.T) {
	resetSeams(t)
	encodeFn = func(io.Writer, *gokeepasslib.Database) error { return errors.New("encode fail") }
	path := filepath.Join(t.TempDir(), "enc.kdbx")
	if err := Create(path, "pw", ""); err == nil || !strings.Contains(err.Error(), "cannot write database") {
		t.Errorf("expected encode error, got %v", err)
	}
}

func TestSave_BackupFailureNonFatal(t *testing.T) {
	resetSeams(t)
	// First open returns the source error; subsequent error pretends backup fails.
	d := seedDB(t)
	path := writeKdbx(t, d, "pw")
	loaded, err := OpenSimple(path, "", "", "pw")
	if err != nil {
		t.Fatal(err)
	}
	osOpenFn = func(string) (*os.File, error) { return nil, errors.New("backup-open fail") }
	if err := loaded.Save(); err != nil {
		t.Fatalf("Save should swallow backup error, got %v", err)
	}
}

func TestSave_LockProtectedError(t *testing.T) {
	resetSeams(t)
	d := seedDB(t)
	path := writeKdbx(t, d, "pw")
	loaded, err := OpenSimple(path, "", "", "pw")
	if err != nil {
		t.Fatal(err)
	}
	lockProtectedFn = func(*gokeepasslib.Database) error { return errors.New("lock fail") }
	if err := loaded.Save(); err == nil || !strings.Contains(err.Error(), "lock") {
		t.Errorf("expected lock error, got %v", err)
	}
}

func TestSave_CreateTempError(t *testing.T) {
	resetSeams(t)
	d := seedDB(t)
	path := writeKdbx(t, d, "pw")
	loaded, err := OpenSimple(path, "", "", "pw")
	if err != nil {
		t.Fatal(err)
	}
	osCreateTempFn = func(string, string) (*os.File, error) { return nil, errors.New("tmp fail") }
	if err := loaded.Save(); err == nil || !strings.Contains(err.Error(), "tmp fail") {
		t.Errorf("expected tmp error, got %v", err)
	}
}

func TestSave_ChmodError(t *testing.T) {
	resetSeams(t)
	d := seedDB(t)
	path := writeKdbx(t, d, "pw")
	loaded, err := OpenSimple(path, "", "", "pw")
	if err != nil {
		t.Fatal(err)
	}
	osChmodFileFn = func(*os.File, os.FileMode) error { return errors.New("chmod fail") }
	if err := loaded.Save(); err == nil || !strings.Contains(err.Error(), "chmod fail") {
		t.Errorf("expected chmod error, got %v", err)
	}
}

func TestSave_EncodeError(t *testing.T) {
	resetSeams(t)
	d := seedDB(t)
	path := writeKdbx(t, d, "pw")
	loaded, err := OpenSimple(path, "", "", "pw")
	if err != nil {
		t.Fatal(err)
	}
	encodeFn = func(io.Writer, *gokeepasslib.Database) error { return errors.New("enc fail") }
	if err := loaded.Save(); err == nil || !strings.Contains(err.Error(), "enc fail") {
		t.Errorf("expected encode error, got %v", err)
	}
}

func TestSave_RenameError(t *testing.T) {
	resetSeams(t)
	d := seedDB(t)
	path := writeKdbx(t, d, "pw")
	loaded, err := OpenSimple(path, "", "", "pw")
	if err != nil {
		t.Fatal(err)
	}
	osRenameFn = func(string, string) error { return errors.New("rename fail") }
	if err := loaded.Save(); err == nil || !strings.Contains(err.Error(), "rename fail") {
		t.Errorf("expected rename error, got %v", err)
	}
}

func TestSave_UnlockError(t *testing.T) {
	resetSeams(t)
	d := seedDB(t)
	path := writeKdbx(t, d, "pw")
	loaded, err := OpenSimple(path, "", "", "pw")
	if err != nil {
		t.Fatal(err)
	}
	unlockProtectedFn = func(*gokeepasslib.Database) error { return errors.New("unlock fail") }
	if err := loaded.Save(); err == nil || !strings.Contains(err.Error(), "unlock fail") {
		t.Errorf("expected unlock error, got %v", err)
	}
}

func TestBackup_OpenError(t *testing.T) {
	resetSeams(t)
	d := &DB{Path: filepath.Join(t.TempDir(), "x.kdbx")}
	if err := os.WriteFile(d.Path, []byte("body"), 0o600); err != nil {
		t.Fatal(err)
	}
	osOpenFn = func(string) (*os.File, error) { return nil, errors.New("io fail") }
	if _, err := d.Backup(); err == nil {
		t.Error("expected backup open error")
	}
}

func TestBackup_DstCreateError(t *testing.T) {
	resetSeams(t)
	d := &DB{Path: filepath.Join(t.TempDir(), "x.kdbx")}
	if err := os.WriteFile(d.Path, []byte("body"), 0o600); err != nil {
		t.Fatal(err)
	}
	osOpenFileFn = func(string, int, os.FileMode) (*os.File, error) { return nil, errors.New("dst fail") }
	if _, err := d.Backup(); err == nil || !strings.Contains(err.Error(), "dst fail") {
		t.Errorf("expected dst-create error, got %v", err)
	}
}

func TestBackup_CopyError(t *testing.T) {
	resetSeams(t)
	d := &DB{Path: filepath.Join(t.TempDir(), "x.kdbx")}
	if err := os.WriteFile(d.Path, []byte("body"), 0o600); err != nil {
		t.Fatal(err)
	}
	ioCopyFn = func(io.Writer, io.Reader) (int64, error) { return 0, errors.New("copy fail") }
	if _, err := d.Backup(); err == nil || !strings.Contains(err.Error(), "copy fail") {
		t.Errorf("expected copy error, got %v", err)
	}
}

func TestRestoreBackup_OpenError(t *testing.T) {
	resetSeams(t)
	osOpenFn = func(string) (*os.File, error) { return nil, errors.New("src fail") }
	if err := RestoreBackup("/whatever", "/whatever2"); err == nil || !strings.Contains(err.Error(), "cannot open backup") {
		t.Errorf("expected open error, got %v", err)
	}
}

func TestRestoreBackup_DstCreateError(t *testing.T) {
	resetSeams(t)
	dir := t.TempDir()
	bk := filepath.Join(dir, "b.bak")
	if err := os.WriteFile(bk, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	osOpenFileFn = func(string, int, os.FileMode) (*os.File, error) { return nil, errors.New("dst fail") }
	if err := RestoreBackup(bk, filepath.Join(dir, "out.kdbx")); err == nil || !strings.Contains(err.Error(), "cannot write database") {
		t.Errorf("expected dst error, got %v", err)
	}
}

func TestRestoreBackup_ChmodError(t *testing.T) {
	resetSeams(t)
	dir := t.TempDir()
	bk := filepath.Join(dir, "b.bak")
	if err := os.WriteFile(bk, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	osChmodFileFn = func(*os.File, os.FileMode) error { return errors.New("chmod fail") }
	if err := RestoreBackup(bk, filepath.Join(dir, "out.kdbx")); err == nil || !strings.Contains(err.Error(), "cannot set database permissions") {
		t.Errorf("expected chmod error, got %v", err)
	}
}

func TestRestoreBackup_CopyError(t *testing.T) {
	resetSeams(t)
	dir := t.TempDir()
	bk := filepath.Join(dir, "b.bak")
	if err := os.WriteFile(bk, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	ioCopyFn = func(io.Writer, io.Reader) (int64, error) { return 0, errors.New("copy fail") }
	if err := RestoreBackup(bk, filepath.Join(dir, "out.kdbx")); err == nil || !strings.Contains(err.Error(), "copy fail") {
		t.Errorf("expected copy error, got %v", err)
	}
}

func TestPruneOldBackups_ListError(t *testing.T) {
	resetSeams(t)
	// Use a path with an invalid glob pattern to force filepath.Glob error.
	d := &DB{Path: "[invalid.kdbx"}
	if err := d.pruneOldBackups(); err == nil {
		// Glob with bad pattern returns ErrBadPattern.
		t.Error("expected glob error")
	}
}

func TestListBackups_GlobError(t *testing.T) {
	d := &DB{Path: "[bad.kdbx"}
	if _, err := d.ListBackups(); err == nil {
		t.Error("expected Glob error")
	}
}

func TestOpen_StatError(t *testing.T) {
	resetSeams(t)
	if _, err := Open(config.Config{Database: "/nonexistent/path.kdbx", Password: "pw"}); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not-found error, got %v", err)
	}
}

func TestOpen_OpenHook(t *testing.T) {
	resetSeams(t)
	called := false
	OpenHook = func(config.Config) (*DB, error) {
		called = true
		return &DB{}, nil
	}
	if _, err := Open(config.Config{Database: "/whatever"}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("OpenHook should have been called")
	}
}

func TestOpen_BuildCredsError(t *testing.T) {
	resetSeams(t)
	d := seedDB(t)
	path := writeKdbx(t, d, "pw")
	// No password, no keyfile, no password file → buildCreds inside tryOpen returns error.
	_, err := Open(config.Config{Database: path})
	if err == nil {
		t.Error("expected no-credentials error")
	}
}

func TestOpen_DecodeError(t *testing.T) {
	resetSeams(t)
	// Create a non-kdbx file at a real path.
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.kdbx")
	if err := os.WriteFile(bad, []byte("not a real kdbx file"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Open(config.Config{Database: bad, Password: "x"})
	if err == nil {
		t.Error("expected decode error")
	}
}

func TestOpen_PromptFlow(t *testing.T) {
	resetSeams(t)
	d := seedDB(t)
	path := writeKdbx(t, d, "prompted")
	prompted := false
	PasswordPrompter = func(string, bool) (string, error) {
		prompted = true
		return "prompted", nil
	}
	if _, err := Open(config.Config{Database: path}); err != nil {
		t.Fatal(err)
	}
	if !prompted {
		t.Error("PasswordPrompter should be called when no Password/PasswordFile")
	}
}

func TestOpen_CachedSucceeds(t *testing.T) {
	resetSeams(t)
	d := seedDB(t)
	path := writeKdbx(t, d, "pw")
	tmpRuntime := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", tmpRuntime)
	// Seed cache with the correct password.
	if err := cache.Store(path, "", "pw", 60); err != nil {
		t.Fatal(err)
	}
	loaded, err := Open(config.Config{Database: path, CacheTTL: 60})
	if err != nil {
		t.Fatal(err)
	}
	if loaded == nil {
		t.Error("expected loaded DB")
	}
}

func TestOpen_CachedWrongPasswordClearsAndPrompts(t *testing.T) {
	resetSeams(t)
	d := seedDB(t)
	path := writeKdbx(t, d, "real-pw")
	tmpRuntime := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", tmpRuntime)
	// Seed cache with wrong password.
	if err := cache.Store(path, "", "wrong-pw", 60); err != nil {
		t.Fatal(err)
	}
	prompted := false
	PasswordPrompter = func(string, bool) (string, error) {
		prompted = true
		return "real-pw", nil
	}
	if _, err := Open(config.Config{Database: path, CacheTTL: 60}); err != nil {
		t.Fatal(err)
	}
	if !prompted {
		t.Error("after cache failure, prompt should fire")
	}
}

func TestApplyFields_AllNonNil(t *testing.T) {
	d := seedDB(t)
	email := d.FindEntryByExactPath("work/email")
	if email == nil {
		t.Fatal("email not found")
	}
	u := "newuser"
	url := "https://new"
	notes := "newnotes"
	otp := "otpauth://totp/new?secret=AAA"
	d.ApplyFields(email, &u, &url, &notes, &otp, false)
	if email.Raw().GetContent("UserName") != "newuser" {
		t.Error("username not applied")
	}
	if email.Raw().GetContent("URL") != "https://new" {
		t.Error("url not applied")
	}
	if email.Raw().GetContent("Notes") != "newnotes" {
		t.Error("notes not applied")
	}
	if email.OtpURI() != otp {
		t.Error("otp not applied")
	}
}

func TestMoveEntry_DeleteErrorPropagates(t *testing.T) {
	d := seedDB(t)
	dst := d.EnsureGroup("dst")
	// Manually craft an entry whose parent doesn't contain it → DeleteEntry will fail.
	stray := &gokeepasslib.Entry{}
	parent := &gokeepasslib.Group{}
	bogus := &Entry{d: d, e: stray, parent: parent}
	if _, err := d.MoveEntry(bogus, dst); err == nil {
		t.Error("expected MoveEntry to propagate Delete error")
	}
}

func TestRemoveGroup_EmptyPathErrors(t *testing.T) {
	d := seedDB(t)
	if err := d.RemoveGroup(""); err == nil {
		t.Error("expected error for empty path (cannot remove root)")
	}
}

func TestRemoveGroupAt_EmptyPathNoOp(t *testing.T) {
	g := &gokeepasslib.Group{}
	if err := removeGroupAt(g, nil); err != nil {
		t.Errorf("empty path on removeGroupAt should be no-op, got %v", err)
	}
}

func TestCollectEmpty_NonEmptyPath(t *testing.T) {
	// Cover the "len(path) > 0" branch of collectEmpty by calling it directly with
	// a non-empty starting path. EmptyGroups always passes nil, so this branch is
	// only reachable via a direct helper call.
	g := &gokeepasslib.Group{Name: "leaf"}
	hasEntries := map[*gokeepasslib.Group]bool{}
	var out []string
	collectEmpty(g, []string{"parent"}, hasEntries, &out)
	if len(out) != 1 || out[0] != "parent/leaf" {
		t.Errorf("expected ['parent/leaf'], got %v", out)
	}
}

func TestIsEmptyGroup_NestedNonEmpty(t *testing.T) {
	// isEmptyGroup recurses into Groups; cover the inner "!isEmptyGroup" return-false branch.
	mid := &gokeepasslib.Group{}
	mid.Groups = append(mid.Groups, gokeepasslib.NewGroup())
	leafPtr := &mid.Groups[0]
	hasEntries := map[*gokeepasslib.Group]bool{leafPtr: true}
	if isEmptyGroup(mid, hasEntries) {
		t.Error("group whose child has entries must not be empty")
	}
}

func TestAttachmentContent_BinaryMissing(t *testing.T) {
	d := seedDB(t)
	email := d.FindEntryByExactPath("work/email")
	// Wipe the binary table but keep the binary reference on the entry.
	d.Raw.Content.InnerHeader.Binaries = nil
	if _, err := email.AttachmentContent("doc.txt"); err == nil || !strings.Contains(err.Error(), "binary missing") {
		t.Errorf("expected binary-missing error, got %v", err)
	}
}

func TestGetAttribute_ProtectedCustomFieldErrors(t *testing.T) {
	d := seedDB(t)
	email := d.FindEntryByExactPath("work/email")
	// Add a protected custom field whose Content is "" so GetContent returns ""
	// and we fall through to the Values-slice scan that flags the protection.
	email.Raw().Values = append(email.Raw().Values, gokeepasslib.ValueData{
		Key:   "SecretToken",
		Value: gokeepasslib.V{Content: "", Protected: w.NewBoolWrapper(true)},
	})
	if _, err := email.GetAttribute("SecretToken"); err == nil || !strings.Contains(err.Error(), "cannot read protected") {
		t.Errorf("expected protected-field error, got %v", err)
	}
}

func TestOpenSimple_BuildCredsError(t *testing.T) {
	resetSeams(t)
	// inlinePassword + keyFile both empty, but passwordFile points at a real but unrelated file → password is non-empty so buildCreds succeeds.
	// Trigger buildCreds error: pass a non-existent keyfile.
	d := seedDB(t)
	path := writeKdbx(t, d, "pw")
	_, err := OpenSimple(path, "", "/no/such/keyfile", "pw")
	if err == nil {
		t.Error("expected buildCreds error from missing keyfile")
	}
}

func TestOpenSimple_UnlockError(t *testing.T) {
	resetSeams(t)
	d := seedDB(t)
	path := writeKdbx(t, d, "pw")
	unlockProtectedFn = func(*gokeepasslib.Database) error { return errors.New("unlock fail") }
	if _, err := OpenSimple(path, "", "", "pw"); err == nil || !strings.Contains(err.Error(), "unlock fail") {
		t.Errorf("expected unlock error, got %v", err)
	}
}

func TestGetAttribute_CustomNonProtectedEmpty(t *testing.T) {
	d := seedDB(t)
	email := d.FindEntryByExactPath("work/email")
	// Empty-content non-protected custom field forces fall-through to the
	// Values-scan loop returning ("", nil).
	email.Raw().Values = append(email.Raw().Values, gokeepasslib.ValueData{
		Key:   "EmptyCustom",
		Value: gokeepasslib.V{Content: "", Protected: w.NewBoolWrapper(false)},
	})
	val, err := email.GetAttribute("EmptyCustom")
	if err != nil {
		t.Fatal(err)
	}
	if val != "" {
		t.Errorf("value = %q, want empty", val)
	}
}

func TestOpen_TryOpenBuildCredsError(t *testing.T) {
	resetSeams(t)
	d := seedDB(t)
	path := writeKdbx(t, d, "pw")
	// PasswordPrompter returns empty → tryOpen("") → buildCreds("","") fails.
	PasswordPrompter = func(string, bool) (string, error) { return "", nil }
	if _, err := Open(config.Config{Database: path}); err == nil {
		t.Error("expected buildCreds error to propagate")
	}
}

func TestOpen_TryOpenOpenError(t *testing.T) {
	resetSeams(t)
	d := seedDB(t)
	path := writeKdbx(t, d, "pw")
	osOpenFn = func(string) (*os.File, error) { return nil, errors.New("open fail") }
	if _, err := Open(config.Config{Database: path, Password: "pw"}); err == nil {
		t.Error("expected open error inside tryOpen")
	}
}

func TestOpen_TryOpenUnlockError(t *testing.T) {
	resetSeams(t)
	d := seedDB(t)
	path := writeKdbx(t, d, "pw")
	unlockProtectedFn = func(*gokeepasslib.Database) error { return errors.New("unlock fail") }
	if _, err := Open(config.Config{Database: path, Password: "pw"}); err == nil {
		t.Error("expected unlock error inside tryOpen")
	}
}

func TestSave_CloseError(t *testing.T) {
	resetSeams(t)
	d := seedDB(t)
	path := writeKdbx(t, d, "pw")
	loaded, err := OpenSimple(path, "", "", "pw")
	if err != nil {
		t.Fatal(err)
	}
	// encodeFn closes the file early so the subsequent tmp.Close errors.
	encodeFn = func(wr io.Writer, _ *gokeepasslib.Database) error {
		if f, ok := wr.(*os.File); ok {
			_ = f.Close()
		}
		return nil
	}
	if err := loaded.Save(); err == nil {
		t.Error("expected tmp.Close error")
	}
}

// buildSrcWithBadBinary returns a source entry whose database has both a
// non-existent binary ref AND a binary whose content fails GetContentBytes
// (bad gzip).
func buildSrcWithBadBinary(t *testing.T) *Entry {
	t.Helper()
	src := &DB{Raw: gokeepasslib.NewDatabase(gokeepasslib.WithDatabaseKDBXVersion4())}
	src.Raw.Credentials = gokeepasslib.NewPasswordCredentials("pw")
	srcRoot := &src.Raw.Content.Root.Groups[0]
	srcRoot.Entries = nil
	srcRoot.Groups = nil
	g := gokeepasslib.NewGroup()
	g.Name = "src"
	seedEntry(&g, "item", "u", "p", "", "", "")
	// Add a custom (non-standard) key to exercise replaceEntryData's append branch.
	g.Entries[0].Values = append(g.Entries[0].Values, gokeepasslib.ValueData{
		Key: "Pin", Value: gokeepasslib.V{Content: "1234"},
	})
	// Ref 1: missing binary → bin == nil branch.
	g.Entries[0].Binaries = append(g.Entries[0].Binaries, gokeepasslib.NewBinaryReference("ghost.bin", 999))
	// Ref 2: present binary but GetContentBytes fails (Compressed=true + bad gzip body).
	src.Raw.Content.InnerHeader.Binaries = append(src.Raw.Content.InnerHeader.Binaries, gokeepasslib.Binary{
		ID:         42,
		Content:    []byte("not-gzip"),
		Compressed: w.NewBoolWrapper(true),
	})
	g.Entries[0].Binaries = append(g.Entries[0].Binaries, gokeepasslib.NewBinaryReference("bad.bin", 42))
	srcRoot.Groups = append(srcRoot.Groups, g)

	var srcEntry *Entry
	for _, e := range src.SortedEntries() {
		if e.Raw().GetTitle() == "item" {
			srcEntry = e
		}
	}
	if srcEntry == nil {
		t.Fatal("source entry not found via SortedEntries")
	}
	return srcEntry
}

func TestImportEntry_SkipsMissingAndUnreadableBinaries(t *testing.T) {
	srcEntry := buildSrcWithBadBinary(t)
	target := seedDB(t)
	target.importEntry(srcEntry, "imported/item")
}

func TestReplaceEntryData_BinaryAndStandardBranches(t *testing.T) {
	srcEntry := buildSrcWithBadBinary(t)
	target := seedDB(t)
	tgt := target.FindEntryByExactPath("work/email")
	if tgt == nil {
		t.Fatal("target entry not found")
	}
	target.replaceEntryData(tgt, srcEntry)
	// Standard-key skip + binary-missing + bad-binary branches all reached.
	if len(tgt.Raw().Binaries) != 0 {
		t.Errorf("expected 0 binaries after replace (all sources unusable), got %d", len(tgt.Raw().Binaries))
	}
	// Custom "Pin" value should be appended.
	foundPin := false
	for _, v := range tgt.Raw().Values {
		if v.Key == "Pin" && v.Value.Content == "1234" {
			foundPin = true
		}
	}
	if !foundPin {
		t.Error("custom Pin value should be appended via the non-standard branch")
	}
}
