package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/irasikhin/kpass/internal/config"
	"github.com/irasikhin/kpass/internal/runtimex"
)

func TestInitWritesLoadableConfig(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.toml")
	dbPath := filepath.Join(root, "init.kdbx")
	t.Setenv("KPASS_CONFIG", cfgPath)

	prev := runtimex.PromptHook
	runtimex.PromptHook = func(prompt string, confirm bool) (string, error) {
		return "master-password", nil
	}
	t.Cleanup(func() { runtimex.PromptHook = prev })

	stdout, stderr, code := (&fixture{t: t}).runCLIWith(runOpts{
		skipDatabaseArg:  true,
		skipPasswordFile: true,
	}, "init", "--path", dbPath)
	if code != 0 {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}

	fc, _, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if fc.DefaultDatabase != "default" {
		t.Fatalf("default=%q", fc.DefaultDatabase)
	}
	if fc.Databases["default"].Database != dbPath {
		t.Fatalf("database=%q", fc.Databases["default"].Database)
	}
}

func TestInitForceOverwritesExisting(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.toml")
	dbPath := filepath.Join(root, "init.kdbx")
	t.Setenv("KPASS_CONFIG", cfgPath)

	prev := runtimex.PromptHook
	runtimex.PromptHook = func(prompt string, confirm bool) (string, error) {
		return "master-password", nil
	}
	t.Cleanup(func() { runtimex.PromptHook = prev })

	if err := os.WriteFile(dbPath, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}

	f := &fixture{t: t}

	_, stderr, code := f.runCLIWith(runOpts{skipDatabaseArg: true, skipPasswordFile: true}, "init", "--path", dbPath)
	if code == 0 {
		t.Fatalf("expected failure without --force, got code=0 stderr=%s", stderr)
	}
	if !strings.Contains(stderr, "use --force") {
		t.Fatalf("stderr=%q", stderr)
	}

	stdout, stderr, code := f.runCLIWith(runOpts{skipDatabaseArg: true, skipPasswordFile: true}, "init", "--path", dbPath, "--force")
	if code != 0 {
		t.Fatalf("--force code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	info, err := os.Stat(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() < 100 {
		t.Fatalf("expected a real kdbx written; size=%d", info.Size())
	}
}
