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

type PickOption func(*pickConfig)

type pickConfig struct {
	preview   string
	delimiter string
	withNth   string
}

func WithPreview(expr string) PickOption {
	return func(c *pickConfig) { c.preview = expr }
}
func WithDelimiter(d string) PickOption {
	return func(c *pickConfig) { c.delimiter = d }
}
func WithWithNth(n string) PickOption {
	return func(c *pickConfig) { c.withNth = n }
}

// Hook is the test injection seam.
var Hook func(paths []string, query string) (string, error)

func Pick(lines []string, query string, opts ...PickOption) (string, error) {
	if Hook != nil {
		return Hook(lines, query)
	}
	if len(lines) == 0 {
		return "", errors.New("no entries found.")
	}

	cfg := &pickConfig{}
	for _, o := range opts {
		o(cfg)
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
	if cfg.preview != "" {
		args = append(args, "--preview", cfg.preview)
	}
	if cfg.delimiter != "" {
		args = append(args, "--delimiter", cfg.delimiter)
	}
	if cfg.withNth != "" {
		args = append(args, "--with-nth", cfg.withNth)
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
		return "", errors.New("no entry selected.")
	}
	return selected, nil
}
