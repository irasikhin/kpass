package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/irasikhin/kpass/internal/config"
	"github.com/irasikhin/kpass/internal/picker"
)

func TestAttachAddLsExtractAndRemove(t *testing.T) {
	f := newFixture(t)
	pdfPath := filepath.Join(f.root, "sample.pdf")
	pdfBytes := []byte("%PDF-1.4\nsample pdf\n")
	if err := os.WriteFile(pdfPath, pdfBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	extracted := filepath.Join(f.root, "out", "sample.pdf")

	stdout, stderr, code := f.runCLI("attach", "add", "internet/email", pdfPath)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "Added attachment sample.pdf to internet/email") {
		t.Fatalf("stdout=%q", stdout)
	}

	stdout, stderr, code = f.runCLI("attach", "ls", "internet/email")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if strings.TrimSpace(stdout) != "sample.pdf" {
		t.Fatalf("stdout=%q", stdout)
	}

	_, stderr, code = f.runCLI("attach", "extract", "internet/email", "sample.pdf", extracted)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	got, err := os.ReadFile(extracted)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, pdfBytes) {
		t.Fatalf("contents=%q", got)
	}

	stdout, stderr, code = f.runCLI("attach", "remove", "internet/email", "sample.pdf")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "Removed attachment sample.pdf from internet/email") {
		t.Fatalf("stdout=%q", stdout)
	}

	stdout, stderr, code = f.runCLI("attach", "ls", "internet/email")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("stdout=%q", stdout)
	}
}

func TestAttachAddForceReplaces(t *testing.T) {
	f := newFixture(t)
	first := filepath.Join(f.root, "first.pdf")
	second := filepath.Join(f.root, "second.pdf")
	extracted := filepath.Join(f.root, "replaced.pdf")
	if err := os.WriteFile(first, []byte("first-pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("second-pdf"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, stderr, code := f.runCLI("attach", "add", "simple", first, "--name", "doc.pdf")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}

	_, stderr, code = f.runCLI("attach", "add", "simple", second, "--name", "doc.pdf")
	if code != 1 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(stderr, "attachment already exists: doc.pdf. Use --force to replace it") {
		t.Fatalf("stderr=%q", stderr)
	}

	_, stderr, code = f.runCLI("attach", "add", "simple", second, "--name", "doc.pdf", "--force")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}

	_, stderr, code = f.runCLI("attach", "extract", "simple", "doc.pdf", extracted)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	got, err := os.ReadFile(extracted)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "second-pdf" {
		t.Fatalf("contents=%q", got)
	}
}

func TestAttachAddWarnsForLargeFiles(t *testing.T) {
	f := newFixture(t)
	largePath := filepath.Join(f.root, "large.pdf")
	if err := os.WriteFile(largePath, bytes.Repeat([]byte{'x'}, config.LargeAttachmentWarnBytes), 0o600); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := f.runCLI("attach", "add", "simple", largePath)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(stderr, "Warning: attachment 'large.pdf' is 5.0 MiB and will increase the KeePass database size.") {
		t.Fatalf("stderr=%q", stderr)
	}
}

func TestPickDefaultsToCopyPassword(t *testing.T) {
	f := newFixture(t)
	var capturedPaths []string
	prev := picker.Hook
	picker.Hook = func(paths []string, query string) (string, error) {
		capturedPaths = append([]string(nil), paths...)
		return "internet/email", nil
	}
	t.Cleanup(func() { picker.Hook = prev })

	var copyValue string
	var copyTimeout int
	ClipboardWriter = func(v string, to int) error {
		copyValue = v
		copyTimeout = to
		return nil
	}

	stdout, stderr, code := f.runCLI("pick")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "Copied password for internet/email to clipboard") {
		t.Fatalf("stdout=%q", stdout)
	}
	if copyValue != "pw-email" || copyTimeout != 10 {
		t.Fatalf("copy=%q timeout=%d", copyValue, copyTimeout)
	}
	if len(capturedPaths) == 0 || capturedPaths[0] != "db-passwords/work" {
		t.Fatalf("paths=%v", capturedPaths)
	}
}
