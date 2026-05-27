package editor

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// SpawnHook is the injection seam used by tests (mirrors Python's
// patch.object(KPASS.subprocess, "run", side_effect=fake_editor)).
// It receives the full editor argv (last element is the tempfile path) and
// must return a zero-or-positive exit status.
var SpawnHook func(argv []string) (int, error)

// Filesystem seams for tests that exercise tempfile error paths.
var (
	createTempFn = os.CreateTemp
	chmodFn      = os.Chmod
)

// Edit writes `initial` to a tempfile, spawns $VISUAL/$EDITOR (or the explicit
// editor command) on it, and returns the file contents after the editor exits.
func Edit(initial string, explicit string) (string, error) {
	argv, err := EditorArgv(explicit)
	if err != nil {
		return "", err
	}
	tmp, err := createTempFn("", "kpass-edit-*.txt")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if err := chmodFn(tmpPath, 0o600); err != nil {
		_ = tmp.Close()
		return "", err
	}
	if _, err := tmp.WriteString(initial); err != nil {
		_ = tmp.Close()
		return "", err
	}
	// Tempfile is removed below regardless; a Close error here would not change
	// the outcome since we still read the contents via os.ReadFile.
	_ = tmp.Close()

	cmd := append(append([]string{}, argv...), tmpPath)
	status, err := runEditor(cmd)
	if err != nil {
		return "", err
	}
	if status != 0 {
		return "", fmt.Errorf("editor exited with status %d", status)
	}
	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// EditorArgv resolves the editor command from --editor / $VISUAL / $EDITOR.
// shlex-style splitting: spaces split tokens.
func EditorArgv(explicit string) ([]string, error) {
	editor := explicit
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		for _, candidate := range []string{"vi", "nano"} {
			if path, err := exec.LookPath(candidate); err == nil {
				editor = path
				break
			}
		}
	}
	if editor == "" {
		return nil, errors.New("no editor found. Set $VISUAL or $EDITOR")
	}
	argv := strings.Fields(editor)
	if len(argv) == 0 {
		return nil, errors.New("editor command cannot be empty")
	}
	return argv, nil
}

func runEditor(argv []string) (int, error) {
	if SpawnHook != nil {
		return SpawnHook(argv)
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return 0, err
	}
	return 0, nil
}
