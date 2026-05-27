package db

import (
	"os"
	"path/filepath"
	"testing"
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
