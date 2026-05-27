// Package clip copies text to the system clipboard using standard
// command-line tools. On X11 it prefers xclip, on Wayland wl-clipboard,
// on macOS pbcopy/pbpaste.
package clip

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Backend reports which clipboard tool is available, or an error with
// install instructions.
func Backend() (string, error) {
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		if look("wl-copy") && look("wl-paste") {
			return "wl", nil
		}
		return "", errors.New(
			"wayland detected; install wl-clipboard: https://github.com/bugaevc/wl-clipboard")
	}
	if look("xclip") {
		return "xclip", nil
	}
	if look("xsel") {
		return "xsel", nil
	}
	if look("pbcopy") && look("pbpaste") {
		return "mac", nil
	}
	return "", errors.New(
		"no clipboard tool found. Install xclip, xsel, or wl-clipboard")
}

func look(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func Write(value string) error {
	b, err := Backend()
	if err != nil {
		return err
	}
	switch b {
	case "xclip":
		cmd := exec.Command("xclip", "-selection", "clipboard")
		cmd.Stdin = strings.NewReader(value)
		return cmd.Run()
	case "xsel":
		cmd := exec.Command("xsel", "--clipboard", "--input")
		cmd.Stdin = strings.NewReader(value)
		return cmd.Run()
	case "wl":
		cmd := exec.Command("wl-copy", "--sensitive")
		cmd.Stdin = strings.NewReader(value)
		return cmd.Run()
	case "mac":
		// pbcopy reads from stdin verbatim — no escaping pitfalls like the
		// AppleScript literal had (newlines, quotes, control chars).
		cmd := exec.Command("pbcopy")
		cmd.Stdin = strings.NewReader(value)
		return cmd.Run()
	}
	return fmt.Errorf("unsupported backend: %s", b)
}

func Read() (string, error) {
	b, err := Backend()
	if err != nil {
		return "", err
	}
	switch b {
	case "xclip":
		out, err := exec.Command("xclip", "-o", "-selection", "clipboard").Output()
		return string(out), err
	case "xsel":
		out, err := exec.Command("xsel", "--clipboard", "--output").Output()
		return string(out), err
	case "wl":
		out, err := exec.Command("wl-paste", "--no-newline").Output()
		return string(out), err
	case "mac":
		out, err := exec.Command("pbpaste").Output()
		return string(out), err
	}
	return "", fmt.Errorf("unsupported backend: %s", b)
}

func Clear() error {
	b, err := Backend()
	if err != nil {
		return err
	}
	switch b {
	case "xclip":
		cmd := exec.Command("xclip", "-selection", "clipboard")
		cmd.Stdin = strings.NewReader("")
		return cmd.Run()
	case "xsel":
		cmd := exec.Command("xsel", "--clipboard", "--clear")
		return cmd.Run()
	case "wl":
		return exec.Command("wl-copy", "--clear").Run()
	case "mac":
		cmd := exec.Command("pbcopy")
		cmd.Stdin = strings.NewReader("")
		return cmd.Run()
	}
	return fmt.Errorf("unsupported backend: %s", b)
}

// WriteWithAutoClear copies value to the clipboard and, if timeout > 0,
// spawns a detached child process that sleeps `timeout` seconds and then
// clears the clipboard if it still holds value. The child is the kpass
// binary itself invoked as `kpass __clear-clipboard <timeout>` with the
// secret piped on stdin; the parent does not wait, so the kpass copy
// command returns immediately.
//
// Detaching matters: a goroutine in the parent would die when the parent
// exits (which it does almost immediately after kpass copy). The argv
// carries only the timeout — never the secret — so the secret is not
// visible in /proc/PID/cmdline.
func WriteWithAutoClear(value string, timeout int) error {
	if err := Write(value); err != nil {
		return err
	}
	if timeout <= 0 {
		return nil
	}
	self, err := os.Executable()
	if err != nil {
		return nil //nolint:nilerr // best-effort: clipboard write succeeded
	}
	cmd := exec.Command(self, "__clear-clipboard", fmt.Sprintf("%d", timeout))
	cmd.Stdin = strings.NewReader(value)
	// Detach: no inherited stdout/stderr; parent will not wait.
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return nil //nolint:nilerr // best-effort
	}
	_ = cmd.Process.Release()
	return nil
}
