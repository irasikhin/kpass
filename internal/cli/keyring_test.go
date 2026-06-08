package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// resetCLIKeyring restores the cli-package keyring seams after each test.
func resetCLIKeyring(t *testing.T) {
	t.Helper()
	oSet, oGet, oDel := keyringSetFn, keyringGetFn, keyringDeleteFn
	oAvail, oAcct, oPrompt := keyringAvailableFn, keyringAccountFn, keyringPromptFn
	t.Cleanup(func() {
		keyringSetFn, keyringGetFn, keyringDeleteFn = oSet, oGet, oDel
		keyringAvailableFn, keyringAccountFn, keyringPromptFn = oAvail, oAcct, oPrompt
	})
	// Default to a deterministic account so tests don't touch the real FS.
	keyringAccountFn = func(db, _ string) string { return db }
}

func keyringConfig(t *testing.T, f *fixture, useKeyring bool) string {
	t.Helper()
	cfg := filepath.Join(f.root, "kpass-keyring.toml")
	var b strings.Builder
	b.WriteString(`default = "main"` + "\n\n")
	b.WriteString("[databases.main]\n")
	b.WriteString(`database = "` + f.dbPath + `"` + "\n")
	if useKeyring {
		b.WriteString("use_keyring = true\n")
	}
	b.WriteString("\n")
	writeFile(t, cfg, b.String())
	return cfg
}

func TestKeyringSet_StoresAndEnables(t *testing.T) {
	f := newFixture(t)
	resetCLIKeyring(t)
	cfg := keyringConfig(t, f, false)

	var stored string
	keyringSetFn = func(_, pw string) error { stored = pw; return nil }
	keyringPromptFn = func(string, bool) (string, error) { return "master-password", nil }

	stdout, stderr, code := f.runCLIWith(runOpts{configFile: cfg, skipDatabaseArg: true, skipPasswordFile: true}, "keyring", "set")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if stored != "master-password" {
		t.Errorf("stored = %q, want master-password", stored)
	}
	if !strings.Contains(stdout, "Stored master password") {
		t.Errorf("missing confirmation: %s", stdout)
	}
	data, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "use_keyring = true") {
		t.Errorf("config not updated with use_keyring:\n%s", data)
	}
}

func TestKeyringSet_WrongPasswordRejected(t *testing.T) {
	f := newFixture(t)
	resetCLIKeyring(t)
	cfg := keyringConfig(t, f, false)

	var stored bool
	keyringSetFn = func(string, string) error { stored = true; return nil }
	keyringPromptFn = func(string, bool) (string, error) { return "definitely-wrong", nil }

	_, stderr, code := f.runCLIWith(runOpts{configFile: cfg, skipDatabaseArg: true, skipPasswordFile: true}, "keyring", "set")
	if code == 0 {
		t.Fatal("expected non-zero exit for wrong password")
	}
	if stored {
		t.Error("password must not be stored when it fails to unlock")
	}
	if !strings.Contains(stderr, "did not unlock") {
		t.Errorf("expected unlock-failure message, got %s", stderr)
	}
}

func TestKeyringSet_RejectsPasswordFileProfile(t *testing.T) {
	f := newFixture(t)
	resetCLIKeyring(t)
	keyringPromptFn = func(string, bool) (string, error) {
		t.Fatal("prompt should not run for a password_file profile")
		return "", nil
	}
	// Default runCLI injects --password-file, making cfg.PasswordFile non-empty.
	_, stderr, code := f.runCLIWith(runOpts{skipDatabaseArg: false}, "keyring", "set")
	if code == 0 {
		t.Fatal("expected rejection when password_file is set")
	}
	if !strings.Contains(stderr, "password_file") {
		t.Errorf("expected password_file rejection, got %s", stderr)
	}
}

func TestKeyringRm_DeletesAndDisables(t *testing.T) {
	f := newFixture(t)
	resetCLIKeyring(t)
	cfg := keyringConfig(t, f, true)

	var deleted bool
	keyringDeleteFn = func(string) error { deleted = true; return nil }

	stdout, stderr, code := f.runCLIWith(runOpts{configFile: cfg, skipDatabaseArg: true, skipPasswordFile: true}, "keyring", "rm")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if !deleted {
		t.Error("expected keyring delete to be called")
	}
	if !strings.Contains(stdout, "Removed master password") {
		t.Errorf("missing confirmation: %s", stdout)
	}
	data, _ := os.ReadFile(cfg)
	if strings.Contains(string(data), "use_keyring = true") {
		t.Errorf("use_keyring should be disabled in config:\n%s", data)
	}
}

