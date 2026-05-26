package cli

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/irasikhin/kpass/internal/color"
)

// OpenCmd opens an entry's URL (or a custom field) in the system browser.
type OpenCmd struct {
	Entry string `arg:"" help:"Entry path or partial path."`
	Field string `short:"F" default:"url" help:"Field to open (must contain a URL)." enum:"url,otp"`
}

func (cmd *OpenCmd) Run(c *ctx) error {
	if err := c.openDatabase(); err != nil {
		return err
	}
	entry, err := c.db.ResolveEntry(cmd.Entry)
	if err != nil {
		return err
	}

	var target string
	switch cmd.Field {
	case "otp":
		uri := entry.OtpURI()
		if uri == "" {
			return &UserError{Msg: fmt.Sprintf("Entry '%s' has no TOTP URI.", entry.DisplayPath())}
		}
		target = uri
	default:
		url := entry.Raw().GetContent("URL")
		if url == "" {
			return &UserError{Msg: fmt.Sprintf("Entry '%s' has no URL.", entry.DisplayPath())}
		}
		target = url
	}

	opener := openCommand()
	fmt.Fprintf(c.out, "%s %s\n", color.Faint("Opening"), color.Bold(target))
	if err := exec.Command(opener, target).Start(); err != nil {
		return &UserError{Msg: fmt.Sprintf("Failed to open URL: %v", err)}
	}
	return nil
}

// openCommand returns the platform-appropriate command for opening URLs.
func openCommand() string {
	switch runtime.GOOS {
	case "darwin":
		return "open"
	case "windows":
		return "rundll32"
	default:
		// Linux, BSD, etc. — prefer xdg-open.
		if path, err := exec.LookPath("xdg-open"); err == nil {
			return path
		}
		return "xdg-open" // fallback; exec will fail with a clear error
	}
}
