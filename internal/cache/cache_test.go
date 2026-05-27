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

func TestPath_WithKeyfile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", dir)

	p1, err := Path("/tmp/db.kdbx", "")
	if err != nil {
		t.Fatal(err)
	}
	p2, err := Path("/tmp/db.kdbx", "/tmp/keyfile.key")
	if err != nil {
		t.Fatal(err)
	}
	if p1 == p2 {
		t.Errorf("keyfile must change cache path, got identical %q", p1)
	}
	if filepath.Dir(p1) != filepath.Join(dir, "kpass") {
		t.Errorf("cache dir = %q, want %q", filepath.Dir(p1), filepath.Join(dir, "kpass"))
	}
	info, err := os.Stat(filepath.Join(dir, "kpass"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Errorf("cache dir mode = %#o, want 0700", info.Mode().Perm())
	}
}

func TestPath_MkdirError(t *testing.T) {
	// Point XDG_RUNTIME_DIR at a regular file so MkdirAll fails.
	dir := t.TempDir()
	conflict := filepath.Join(dir, "conflict")
	if err := os.WriteFile(conflict, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_RUNTIME_DIR", conflict)
	if _, err := Path("/tmp/db.kdbx", ""); err == nil {
		t.Error("expected MkdirAll error, got nil")
	}
}

func TestLoad_CorruptJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", dir)

	dbPath := filepath.Join(dir, "test.kdbx")
	if err := os.WriteFile(dbPath, []byte("fake"), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := Path(dbPath, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	pw, err := Load(dbPath, "")
	if err != nil {
		t.Fatalf("Load on corrupt = %v, want nil", err)
	}
	if pw != "" {
		t.Errorf("password from corrupt cache = %q, want empty", pw)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Errorf("corrupt cache file should be removed, stat err=%v", err)
	}
}

func TestLoad_EmptyPasswordTreatedAsMiss(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", dir)

	dbPath := filepath.Join(dir, "test.kdbx")
	if err := os.WriteFile(dbPath, []byte("fake"), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := Path(dbPath, "")
	if err != nil {
		t.Fatal(err)
	}
	// expires_at far in future, password empty.
	if err := os.WriteFile(p, []byte(`{"expires_at":9999999999,"password":""}`), 0o600); err != nil {
		t.Fatal(err)
	}

	pw, err := Load(dbPath, "")
	if err != nil {
		t.Fatal(err)
	}
	if pw != "" {
		t.Errorf("password = %q, want empty (sentinel miss)", pw)
	}
}

func TestLoad_PathIsDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", dir)
	dbPath := filepath.Join(dir, "test.kdbx")
	if err := os.WriteFile(dbPath, []byte("fake"), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := Path(dbPath, "")
	if err != nil {
		t.Fatal(err)
	}
	// Replace cache file with a dir to exercise info.IsDir() branch.
	if err := os.MkdirAll(p, 0o700); err != nil {
		t.Fatal(err)
	}
	pw, err := Load(dbPath, "")
	if err != nil {
		t.Fatal(err)
	}
	if pw != "" {
		t.Errorf("password = %q, want empty (dir → miss)", pw)
	}
}

func TestLoad_NoRuntimeDir(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	pw, err := Load("/tmp/db.kdbx", "")
	if err != nil {
		t.Fatal(err)
	}
	if pw != "" {
		t.Errorf("Load without XDG should return empty, got %q", pw)
	}
}

func TestStore_NoRuntimeDir(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	if err := Store("/tmp/db.kdbx", "", "secret", 300); err != nil {
		t.Errorf("Store without XDG should be a no-op, got err=%v", err)
	}
}

func TestClear_NoRuntimeDir(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	ok, err := Clear("/tmp/db.kdbx", "")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("Clear without XDG should return false")
	}
}

func TestStore_PathError(t *testing.T) {
	// Make XDG_RUNTIME_DIR a file so Path's MkdirAll fails → Store returns err.
	dir := t.TempDir()
	conflict := filepath.Join(dir, "conflict")
	if err := os.WriteFile(conflict, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_RUNTIME_DIR", conflict)
	if err := Store("/tmp/db.kdbx", "", "secret", 300); err == nil {
		t.Error("expected Path error to bubble through Store, got nil")
	}
}

func TestAbsResolve_FollowsSymlink(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real.kdbx")
	link := filepath.Join(dir, "link.kdbx")
	if err := os.WriteFile(real, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	t.Setenv("XDG_RUNTIME_DIR", dir)
	pReal, err := Path(real, "")
	if err != nil {
		t.Fatal(err)
	}
	pLink, err := Path(link, "")
	if err != nil {
		t.Fatal(err)
	}
	if pReal != pLink {
		t.Errorf("symlink and real path should hash to same cache key:\n  real=%q\n  link=%q", pReal, pLink)
	}
}

func TestAbsResolve_NonExistent(t *testing.T) {
	// Non-existent path → EvalSymlinks fails → absResolve falls back to abs.
	dir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", dir)
	if _, err := Path(filepath.Join(dir, "missing.kdbx"), ""); err != nil {
		t.Errorf("Path with missing file should still succeed, got %v", err)
	}
}
