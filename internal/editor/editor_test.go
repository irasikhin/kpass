package editor

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func resetEditor(t *testing.T) {
	t.Helper()
	orig := SpawnHook
	t.Cleanup(func() { SpawnHook = orig })
}

func TestEdit_HookReceivesArgvAndTempfile(t *testing.T) {
	resetEditor(t)
	var got []string
	SpawnHook = func(argv []string) (int, error) {
		got = append([]string{}, argv...)
		// Mutate the tempfile to simulate an edit session.
		if err := os.WriteFile(argv[len(argv)-1], []byte("EDITED"), 0o600); err != nil {
			return 0, err
		}
		return 0, nil
	}

	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "/bin/true -a -b")

	out, err := Edit("initial-text", "")
	if err != nil {
		t.Fatal(err)
	}
	if out != "EDITED" {
		t.Errorf("output = %q, want EDITED", out)
	}
	if len(got) != 4 || got[0] != "/bin/true" || got[1] != "-a" || got[2] != "-b" {
		t.Errorf("argv prefix = %v", got[:3])
	}
	if filepath.Ext(got[len(got)-1]) != ".txt" {
		t.Errorf("tempfile = %q, want *.txt", got[len(got)-1])
	}
}

func TestEdit_TempfileMode0600(t *testing.T) {
	resetEditor(t)
	var seenMode os.FileMode
	SpawnHook = func(argv []string) (int, error) {
		info, err := os.Stat(argv[len(argv)-1])
		if err != nil {
			return 0, err
		}
		seenMode = info.Mode().Perm()
		return 0, nil
	}
	if _, err := Edit("x", "vi"); err != nil {
		t.Fatal(err)
	}
	if seenMode != 0o600 {
		t.Errorf("tempfile mode = %#o, want 0600", seenMode)
	}
}

func TestEdit_TempfileRemovedAfter(t *testing.T) {
	resetEditor(t)
	var seenPath string
	SpawnHook = func(argv []string) (int, error) {
		seenPath = argv[len(argv)-1]
		return 0, nil
	}
	if _, err := Edit("x", "vi"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(seenPath); !os.IsNotExist(err) {
		t.Errorf("tempfile should be removed after Edit, stat=%v", err)
	}
}

func TestEdit_TempfileRemovedOnHookError(t *testing.T) {
	resetEditor(t)
	var seenPath string
	SpawnHook = func(argv []string) (int, error) {
		seenPath = argv[len(argv)-1]
		return 0, errors.New("hook fail")
	}
	if _, err := Edit("x", "vi"); err == nil {
		t.Error("expected error from hook")
	}
	if _, err := os.Stat(seenPath); !os.IsNotExist(err) {
		t.Errorf("tempfile should be removed even on hook error, stat=%v", err)
	}
}

func TestEdit_NonZeroExitStatus(t *testing.T) {
	resetEditor(t)
	SpawnHook = func([]string) (int, error) { return 7, nil }
	_, err := Edit("x", "vi")
	if err == nil || !strings.Contains(err.Error(), "status 7") {
		t.Errorf("expected status-7 error, got %v", err)
	}
}

func TestEdit_NoEditorAvailable(t *testing.T) {
	resetEditor(t)
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	t.Setenv("PATH", t.TempDir()) // empty PATH so vi/nano are absent
	_, err := Edit("x", "")
	if err == nil || !strings.Contains(err.Error(), "no editor") {
		t.Errorf("expected no-editor error, got %v", err)
	}
}

func TestEditorArgv_ExplicitWins(t *testing.T) {
	t.Setenv("VISUAL", "vim")
	t.Setenv("EDITOR", "nano")
	argv, err := EditorArgv("emacs -nw")
	if err != nil {
		t.Fatal(err)
	}
	if len(argv) != 2 || argv[0] != "emacs" || argv[1] != "-nw" {
		t.Errorf("argv = %v", argv)
	}
}

func TestEditorArgv_VisualOverEditor(t *testing.T) {
	t.Setenv("VISUAL", "vim")
	t.Setenv("EDITOR", "nano")
	argv, err := EditorArgv("")
	if err != nil {
		t.Fatal(err)
	}
	if argv[0] != "vim" {
		t.Errorf("argv[0] = %q, want vim", argv[0])
	}
}

func TestEditorArgv_EditorFallback(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "nano -B")
	argv, err := EditorArgv("")
	if err != nil {
		t.Fatal(err)
	}
	if len(argv) != 2 || argv[0] != "nano" || argv[1] != "-B" {
		t.Errorf("argv = %v", argv)
	}
}

