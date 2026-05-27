package cache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/irasikhin/kpass/internal/config"
)

func TestEnabled_FalseWhenNoCache(t *testing.T) {
	cfg := config.Config{NoCache: true, CacheTTL: 300}
	t.Setenv("XDG_RUNTIME_DIR", "/tmp")
	if Enabled(cfg) {
		t.Error("should be disabled when NoCache=true")
	}
}

func TestEnabled_FalseWhenTTLZero(t *testing.T) {
	cfg := config.Config{CacheTTL: 0}
	t.Setenv("XDG_RUNTIME_DIR", "/tmp")
	if Enabled(cfg) {
		t.Error("should be disabled when TTL <= 0")
	}
}

func TestEnabled_FalseWhenPasswordFile(t *testing.T) {
	cfg := config.Config{PasswordFile: "/tmp/pw.txt", CacheTTL: 300}
	t.Setenv("XDG_RUNTIME_DIR", "/tmp")
	if Enabled(cfg) {
		t.Error("should be disabled when password_file is set")
	}
}

func TestEnabled_FalseWhenNoRuntimeDir(t *testing.T) {
	cfg := config.Config{CacheTTL: 300}
	t.Setenv("XDG_RUNTIME_DIR", "")
	if Enabled(cfg) {
		t.Error("should be disabled when XDG_RUNTIME_DIR is unset")
	}
}

func TestEnabled_True(t *testing.T) {
	cfg := config.Config{CacheTTL: 300}
	t.Setenv("XDG_RUNTIME_DIR", "/tmp")
	if !Enabled(cfg) {
		t.Error("should be enabled")
	}
}

func TestPath_NoRuntimeDir(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	p, err := Path("/tmp/test.kdbx", "")
	if err != nil {
		t.Fatal(err)
	}
	if p != "" {
		t.Errorf("path should be empty when no runtime dir, got %q", p)
	}
}

func TestStoreLoadClear(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", dir)

	dbPath := filepath.Join(dir, "test.kdbx")
	if err := os.WriteFile(dbPath, []byte("fake"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Store.
	if err := Store(dbPath, "", "secret123", 300); err != nil {
		t.Fatalf("Store error: %v", err)
	}

	// Load.
	pw, err := Load(dbPath, "")
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if pw != "secret123" {
		t.Errorf("password = %q, want secret123", pw)
	}

	// Clear.
	ok, err := Clear(dbPath, "")
	if err != nil {
		t.Fatalf("Clear error: %v", err)
	}
	if !ok {
		t.Error("Clear should return true")
	}

	// Load after clear.
	pw, err = Load(dbPath, "")
	if err != nil {
		t.Fatal(err)
	}
	if pw != "" {
		t.Errorf("password after clear = %q, want empty", pw)
	}
}

func TestLoad_Expired(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", dir)

	dbPath := filepath.Join(dir, "test.kdbx")
	if err := os.WriteFile(dbPath, []byte("fake"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Store with 0 TTL (expires immediately).
	if err := Store(dbPath, "", "expired", 0); err != nil {
		t.Fatalf("Store error: %v", err)
	}

	pw, err := Load(dbPath, "")
	if err != nil {
		t.Fatal(err)
	}
	if pw != "" {
		t.Errorf("expired password = %q, want empty", pw)
	}
}

func TestClearAll(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", dir)

	db1 := filepath.Join(dir, "a.kdbx")
	db2 := filepath.Join(dir, "b.kdbx")
	if err := os.WriteFile(db1, []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(db2, []byte("b"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Store(db1, "", "pw1", 300); err != nil {
		t.Fatal(err)
	}
	if err := Store(db2, "", "pw2", 300); err != nil {
		t.Fatal(err)
	}

	n, err := ClearAll()
	if err != nil {
		t.Fatal(err)
	}
	if n < 2 {
		t.Errorf("ClearAll removed %d files, want >= 2", n)
	}
}

func TestClear_Missing(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	ok, err := Clear("/nonexistent/db.kdbx", "")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("Clear missing should return false")
	}
}