func TestKeyringStatus_AvailableAndStored(t *testing.T) {
	f := newFixture(t)
	resetCLIKeyring(t)
	cfg := keyringConfig(t, f, true)

	keyringAvailableFn = func() error { return nil }
	keyringGetFn = func(string) (string, error) { return "secret", nil }

	stdout, stderr, code := f.runCLIWith(runOpts{configFile: cfg, skipDatabaseArg: true, skipPasswordFile: true}, "keyring", "status")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "available") || !strings.Contains(stdout, "yes") {
		t.Errorf("unexpected status output: %s", stdout)
	}
}

func TestKeyringStatus_WithSelector(t *testing.T) {
	f := newFixture(t)
	resetCLIKeyring(t)
	cfg := keyringConfig(t, f, true)

	keyringAvailableFn = func() error { return nil }
	keyringGetFn = func(string) (string, error) { return "", errors.New("miss") }

	stdout, stderr, code := f.runCLIWith(runOpts{configFile: cfg, skipDatabaseArg: true, skipPasswordFile: true}, "keyring", "status", "@main")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "available") {
		t.Errorf("expected available backend, got %s", stdout)
	}
}

func TestKeyringSet_StoreFailure(t *testing.T) {
	f := newFixture(t)
	resetCLIKeyring(t)
	cfg := keyringConfig(t, f, false)

	keyringSetFn = func(string, string) error { return errors.New("dbus down") }
	keyringPromptFn = func(string, bool) (string, error) { return "master-password", nil }

	_, stderr, code := f.runCLIWith(runOpts{configFile: cfg, skipDatabaseArg: true, skipPasswordFile: true}, "keyring", "set")
	if code == 0 {
		t.Fatal("expected failure when keyring store errors")
	}
	if !strings.Contains(stderr, "Failed to store") {
		t.Errorf("expected store-failure message, got %s", stderr)
	}
}

func TestKeyringSet_NoMatchingProfileWarns(t *testing.T) {
	f := newFixture(t)
	resetCLIKeyring(t)

	var stored bool
	keyringSetFn = func(string, string) error { stored = true; return nil }
	keyringPromptFn = func(string, bool) (string, error) { return "master-password", nil }

	// No config (default missing file) => no profile to enable; --database used.
	stdout, stderr, code := f.runCLIWith(runOpts{skipPasswordFile: true}, "keyring", "set")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if !stored {
		t.Error("secret should still be stored")
	}
	if !strings.Contains(stdout, "use_keyring was not enabled") {
		t.Errorf("expected no-profile warning, got %s", stdout)
	}
}

func TestKeyringSet_RejectsPasswordDatabaseProfile(t *testing.T) {
	f := newFixture(t)
	resetCLIKeyring(t)
	cfg := filepath.Join(f.root, "kpass-chain.toml")
	writeFile(t, cfg, strings.Join([]string{
		`default = "main"`,
		"",
		"[databases.main]",
		`database = "` + f.dbPath + `"`,
		`password_file = "` + f.passwordFile + `"`,
		"",
		"[databases.work]",
		`database = "` + f.workDBPath + `"`,
		`password_database = "main"`,
		`password_entry = "db-passwords/work"`,
		"",
	}, "\n"))

	keyringPromptFn = func(string, bool) (string, error) {
		t.Fatal("prompt should not run for a password_database profile")
		return "", nil
	}

	_, stderr, code := f.runCLIWith(runOpts{configFile: cfg, skipDatabaseArg: true, skipPasswordFile: true}, "keyring", "set", "@work")
	if code == 0 {
		t.Fatal("expected rejection for password_database profile")
	}
	if !strings.Contains(stderr, "password_database") {
		t.Errorf("expected password_database rejection, got %s", stderr)
	}
}

func TestKeyringStatus_JSONUnavailable(t *testing.T) {
	f := newFixture(t)
	resetCLIKeyring(t)
	cfg := keyringConfig(t, f, false)

	keyringAvailableFn = func() error { return errors.New("no provider") }

	stdout, stderr, code := f.runCLIWith(runOpts{configFile: cfg, skipDatabaseArg: true, skipPasswordFile: true}, "keyring", "status", "--json")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, `"backend_available": false`) {
		t.Errorf("expected backend_available false in JSON: %s", stdout)
	}
	if !strings.Contains(stdout, "no provider") {
		t.Errorf("expected error in JSON: %s", stdout)
	}
}