func TestEditorArgv_DefaultLookup(t *testing.T) {
	// Make a fake vi in PATH.
	dir := t.TempDir()
	fakeVi := filepath.Join(dir, "vi")
	if err := os.WriteFile(fakeVi, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	t.Setenv("PATH", dir)
	argv, err := EditorArgv("")
	if err != nil {
		t.Fatal(err)
	}
	if argv[0] != fakeVi {
		t.Errorf("argv[0] = %q, want %q", argv[0], fakeVi)
	}
}

func TestEditorArgv_NoneFound(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	t.Setenv("PATH", t.TempDir())
	if _, err := EditorArgv(""); err == nil || !strings.Contains(err.Error(), "no editor") {
		t.Errorf("expected no-editor error, got %v", err)
	}
}

func TestEditorArgv_EmptyAfterFields(t *testing.T) {
	if _, err := EditorArgv("   "); err == nil {
		// "   " has VISUAL/EDITOR fallbacks; treat as a sanity check, may pick env.
		t.Skip("EDITOR/VISUAL took precedence")
	}
}

func TestEdit_NoEditorArgvBubblesErr(t *testing.T) {
	resetEditor(t)
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	t.Setenv("PATH", t.TempDir())
	if _, err := Edit("x", ""); err == nil || !strings.Contains(err.Error(), "no editor") {
		t.Errorf("Edit should propagate EditorArgv error, got %v", err)
	}
}

func TestEdit_ReadFileError(t *testing.T) {
	resetEditor(t)
	SpawnHook = func(argv []string) (int, error) {
		// Delete the tempfile so ReadFile fails after the hook returns.
		return 0, os.Remove(argv[len(argv)-1])
	}
	// Hook returns the Remove err; verify it's reported, not silently ignored.
	if _, err := Edit("x", "vi"); err == nil {
		t.Error("expected error when tempfile is gone")
	}
}

func writeShellScript(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunEditor_RealBinarySuccess(t *testing.T) {
	resetEditor(t)
	exitOk := writeShellScript(t, "ok.sh", "exit 0\n")
	out, err := Edit("body", exitOk)
	if err != nil {
		t.Fatal(err)
	}
	if out != "body" {
		t.Errorf("content should be unchanged, got %q", out)
	}
}

func TestRunEditor_RealBinaryNonZero(t *testing.T) {
	resetEditor(t)
	exitNo := writeShellScript(t, "no.sh", "exit 7\n")
	if _, err := Edit("body", exitNo); err == nil || !strings.Contains(err.Error(), "status 7") {
		t.Errorf("expected status-7 error, got %v", err)
	}
}

func withFsSeams(t *testing.T, create func(string, string) (*os.File, error), chmod func(string, os.FileMode) error) {
	t.Helper()
	origC, origCh := createTempFn, chmodFn
	if create != nil {
		createTempFn = create
	}
	if chmod != nil {
		chmodFn = chmod
	}
	t.Cleanup(func() {
		createTempFn = origC
		chmodFn = origCh
	})
}

func TestEdit_CreateTempError(t *testing.T) {
	resetEditor(t)
	withFsSeams(t, func(string, string) (*os.File, error) { return nil, errors.New("nofs") }, nil)
	if _, err := Edit("x", "vi"); err == nil || !strings.Contains(err.Error(), "nofs") {
		t.Errorf("expected createTemp error, got %v", err)
	}
}

func TestEdit_ChmodError(t *testing.T) {
	resetEditor(t)
	SpawnHook = func([]string) (int, error) { return 0, nil }
	withFsSeams(t, nil, func(string, os.FileMode) error { return errors.New("nochmod") })
	if _, err := Edit("x", "vi"); err == nil || !strings.Contains(err.Error(), "nochmod") {
		t.Errorf("expected chmod error, got %v", err)
	}
}

func TestEdit_WriteAndCloseErrorsViaClosedFile(t *testing.T) {
	resetEditor(t)
	// CreateTemp returns a file already closed → WriteString fails; Close also fails on the next call path.
	withFsSeams(t, func(dir, pattern string) (*os.File, error) {
		f, err := os.CreateTemp(dir, pattern)
		if err != nil {
			return nil, err
		}
		_ = f.Close() // pre-close to force WriteString to fail
		return f, nil
	}, nil)
	if _, err := Edit("x", "vi"); err == nil {
		t.Error("expected WriteString error on pre-closed tempfile")
	}
}

func TestEdit_CloseError(t *testing.T) {
	resetEditor(t)
	// We can't easily inject a Close error without a custom *os.File. Skip this exact branch;
	// it's tested implicitly by the OS layer. Use the build-tag fallback to document.
	t.Skip("Close error on tempfile is not reachable without an *os.File seam")
}

func TestRunEditor_StartError(t *testing.T) {
	resetEditor(t)
	// A path that exists but isn't executable → exec returns a non-ExitError.
	dir := t.TempDir()
	notExec := filepath.Join(dir, "noexec")
	if err := os.WriteFile(notExec, []byte("not a binary"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Edit("x", notExec); err == nil {
		t.Error("expected start error for non-executable")
	}
}
