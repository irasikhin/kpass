package picker

import (
	"errors"
	"io"
	"os/exec"
	"strings"
	"testing"
)

func resetPicker(t *testing.T) {
	t.Helper()
	origHook := Hook
	origLook := lookPathFn
	origRun := runCmdFn
	t.Cleanup(func() {
		Hook = origHook
		lookPathFn = origLook
		runCmdFn = origRun
	})
}

func TestPick_HookForwarded(t *testing.T) {
	resetPicker(t)
	var (
		gotPaths []string
		gotQuery string
	)
	Hook = func(paths []string, query string) (string, error) {
		gotPaths = paths
		gotQuery = query
		return "selected", nil
	}
	result, err := Pick([]string{"a", "b"}, "needle", PickOpts{Preview: "ignored"})
	if err != nil {
		t.Fatal(err)
	}
	if result != "selected" {
		t.Errorf("result = %q", result)
	}
	if len(gotPaths) != 2 || gotPaths[0] != "a" || gotPaths[1] != "b" {
		t.Errorf("paths = %v", gotPaths)
	}
	if gotQuery != "needle" {
		t.Errorf("query = %q", gotQuery)
	}
}

func TestPick_NoEntries(t *testing.T) {
	resetPicker(t)
	Hook = nil
	if _, err := Pick(nil, "", PickOpts{}); err == nil || !strings.Contains(err.Error(), "no entries found") {
		t.Errorf("expected no-entries error, got %v", err)
	}
}

func TestPick_FzfNotFound(t *testing.T) {
	resetPicker(t)
	Hook = nil
	lookPathFn = func(string) (string, error) { return "", exec.ErrNotFound }
	if _, err := Pick([]string{"a"}, "", PickOpts{}); err == nil || !strings.Contains(err.Error(), "fzf not found") {
		t.Errorf("expected fzf-not-found error, got %v", err)
	}
}

func TestPick_BuildsArgvAndStdin(t *testing.T) {
	resetPicker(t)
	Hook = nil
	lookPathFn = func(string) (string, error) { return "/usr/bin/fzf", nil }

	var got *exec.Cmd
	var gotStdin string
	runCmdFn = func(cmd *exec.Cmd) error {
		got = cmd
		if cmd.Stdin != nil {
			data, _ := io.ReadAll(cmd.Stdin)
			gotStdin = string(data)
		}
		// Write something to stdout so we get a non-empty selection.
		if cmd.Stdout != nil {
			_, _ = cmd.Stdout.Write([]byte("a\n"))
		}
		return nil
	}

	val, err := Pick([]string{"a", "b", "c"}, "needle", PickOpts{
		Preview:   "cat {}",
		Delimiter: ":",
		WithNth:   "2..",
	})
	if err != nil {
		t.Fatal(err)
	}
	if val != "a" {
		t.Errorf("val = %q, want a", val)
	}
	want := []string{
		"/usr/bin/fzf",
		"--select-1", "--exit-0",
		"-q", "needle",
		"--preview", "cat {}",
		"--delimiter", ":",
		"--with-nth", "2..",
	}
	if len(got.Args) != len(want) {
		t.Fatalf("argv = %v, want %v", got.Args, want)
	}
	for i := range want {
		if got.Args[i] != want[i] {
			t.Errorf("argv[%d] = %q, want %q", i, got.Args[i], want[i])
		}
	}
	if gotStdin != "a\nb\nc\n" {
		t.Errorf("stdin = %q, want %q", gotStdin, "a\nb\nc\n")
	}
}

func TestPick_NoOptionalFlagsWhenEmpty(t *testing.T) {
	resetPicker(t)
	Hook = nil
	lookPathFn = func(string) (string, error) { return "/usr/bin/fzf", nil }
	var got *exec.Cmd
	runCmdFn = func(cmd *exec.Cmd) error {
		got = cmd
		_, _ = cmd.Stdout.Write([]byte("x\n"))
		return nil
	}
	if _, err := Pick([]string{"x"}, "", PickOpts{}); err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"-q", "--preview", "--delimiter", "--with-nth"} {
		for _, arg := range got.Args {
			if arg == flag {
				t.Errorf("argv should omit %q when option empty, got %v", flag, got.Args)
			}
		}
	}
}

func TestPick_NonExitErrorBubbles(t *testing.T) {
	resetPicker(t)
	Hook = nil
	lookPathFn = func(string) (string, error) { return "/usr/bin/fzf", nil }
	runCmdFn = func(*exec.Cmd) error { return errors.New("io fail") }
	if _, err := Pick([]string{"a"}, "", PickOpts{}); err == nil || !strings.Contains(err.Error(), "io fail") {
		t.Errorf("expected io fail, got %v", err)
	}
}

func TestPick_ExitOne_NoSelection(t *testing.T) {
	// Use a real /bin/false to produce *exec.ExitError with code 1.
	if _, err := exec.LookPath("/bin/false"); err != nil {
		t.Skip("/bin/false unavailable")
	}
	resetPicker(t)
	Hook = nil
	lookPathFn = func(string) (string, error) { return "/bin/false", nil }
	// Use the real runCmd so we get an ExitError. /bin/false ignores stdin/args.
	if _, err := Pick([]string{"a"}, "", PickOpts{}); err == nil || !strings.Contains(err.Error(), "no entry selected") {
		t.Errorf("expected no-selection error, got %v", err)
	}
}

func TestPick_ExitOther_Status(t *testing.T) {
	// /bin/sh -c "exit 130" → ExitError code 130.
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh unavailable")
	}
	resetPicker(t)
	Hook = nil
	lookPathFn = func(string) (string, error) { return sh, nil }

	runCmdFn = func(cmd *exec.Cmd) error {
		// Replace argv with our shell-exits-130 invocation while preserving Stdin/Stdout.
		cmd.Args = []string{sh, "-c", "exit 130"}
		cmd.Path = sh
		return cmd.Run()
	}
	if _, err := Pick([]string{"a"}, "", PickOpts{}); err == nil || !strings.Contains(err.Error(), "status 130") {
		t.Errorf("expected status-130 error, got %v", err)
	}
}

func TestPick_EmptySelection(t *testing.T) {
	resetPicker(t)
	Hook = nil
	lookPathFn = func(string) (string, error) { return "/usr/bin/fzf", nil }
	runCmdFn = func(*exec.Cmd) error { return nil } // success but no stdout
	if _, err := Pick([]string{"a"}, "", PickOpts{}); err == nil || !strings.Contains(err.Error(), "no entry selected") {
		t.Errorf("expected no-selection error, got %v", err)
	}
}
