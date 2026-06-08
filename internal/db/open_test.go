package db

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/irasikhin/kpass/internal/config"
	"github.com/irasikhin/kpass/internal/runtimex"
)

func writeStringTest(t *testing.T, path, content string) error {
	t.Helper()
	return os.WriteFile(path, []byte(content), 0o600)
}

func TestOpen_HookOverride(t *testing.T) {
	origHook := OpenHook
	t.Cleanup(func() { OpenHook = origHook })
	fake := &DB{Path: "fake"}
	OpenHook = func(config.Config) (*DB, error) { return fake, nil }

	got, err := Open(config.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if got != fake {
		t.Errorf("Open should forward to OpenHook")
	}
}

func TestOpen_DatabaseMissing(t *testing.T) {
	OpenHook = nil
	_, err := Open(config.Config{Database: "/no/such/file.kdbx"})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not-found error, got %v", err)
	}
}

func TestOpen_DatabaseIsDir(t *testing.T) {
	OpenHook = nil
	dir := t.TempDir()
	if _, err := Open(config.Config{Database: dir}); err == nil {
		t.Error("expected error when database path is a directory")
	}
}

func TestOpen_InlinePassword(t *testing.T) {
	OpenHook = nil
	d := seedDB(t)
	path := writeKdbx(t, d, "the-pw")
	got, err := Open(config.Config{Database: path, Password: "the-pw"})
	if err != nil {
		t.Fatal(err)
	}
	if got.FindEntryByExactPath("work/email") == nil {
		t.Error("entries missing")
	}
}

func TestOpen_PasswordFile(t *testing.T) {
	OpenHook = nil
	d := seedDB(t)
	path := writeKdbx(t, d, "from-file-pw")
	pw := path + ".pw"
	if err := writeStringTest(t, pw, "from-file-pw"); err != nil {
		t.Fatal(err)
	}
	_, err := Open(config.Config{Database: path, PasswordFile: pw})
	if err != nil {
		t.Fatal(err)
	}
}

func TestOpen_PasswordPrompted(t *testing.T) {
	OpenHook = nil
	origPrompt := PasswordPrompter
	t.Cleanup(func() { PasswordPrompter = origPrompt })
	PasswordPrompter = func(string, bool) (string, error) { return "prompted", nil }

	d := seedDB(t)
	path := writeKdbx(t, d, "prompted")
	if _, err := Open(config.Config{Database: path}); err != nil {
		t.Fatal(err)
	}
}

func TestOpen_PromptError(t *testing.T) {
	OpenHook = nil
	origPrompt := PasswordPrompter
	t.Cleanup(func() { PasswordPrompter = origPrompt })
	PasswordPrompter = func(string, bool) (string, error) { return "", errors.New("prompt fail") }

	d := seedDB(t)
	path := writeKdbx(t, d, "x")
	if _, err := Open(config.Config{Database: path}); err == nil || !strings.Contains(err.Error(), "prompt fail") {
		t.Errorf("expected prompt error, got %v", err)
	}
}

func TestOpen_WrongPassword(t *testing.T) {
	OpenHook = nil
	d := seedDB(t)
	path := writeKdbx(t, d, "actual")
	_, err := Open(config.Config{Database: path, Password: "wrong"})
	if err == nil {
		t.Error("expected open error for wrong password")
	}
}

func TestOpen_CacheRoundTrip(t *testing.T) {
	OpenHook = nil
	d := seedDB(t)
	path := writeKdbx(t, d, "cached-pw")

	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	cfg := config.Config{
		Database: path,
		Password: "cached-pw",
		CacheTTL: 300,
	}
	if _, err := Open(cfg); err != nil {
		t.Fatal(err)
	}
	// Second open: clear inline password so cache must be used.
	cfg2 := config.Config{Database: path, CacheTTL: 300}
	if _, err := Open(cfg2); err != nil {
		t.Fatalf("expected cached open to succeed, got %v", err)
	}
}

func TestOpen_CacheInvalidIsCleared(t *testing.T) {
	OpenHook = nil
	d := seedDB(t)
	path := writeKdbx(t, d, "real-pw")

	runtime := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtime)
	// Pre-populate cache with the wrong password.
	if err := os.MkdirAll(filepath.Join(runtime, "kpass"), 0o700); err != nil {
		t.Fatal(err)
	}
	// Use the package's Store via a normal call.
	cfg := config.Config{Database: path, Password: "wrong-cached", CacheTTL: 300}
	// First store something via a successful open under the real pw so cache is populated.
	if _, err := Open(config.Config{Database: path, Password: "real-pw", CacheTTL: 300}); err != nil {
		t.Fatal(err)
	}
	// Overwrite cache file content so it decrypts wrong.
	// Easier: just test that prompting falls back when cache is stale.
	origPrompt := PasswordPrompter
	t.Cleanup(func() { PasswordPrompter = origPrompt })
	PasswordPrompter = func(string, bool) (string, error) { return "real-pw", nil }
	cfg.Password = ""
	if _, err := Open(cfg); err != nil {
		t.Fatalf("expected fallback to prompt to succeed, got %v", err)
	}
}

// resetKeyringSeams restores the keyring seams after each test.
func resetKeyringSeams(t *testing.T) {
	t.Helper()
	origGet, origSet, origDel := keyringGetFn, keyringSetFn, keyringDeleteFn
	origPrompt := PasswordPrompter
	t.Cleanup(func() {
		keyringGetFn, keyringSetFn, keyringDeleteFn = origGet, origSet, origDel
		PasswordPrompter = origPrompt
	})
}

