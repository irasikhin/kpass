package clip

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// resetClip restores all package-level seams after each test.
func resetClip(t *testing.T) {
	t.Helper()
	origGetenv := getenvFn
	origLook := lookPathFn
	origRun := runFn
	origOutput := outputFn
	origExec := executableFn
	origStart := startFn
	origBackend := backendFn
	t.Cleanup(func() {
		getenvFn = origGetenv
		lookPathFn = origLook
		runFn = origRun
		outputFn = origOutput
		executableFn = origExec
		startFn = origStart
		backendFn = origBackend
	})
}

// available returns a stub that reports presence only for names in `installed`.
func availableStub(installed ...string) func(string) (string, error) {
	set := map[string]bool{}
	for _, n := range installed {
		set[n] = true
	}
	return func(name string) (string, error) {
		if set[name] {
			return "/usr/bin/" + name, nil
		}
		return "", exec.ErrNotFound
	}
}

func TestBackend_Wayland(t *testing.T) {
	resetClip(t)
	getenvFn = func(k string) string {
		if k == "WAYLAND_DISPLAY" {
			return ":0"
		}
		return ""
	}
	lookPathFn = availableStub("wl-copy", "wl-paste")
	b, err := Backend()
	if err != nil {
		t.Fatal(err)
	}
	if b != "wl" {
		t.Errorf("backend = %q, want wl", b)
	}
}

func TestBackend_WaylandMissingTools(t *testing.T) {
	resetClip(t)
	getenvFn = func(string) string { return ":0" } // wayland set
	lookPathFn = availableStub()
	_, err := Backend()
	if err == nil || !strings.Contains(err.Error(), "wayland detected") {
		t.Errorf("expected wayland-missing error, got %v", err)
	}
}

func TestBackend_Xclip(t *testing.T) {
	resetClip(t)
	getenvFn = func(string) string { return "" }
	lookPathFn = availableStub("xclip")
	b, err := Backend()
	if err != nil {
		t.Fatal(err)
	}
	if b != "xclip" {
		t.Errorf("backend = %q, want xclip", b)
	}
}

func TestBackend_Xsel(t *testing.T) {
	resetClip(t)
	getenvFn = func(string) string { return "" }
	lookPathFn = availableStub("xsel")
	b, err := Backend()
	if err != nil {
		t.Fatal(err)
	}
	if b != "xsel" {
		t.Errorf("backend = %q, want xsel", b)
	}
}

func TestBackend_Mac(t *testing.T) {
	resetClip(t)
	getenvFn = func(string) string { return "" }
	lookPathFn = availableStub("pbcopy", "pbpaste")
	b, err := Backend()
	if err != nil {
		t.Fatal(err)
	}
	if b != "mac" {
		t.Errorf("backend = %q, want mac", b)
	}
}

func TestBackend_NoneAvailable(t *testing.T) {
	resetClip(t)
	getenvFn = func(string) string { return "" }
	lookPathFn = availableStub()
	_, err := Backend()
	if err == nil || !strings.Contains(err.Error(), "no clipboard tool") {
		t.Errorf("expected no-tool error, got %v", err)
	}
}

// readStdin drains cmd.Stdin and returns its contents.
func readStdin(cmd *exec.Cmd) string {
	if cmd.Stdin == nil {
		return ""
	}
	data, _ := io.ReadAll(cmd.Stdin)
	return string(data)
}

func TestWrite_PerBackend(t *testing.T) {
	type tc struct {
		name      string
		installed []string
		wayland   bool
		wantArgv  []string
		wantStdin string
	}
	cases := []tc{
		{"xclip", []string{"xclip"}, false, []string{"xclip", "-selection", "clipboard"}, "secret"},
		{"xsel", []string{"xsel"}, false, []string{"xsel", "--clipboard", "--input"}, "secret"},
		{"wl", []string{"wl-copy", "wl-paste"}, true, []string{"wl-copy", "--sensitive"}, "secret"},
		{"mac", []string{"pbcopy", "pbpaste"}, false, []string{"pbcopy"}, "secret"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resetClip(t)
			getenvFn = func(k string) string {
				if c.wayland && k == "WAYLAND_DISPLAY" {
					return ":0"
				}
				return ""
			}
			lookPathFn = availableStub(c.installed...)
			var got *exec.Cmd
			var gotStdin string
			runFn = func(cmd *exec.Cmd) error {
				got = cmd
				gotStdin = readStdin(cmd)
				return nil
			}
			if err := Write("secret"); err != nil {
				t.Fatal(err)
			}
			if got == nil {
				t.Fatal("runFn not called")
			}
			if len(got.Args) != len(c.wantArgv) {
				t.Errorf("argv = %v, want %v", got.Args, c.wantArgv)
			}
			for i := range c.wantArgv {
				if got.Args[i] != c.wantArgv[i] {
					t.Errorf("argv[%d] = %q, want %q", i, got.Args[i], c.wantArgv[i])
				}
			}
			if gotStdin != c.wantStdin {
				t.Errorf("stdin = %q, want %q", gotStdin, c.wantStdin)
			}
		})
	}
}

