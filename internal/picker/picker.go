// Package picker provides an interactive entry selector using fzf.
// Falls back to a simple stdin picker when fzf is not installed.
package picker

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// PickOpts tweaks the fzf invocation. Zero-value disables all decorations.
type PickOpts struct {
	Preview   string // --preview expression
	Delimiter string // --delimiter for multi-column input
	WithNth   string // --with-nth visible columns
}

// Hook is the test injection seam.
var Hook func(paths []string, query string) (string, error)

func Pick(lines []string, query string, opts PickOpts) (string, error) {
	if Hook != nil {
		return Hook(lines, query)
	}
	if len(lines) == 0 {
		return "", errors.New("no entries found")
	}

	binary, err := exec.LookPath("fzf")
	if err != nil {
		return "", errors.New(
			"fzf not found. Install: https://github.com/junegunn/fzf")
	}

	args := []string{"--select-1", "--exit-0"}
	if query != "" {
		args = append(args, "-q", query)
	}
	if opts.Preview != "" {
		args = append(args, "--preview", opts.Preview)
	}
	if opts.Delimiter != "" {
		args = append(args, "--delimiter", opts.Delimiter)
	}
	if opts.WithNth != "" {
		args = append(args, "--with-nth", opts.WithNth)
	}

	cmd := exec.Command(binary, args...)
	cmd.Stdin = strings.NewReader(strings.Join(lines, "\n") + "\n")
	cmd.Stderr = os.Stderr
	var out bytes.Buffer
	cmd.Stdout = &out
	err = cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code := exitErr.ExitCode()
			if code != 0 && code != 1 {
				return "", fmt.Errorf("fzf exited with status %d: %s",
					code, strings.TrimSpace(string(exitErr.Stderr)))
			}
		} else {
			return "", err
		}
	}
	selected := strings.TrimSpace(out.String())
	if selected == "" {
		return "", errors.New("no entry selected")
	}
	return selected, nil
}