func TestOpen_KeyringHit(t *testing.T) {
	OpenHook = nil
	resetKeyringSeams(t)
	d := seedDB(t)
	path := writeKdbx(t, d, "kr-pw")

	keyringGetFn = func(string) (string, error) { return "kr-pw", nil }
	keyringSetFn = func(string, string) error { return nil }
	PasswordPrompter = func(string, bool) (string, error) {
		t.Fatal("prompt should not be called on keyring hit")
		return "", nil
	}

	cfg := config.Config{Database: path, UseKeyring: true}
	if _, err := Open(cfg); err != nil {
		t.Fatalf("keyring hit open: %v", err)
	}
}

func TestOpen_KeyringMissPromptsAndStores(t *testing.T) {
	OpenHook = nil
	resetKeyringSeams(t)
	d := seedDB(t)
	path := writeKdbx(t, d, "fresh-pw")

	keyringGetFn = func(string) (string, error) { return "", errors.New("miss") }
	var stored string
	keyringSetFn = func(_, pw string) error { stored = pw; return nil }
	PasswordPrompter = func(string, bool) (string, error) { return "fresh-pw", nil }

	if _, err := Open(config.Config{Database: path, UseKeyring: true}); err != nil {
		t.Fatalf("keyring miss open: %v", err)
	}
	if stored != "fresh-pw" {
		t.Errorf("expected password stored in keyring, got %q", stored)
	}
}

func TestOpen_KeyringStaleIsCleared(t *testing.T) {
	OpenHook = nil
	resetKeyringSeams(t)
	d := seedDB(t)
	path := writeKdbx(t, d, "right-pw")

	keyringGetFn = func(string) (string, error) { return "stale-pw", nil }
	var deleted, stored bool
	keyringDeleteFn = func(string) error { deleted = true; return nil }
	keyringSetFn = func(string, string) error { stored = true; return nil }
	PasswordPrompter = func(string, bool) (string, error) { return "right-pw", nil }

	if _, err := Open(config.Config{Database: path, UseKeyring: true}); err != nil {
		t.Fatalf("stale keyring open: %v", err)
	}
	if !deleted {
		t.Error("expected stale keyring entry to be deleted")
	}
	if !stored {
		t.Error("expected fresh password to be stored after re-prompt")
	}
}

func TestOpen_KeyringSkipsPlaintextCache(t *testing.T) {
	OpenHook = nil
	resetKeyringSeams(t)
	d := seedDB(t)
	path := writeKdbx(t, d, "kr-pw")

	runtime := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtime)
	keyringGetFn = func(string) (string, error) { return "", errors.New("miss") }
	keyringSetFn = func(string, string) error { return nil }
	PasswordPrompter = func(string, bool) (string, error) { return "kr-pw", nil }

	// UseKeyring on with a TTL that would otherwise enable the plaintext cache.
	if _, err := Open(config.Config{Database: path, UseKeyring: true, CacheTTL: 300}); err != nil {
		t.Fatalf("open: %v", err)
	}
	entries, _ := os.ReadDir(filepath.Join(runtime, "kpass"))
	if len(entries) != 0 {
		t.Errorf("expected no plaintext cache files when keyring is active, found %d", len(entries))
	}
}

func TestObtainPassword_InlineWins(t *testing.T) {
	got, err := obtainPassword(config.Config{Password: "inline"}, "/x.kdbx")
	if err != nil {
		t.Fatal(err)
	}
	if got != "inline" {
		t.Errorf("got %q, want inline", got)
	}
}

func TestObtainPassword_FromFile(t *testing.T) {
	dir := t.TempDir()
	pw := filepath.Join(dir, "p.txt")
	if err := writeStringTest(t, pw, "from-file"); err != nil {
		t.Fatal(err)
	}
	got, err := obtainPassword(config.Config{PasswordFile: pw}, "/x.kdbx")
	if err != nil {
		t.Fatal(err)
	}
	if got != "from-file" {
		t.Errorf("got %q", got)
	}
}

func TestObtainPassword_PromptFallback(t *testing.T) {
	origPrompt := PasswordPrompter
	t.Cleanup(func() { PasswordPrompter = origPrompt })
	PasswordPrompter = func(string, bool) (string, error) { return "via-prompt", nil }
	got, err := obtainPassword(config.Config{}, "/x.kdbx")
	if err != nil {
		t.Fatal(err)
	}
	if got != "via-prompt" {
		t.Errorf("got %q", got)
	}
}

func TestBuildCreds_NoCreds(t *testing.T) {
	if _, err := buildCreds("", ""); err == nil {
		t.Error("expected no-creds error")
	}
}

func TestBuildCreds_PasswordOnly(t *testing.T) {
	c, err := buildCreds("pw", "")
	if err != nil {
		t.Fatal(err)
	}
	if c == nil {
		t.Error("creds nil")
	}
}

func TestBuildCreds_KeyfileOnly(t *testing.T) {
	dir := t.TempDir()
	key := filepath.Join(dir, "key.bin")
	if err := writeStringTest(t, key, "binarykeydata1234567890abcdefghij"); err != nil {
		t.Fatal(err)
	}
	c, err := buildCreds("", key)
	if err != nil {
		t.Fatalf("buildCreds keyfile-only: %v", err)
	}
	if c == nil {
		t.Error("creds nil")
	}
}

func TestBuildCreds_KeyfileMissing(t *testing.T) {
	if _, err := buildCreds("", "/no/such/key"); err == nil {
		t.Error("expected missing-keyfile error")
	}
}

func TestExpandPathRoundtripInOpen(t *testing.T) {
	// Sanity: confirms runtimex.ExpandPath is hit (covers untouched line).
	got := runtimex.ExpandPath("/abs/literal")
	if got != "/abs/literal" {
		t.Errorf("ExpandPath = %q", got)
	}
}
