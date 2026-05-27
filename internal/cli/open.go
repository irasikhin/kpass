package cli

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/irasikhin/kpass/internal/color"
)

// OpenCmd opens an entry's URL in the system browser.
type OpenCmd struct {
	Entry string `arg:"" help:"Entry path or partial path."`
}

func (cmd *OpenCmd) Run(c *ctx) error {
	if err := c.openDatabase(); err != nil {
		return err
	}
	entry, err := c.db.ResolveEntry(cmd.Entry)
	if err != nil {
		return err
	}

	url := entry.Raw().GetContent("URL")
	if url == "" {
		return &UserError{Msg: fmt.Sprintf("Entry '%s' has no URL.", entry.DisplayPath())}
	}

	argv := append(openCommand(), url)
	fmt.Fprintf(c.out, "%s %s\n", color.Faint("Opening"), color.Bold(url))
	if err := exec.Command(argv[0], argv[1:]...).Start(); err != nil {
		return &UserError{Msg: fmt.Sprintf("Failed to open URL: %v", err)}
	}
	return nil
}

// openCommand returns the platform-appropriate command (and argv prefix)
// for opening URLs. The returned slice is the full argv up to but not
// including the URL.
func openCommand() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{"open"}
	case "windows":
		// `cmd /c start "" <url>` — the empty title argument is required
		// because start interprets the first quoted arg as the window title.
		return []string{"cmd", "/c", "start", ""}
	default:
		// Linux, BSD, etc. — prefer xdg-open.
		if path, err := exec.LookPath("xdg-open"); err == nil {
			return []string{path}
		}
		return []string{"xdg-open"} // fallback; exec will fail with a clear error
	}
}
