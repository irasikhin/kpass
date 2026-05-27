package cli

import (
	"path/filepath"
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
