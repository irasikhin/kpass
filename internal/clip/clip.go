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
	"time"
)

// Backend reports which clipboard tool is available, or an error with
// install instructions.
func Backend() (string, error) {
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		if look("wl-copy") && look("wl-paste") {
			return "wl", nil
		}
		return "", errors.New(
			"Wayland detected. Install wl-clipboard: https://github.com/bugaevc/wl-clipboard")
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
		esc := strings.ReplaceAll(value, `\`, `\\`)
		esc = strings.ReplaceAll(esc, `"`, `\"`)
		return exec.Command("osascript", "-e", fmt.Sprintf(`set the clipboard to "%s"`, esc)).Run()
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
		out, err := exec.Command("osascript", "-e", "the clipboard").Output()
		return strings.TrimSpace(string(out)), err
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
		return exec.Command("osascript", "-e", `set the clipboard to ""`).Run()
	}
	return fmt.Errorf("unsupported backend: %s", b)
}

func WriteWithAutoClear(value string, timeout int) error {
	if err := Write(value); err != nil {
		return err
	}
	if timeout <= 0 {
		return nil
	}
	go func() {
		time.Sleep(time.Duration(timeout) * time.Second)
		current, _ := Read()
		if current == value {
			_ = Clear()
		}
	}()
	return nil
}


