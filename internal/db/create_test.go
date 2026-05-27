package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreate_EmptyPath(t *testing.T) {
	if err := Create("", "pw", ""); err == nil || !strings.Contains(err.Error(), "cannot be empty") {
		t.Errorf("expected empty-path error, got %v", err)
	}
}

func TestCreate_AlreadyExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.kdbx")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Create(path, "pw", ""); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected already-exists error, got %v", err)
	}
}

func TestCreate_MkdirError(t *testing.T) {
	dir := t.TempDir()
	conflict := filepath.Join(dir, "conflict")
	if err := os.WriteFile(conflict, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	// Parent is a file → MkdirAll on subdir fails.
	target := filepath.Join(conflict, "sub", "db.kdbx")
	if err := Create(target, "pw", ""); err == nil || !strings.Contains(err.Error(), "cannot create directory") {
		t.Errorf("expected mkdir error, got %v", err)
	}
}

func TestCreate_NoCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nocreds.kdbx")
	if err := Create(path, "", ""); err == nil || !strings.Contains(err.Error(), "no credentials") {
		t.Errorf("expected no-credentials error, got %v", err)
	}
}

func TestCreate_NoExtensionBase(t *testing.T) {
	// Base name without an extension → rootName falls through to the "else" branch.
	path := filepath.Join(t.TempDir(), "noext")
	if err := Create(path, "pw", ""); err != nil {
		t.Fatalf("Create no-ext: %v", err)
	}
}

func TestCreate_OpenFileError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses dir mode 0o500")
	}
	dir := t.TempDir()
	subdir := filepath.Join(dir, "sub")
	if err := os.MkdirAll(subdir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(subdir, 0o700) })
	target := filepath.Join(subdir, "db.kdbx")
	// Stat sees no file; MkdirAll on an existing dir with 0o500 succeeds; OpenFile fails because dir is read-only.
	if err := Create(target, "pw", ""); err == nil || !strings.Contains(err.Error(), "cannot create database file") {
		t.Errorf("expected OpenFile error, got %v", err)
	}
}
