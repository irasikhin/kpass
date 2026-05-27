package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func modePerm(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}

func TestCreateUsesPrivateMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "created.kdbx")
	if err := Create(path, "secret", ""); err != nil {
		t.Fatal(err)
	}
	if got := modePerm(t, path); got != 0o600 {
		t.Fatalf("mode=%#o, want 0600", got)
	}
}

func TestBackupAndRestoreUsePrivateMode(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "main.kdbx")
	if err := os.WriteFile(dbPath, []byte("database"), 0o600); err != nil {
		t.Fatal(err)
	}

	d := &DB{Path: dbPath}
	backupPath, err := d.Backup()
	if err != nil {
		t.Fatal(err)
	}
	if got := modePerm(t, backupPath); got != 0o600 {
		t.Fatalf("backup mode=%#o, want 0600", got)
	}

	restorePath := filepath.Join(dir, "restored.kdbx")
	if err := RestoreBackup(backupPath, restorePath); err != nil {
		t.Fatal(err)
	}
	if got := modePerm(t, restorePath); got != 0o600 {
		t.Fatalf("restore mode=%#o, want 0600", got)
	}
}

func TestSave_RoundTrip(t *testing.T) {
	d := seedDB(t)
	path := writeKdbx(t, d, "rt-pw")

	// Open with the inline password, mutate, save, reopen, verify.
	OpenHook = nil
	loaded, err := OpenSimple(path, "", "", "rt-pw")
	if err != nil {
		t.Fatal(err)
	}
	work := loaded.EnsureGroup("work")
	loaded.CreateEntry(work, "added-after-load", "u", "added-pw", "", "", "")
	if err := loaded.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if got := modePerm(t, path); got != 0o600 {
		t.Errorf("post-save mode = %#o, want 0o600", got)
	}

	reopened, err := OpenSimple(path, "", "", "rt-pw")
	if err != nil {
		t.Fatal(err)
	}
	e := reopened.FindEntryByExactPath("work/added-after-load")
	if e == nil {
		t.Fatal("added entry missing after reopen")
	}
	if e.Raw().GetPassword() != "added-pw" {
		t.Errorf("password roundtrip: %q", e.Raw().GetPassword())
	}
}

func TestSave_CreatesBackupAndPrunes(t *testing.T) {
	d := seedDB(t)
	path := writeKdbx(t, d, "rt-pw")
	loaded, err := OpenSimple(path, "", "", "rt-pw")
	if err != nil {
		t.Fatal(err)
	}
	loaded.BackupKeep = 1
	loaded.BackupMaxAgeDays = 0

	// Three consecutive saves → up to 3 backups created, but BackupKeep=1
	// should reduce to 1 at the end.
	for i := range 3 {
		if err := loaded.Save(); err != nil {
			t.Fatalf("save %d: %v", i, err)
		}
		// Spin enough to ensure a different timestamp in the next backup name.
		time.Sleep(1100 * time.Millisecond)
	}
	backups, err := loaded.ListBackups()
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) > 1 {
		t.Errorf("expected at most 1 backup, got %d: %v", len(backups), backups)
	}
}

func TestSave_PrunesByAge(t *testing.T) {
	d := seedDB(t)
	path := writeKdbx(t, d, "rt-pw")
	loaded, err := OpenSimple(path, "", "", "rt-pw")
	if err != nil {
		t.Fatal(err)
	}
	loaded.BackupMaxAgeDays = 1

	// Create one "old" .bak file (mtime well in the past).
	oldBak := strings.TrimSuffix(path, filepath.Ext(path)) + ".19990101-000000.bak"
	if err := os.WriteFile(oldBak, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-72 * time.Hour)
	if err := os.Chtimes(oldBak, old, old); err != nil {
		t.Fatal(err)
	}

	if err := loaded.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(oldBak); !os.IsNotExist(err) {
		t.Errorf("old backup should be pruned, stat err=%v", err)
	}
}

func TestListBackups(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "main.kdbx")
	if err := os.WriteFile(dbPath, []byte("body"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Create three .bak files with different timestamps in name.
	for _, ts := range []string{"20260101-000000", "20260102-000000", "20260103-000000"} {
		p := filepath.Join(dir, "main."+ts+".bak")
		if err := os.WriteFile(p, []byte(ts), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	d := &DB{Path: dbPath}
	got, err := d.ListBackups()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("backups = %v", got)
	}
	// Newest first.
	if !strings.Contains(got[0], "20260103") {
		t.Errorf("expected newest first, got %v", got)
	}
}

func TestBackup_NoSourceFile(t *testing.T) {
	d := &DB{Path: filepath.Join(t.TempDir(), "missing.kdbx")}
	bk, err := d.Backup()
	if err != nil {
		t.Fatal(err)
	}
	if bk != "" {
		t.Errorf("missing file should return empty backup path, got %q", bk)
	}
}

func TestFilepathDir(t *testing.T) {
	if filepathDir("") != "." {
		t.Errorf("empty = %q", filepathDir(""))
	}
	if filepathDir("/a/b") != "/a" {
		t.Errorf("got %q", filepathDir("/a/b"))
	}
}

func TestRestoreTightensExistingFileMode(t *testing.T) {
	dir := t.TempDir()
	backupPath := filepath.Join(dir, "backup.kdbx")
	restorePath := filepath.Join(dir, "restore.kdbx")
	if err := os.WriteFile(backupPath, []byte("backup"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(restorePath, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := RestoreBackup(backupPath, restorePath); err != nil {
		t.Fatal(err)
	}
	if got := modePerm(t, restorePath); got != 0o600 {
		t.Fatalf("restore mode=%#o, want 0600", got)
	}
}
