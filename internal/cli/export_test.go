package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExportOutputUsesPrivateMode(t *testing.T) {
	f := newFixture(t)
	outPath := filepath.Join(f.root, "export.json")
	_, stderr, code := f.runCLI("export", "--output", outPath)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode=%#o, want 0600", got)
	}
}

func TestExportForceTightensExistingFileMode(t *testing.T) {
	f := newFixture(t)
	outPath := filepath.Join(f.root, "export.json")
	if err := os.WriteFile(outPath, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, stderr, code := f.runCLI("export", "--output", outPath, "--force")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode=%#o, want 0600", got)
	}
}