func TestWrite_BackendError(t *testing.T) {
	resetClip(t)
	getenvFn = func(string) string { return "" }
	lookPathFn = availableStub()
	if err := Write("x"); err == nil {
		t.Error("expected backend error to bubble")
	}
}

func TestWrite_RunError(t *testing.T) {
	resetClip(t)
	getenvFn = func(string) string { return "" }
	lookPathFn = availableStub("xclip")
	runFn = func(*exec.Cmd) error { return errors.New("boom") }
	if err := Write("x"); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("expected run error, got %v", err)
	}
}

func TestRead_PerBackend(t *testing.T) {
	type tc struct {
		name      string
		installed []string
		wayland   bool
		wantArgv  []string
	}
	cases := []tc{
		{"xclip", []string{"xclip"}, false, []string{"xclip", "-o", "-selection", "clipboard"}},
		{"xsel", []string{"xsel"}, false, []string{"xsel", "--clipboard", "--output"}},
		{"wl", []string{"wl-copy", "wl-paste"}, true, []string{"wl-paste", "--no-newline"}},
		{"mac", []string{"pbcopy", "pbpaste"}, false, []string{"pbpaste"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resetClip(t)
			getenvFn = func(k string) string {
				if c.wayland && k == "WAYLAND_DISPLAY" {
					return ":0"
				}
				return ""
			}
			lookPathFn = availableStub(c.installed...)
			var got *exec.Cmd
			outputFn = func(cmd *exec.Cmd) ([]byte, error) {
				got = cmd
				return []byte("clipped"), nil
			}
			val, err := Read()
			if err != nil {
				t.Fatal(err)
			}
			if val != "clipped" {
				t.Errorf("value = %q", val)
			}
			if len(got.Args) != len(c.wantArgv) {
				t.Errorf("argv = %v, want %v", got.Args, c.wantArgv)
			}
		})
	}
}

func TestRead_BackendError(t *testing.T) {
	resetClip(t)
	getenvFn = func(string) string { return "" }
	lookPathFn = availableStub()
	if _, err := Read(); err == nil {
		t.Error("expected backend error to bubble")
	}
}

func TestClear_PerBackend(t *testing.T) {
	type tc struct {
		name      string
		installed []string
		wayland   bool
		wantArgv  []string
	}
	cases := []tc{
		{"xclip", []string{"xclip"}, false, []string{"xclip", "-selection", "clipboard"}},
		{"xsel", []string{"xsel"}, false, []string{"xsel", "--clipboard", "--clear"}},
		{"wl", []string{"wl-copy", "wl-paste"}, true, []string{"wl-copy", "--clear"}},
		{"mac", []string{"pbcopy", "pbpaste"}, false, []string{"pbcopy"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resetClip(t)
			getenvFn = func(k string) string {
				if c.wayland && k == "WAYLAND_DISPLAY" {
					return ":0"
				}
				return ""
			}
			lookPathFn = availableStub(c.installed...)
			var got *exec.Cmd
			runFn = func(cmd *exec.Cmd) error {
				got = cmd
				return nil
			}
			if err := Clear(); err != nil {
				t.Fatal(err)
			}
			if len(got.Args) != len(c.wantArgv) {
				t.Errorf("argv = %v, want %v", got.Args, c.wantArgv)
			}
		})
	}
}

func TestClear_BackendError(t *testing.T) {
	resetClip(t)
	getenvFn = func(string) string { return "" }
	lookPathFn = availableStub()
	if err := Clear(); err == nil {
		t.Error("expected backend error to bubble")
	}
}

func TestWriteWithAutoClear_TimeoutZero(t *testing.T) {
	resetClip(t)
	getenvFn = func(string) string { return "" }
	lookPathFn = availableStub("xclip")
	runFn = func(*exec.Cmd) error { return nil }
	called := false
	startFn = func(*exec.Cmd) error { called = true; return nil }

	if err := WriteWithAutoClear("x", 0); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Error("timeout=0 must not spawn the clear child")
	}
}

func TestWriteWithAutoClear_DetachedChild(t *testing.T) {
	resetClip(t)
	getenvFn = func(string) string { return "" }
	lookPathFn = availableStub("xclip")
	runFn = func(*exec.Cmd) error { return nil }
	executableFn = func() (string, error) { return "/usr/local/bin/kpass", nil }

	var captured *exec.Cmd
	startFn = func(cmd *exec.Cmd) error { captured = cmd; return nil }

	if err := WriteWithAutoClear("secret-value", 30); err != nil {
		t.Fatal(err)
	}
	if captured == nil {
		t.Fatal("startFn not called")
	}
	if captured.Path != "/usr/local/bin/kpass" {
		t.Errorf("child path = %q, want /usr/local/bin/kpass", captured.Path)
	}
	want := []string{"/usr/local/bin/kpass", "__clear-clipboard", "30"}
	if len(captured.Args) != len(want) {
		t.Errorf("args = %v, want %v", captured.Args, want)
	}
	for i := range want {
		if captured.Args[i] != want[i] {
			t.Errorf("args[%d] = %q, want %q", i, captured.Args[i], want[i])
		}
	}
	if got := readStdin(captured); got != "secret-value" {
		t.Errorf("stdin = %q, want secret-value", got)
	}
	if captured.Stdout != nil || captured.Stderr != nil {
		t.Error("stdout/stderr should be nil (detached)")
	}
}

