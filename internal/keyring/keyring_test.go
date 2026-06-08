package keyring

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// reset restores the backend seams after each test.
func reset(t *testing.T) {
	t.Helper()
	origSet, origGet, origDel := backendSet, backendGet, backendDelete
	t.Cleanup(func() {
		backendSet, backendGet, backendDelete = origSet, origGet, origDel
	})
}

func TestAccount_DatabaseOnly(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "x.kdbx")
	if err := os.WriteFile(dbPath, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := Account(dbPath, "")
	if got != dbPath {
		t.Errorf("Account = %q, want %q", got, dbPath)
	}
}

func TestAccount_WithKeyFile(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "x.kdbx")
	keyPath := filepath.Join(dir, "x.key")
	for _, p := range []string{dbPath, keyPath} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got := Account(dbPath, keyPath)
	if !strings.Contains(got, "::") || !strings.HasPrefix(got, dbPath) {
		t.Errorf("Account = %q, want %q::%q", got, dbPath, keyPath)
	}
}

func TestAccount_NonexistentPathStillResolves(t *testing.T) {
	// EvalSymlinks fails on a missing path; Account should fall back to the
	// absolute path rather than returning empty.
	got := Account("/no/such/db.kdbx", "")
	if got == "" {
		t.Error("Account returned empty for missing path")
	}
}

func TestSetGetDelete_Passthrough(t *testing.T) {
	reset(t)
	var gotService, gotUser, gotPw string
	backendSet = func(s, u, p string) error { gotService, gotUser, gotPw = s, u, p; return nil }
	backendGet = func(string, string) (string, error) { return "stored", nil }
	var deleted bool
	backendDelete = func(string, string) error { deleted = true; return nil }

	if err := Set("acct", "secret"); err != nil {
		t.Fatal(err)
	}
	if gotService != Service || gotUser != "acct" || gotPw != "secret" {
		t.Errorf("Set passed (%q,%q,%q)", gotService, gotUser, gotPw)
	}
	pw, err := Get("acct")
	if err != nil || pw != "stored" {
		t.Errorf("Get = %q, %v", pw, err)
	}
	if err := Delete("acct"); err != nil || !deleted {
		t.Errorf("Delete err=%v deleted=%v", err, deleted)
	}
}

func TestDelete_NotFoundIsNil(t *testing.T) {
	reset(t)
	backendDelete = func(string, string) error { return ErrNotFound }
	if err := Delete("missing"); err != nil {
		t.Errorf("Delete of missing should be nil, got %v", err)
	}
}

func TestDelete_OtherErrorPropagates(t *testing.T) {
	reset(t)
	backendDelete = func(string, string) error { return errors.New("dbus down") }
	if err := Delete("x"); err == nil {
		t.Error("expected delete error to propagate")
	}
}

func TestAvailable_NotFoundMeansReachable(t *testing.T) {
	reset(t)
	backendGet = func(string, string) (string, error) { return "", ErrNotFound }
	if err := Available(); err != nil {
		t.Errorf("Available should be nil on ErrNotFound, got %v", err)
	}
}

func TestAvailable_OKMeansReachable(t *testing.T) {
	reset(t)
	backendGet = func(string, string) (string, error) { return "x", nil }
	if err := Available(); err != nil {
		t.Errorf("Available should be nil when probe returns, got %v", err)
	}
}

func TestAvailable_BackendErrorUnavailable(t *testing.T) {
	reset(t)
	backendGet = func(string, string) (string, error) { return "", errors.New("no provider") }
	if err := Available(); err == nil {
		t.Error("expected Available to report backend error")
	}
}
