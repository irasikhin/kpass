package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/irasikhin/kpass/internal/cli"
)

// Tests for the cmd/kpass entry point. main() itself calls os.Exit so we can't
// invoke it in-process; instead we verify the same wiring main() relies on —
// cli.Version is set and cli.Run() returns a status code for a recognised flag.
// This gives the cmd/kpass package coverage of its imports/init path.

func TestVersionWiring(t *testing.T) {
	// Mirror main()'s behaviour: set the version, then call cli.Run.
	cli.Version = version
	var out, errBuf bytes.Buffer
	code := cli.Run([]string{"--version"}, strings.NewReader(""), &out, &errBuf)
	if code != 0 {
		t.Fatalf("--version exit code = %d, stderr=%s", code, errBuf.String())
	}
	combined := out.String() + errBuf.String()
	if !strings.Contains(combined, "dev") && !strings.Contains(combined, "kpass") {
		t.Errorf("unexpected version output: %q", combined)
	}
}

func TestVersionIsDevByDefault(t *testing.T) {
	if version != "dev" {
		t.Errorf("default version = %q, want dev (this builds with no ldflags)", version)
	}
}

func TestRun_HelpSucceeds(t *testing.T) {
	cli.Version = version
	var out, errBuf bytes.Buffer
	code := cli.Run([]string{"--help"}, strings.NewReader(""), &out, &errBuf)
	combined := out.String() + errBuf.String()
	if !strings.Contains(combined, "Usage") {
		t.Errorf("--help missing 'Usage': %q (exit=%d)", combined, code)
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	cli.Version = version
	var out, errBuf bytes.Buffer
	code := cli.Run([]string{"totally-not-a-command"}, strings.NewReader(""), &out, &errBuf)
	if code == 0 {
		t.Errorf("expected non-zero exit for unknown command, stdout=%q stderr=%q", out.String(), errBuf.String())
	}
}

// TestMainEntry drives main() in-process via the `exit` seam, covering the
// 3-line body of cmd/kpass/main.go.
func TestMainEntry(t *testing.T) {
	origArgs := os.Args
	origExit := exit
	t.Cleanup(func() {
		os.Args = origArgs
		exit = origExit
	})

	os.Args = []string{"kpass", "--version"}
	var captured int
	exit = func(code int) { captured = code }
	main()
	if captured != 0 {
		t.Errorf("main() --version exit code = %d", captured)
	}
}