func TestWriteWithAutoClear_WriteError(t *testing.T) {
	resetClip(t)
	getenvFn = func(string) string { return "" }
	lookPathFn = availableStub("xclip")
	runFn = func(*exec.Cmd) error { return errors.New("write fail") }
	startFn = func(*exec.Cmd) error {
		t.Error("startFn must not run when Write fails")
		return nil
	}

	if err := WriteWithAutoClear("x", 30); err == nil {
		t.Error("expected Write error to bubble")
	}
}

func TestWriteWithAutoClear_ExecutableError(t *testing.T) {
	resetClip(t)
	getenvFn = func(string) string { return "" }
	lookPathFn = availableStub("xclip")
	runFn = func(*exec.Cmd) error { return nil }
	executableFn = func() (string, error) { return "", errors.New("self lookup failed") }
	called := false
	startFn = func(*exec.Cmd) error { called = true; return nil }

	if err := WriteWithAutoClear("x", 30); err != nil {
		t.Errorf("should swallow executable error (best-effort), got %v", err)
	}
	if called {
		t.Error("must not Start when executableFn fails")
	}
}

func TestWrite_UnsupportedBackend(t *testing.T) {
	resetClip(t)
	backendFn = func() (string, error) { return "weird", nil }
	if err := Write("x"); err == nil || !strings.Contains(err.Error(), "unsupported backend") {
		t.Errorf("Write unsupported = %v", err)
	}
}

func TestRead_UnsupportedBackend(t *testing.T) {
	resetClip(t)
	backendFn = func() (string, error) { return "weird", nil }
	if _, err := Read(); err == nil || !strings.Contains(err.Error(), "unsupported backend") {
		t.Errorf("Read unsupported = %v", err)
	}
}

func TestClear_UnsupportedBackend(t *testing.T) {
	resetClip(t)
	backendFn = func() (string, error) { return "weird", nil }
	if err := Clear(); err == nil || !strings.Contains(err.Error(), "unsupported backend") {
		t.Errorf("Clear unsupported = %v", err)
	}
}

// Exercise the default runFn lambda body via a deterministic failure path.
func TestDefaultRunFn_BodyExecutes(t *testing.T) {
	resetClip(t)
	backendFn = func() (string, error) { return "xclip", nil }
	// runFn NOT overridden — default c.Run() runs and fails because the binary is absent.
	if err := Write("x"); err == nil {
		t.Error("expected exec error from non-existent xclip")
	}
}

func TestDefaultOutputFn_BodyExecutes(t *testing.T) {
	resetClip(t)
	backendFn = func() (string, error) { return "xclip", nil }
	if _, err := Read(); err == nil {
		t.Error("expected exec error from non-existent xclip via default outputFn")
	}
}

func TestDefaultStartFn_BodyExecutes(t *testing.T) {
	resetClip(t)
	backendFn = func() (string, error) { return "xclip", nil }
	runFn = func(*exec.Cmd) error { return nil } // make Write succeed
	executableFn = func() (string, error) { return "/no/such/binary/please", nil }
	// startFn NOT overridden — default c.Start() runs and fails.
	if err := WriteWithAutoClear("x", 5); err != nil {
		t.Errorf("WriteWithAutoClear should swallow start error, got %v", err)
	}
}

func TestWriteWithAutoClear_ProcessRelease(t *testing.T) {
	resetClip(t)
	backendFn = func() (string, error) { return "xclip", nil }
	runFn = func(*exec.Cmd) error { return nil }
	executableFn = func() (string, error) { return "/whatever", nil }
	released := false
	startFn = func(c *exec.Cmd) error {
		// Attach the current process so cmd.Process.Release() is a safe no-op.
		proc, err := os.FindProcess(os.Getpid())
		if err != nil {
			return err
		}
		c.Process = proc
		released = true
		return nil
	}
	if err := WriteWithAutoClear("x", 5); err != nil {
		t.Fatal(err)
	}
	if !released {
		t.Error("startFn should have been invoked")
	}
}

func TestWriteWithAutoClear_StartError(t *testing.T) {
	resetClip(t)
	getenvFn = func(string) string { return "" }
	lookPathFn = availableStub("xclip")
	runFn = func(*exec.Cmd) error { return nil }
	executableFn = func() (string, error) { return "/bin/kpass", nil }
	startFn = func(*exec.Cmd) error { return errors.New("start fail") }

	if err := WriteWithAutoClear("x", 30); err != nil {
		t.Errorf("should swallow Start error (best-effort), got %v", err)
	}
}
