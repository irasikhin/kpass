package cli

import (
	"encoding/json"
	"fmt"

	"github.com/irasikhin/kpass/internal/color"
	"github.com/irasikhin/kpass/internal/config"
)

// CopyCmd copies an entry field to the system clipboard.
type CopyCmd struct {
	Entry   string `arg:"" help:"Entry path or partial path."`
	Timeout *int   `arg:"" optional:"" help:"Seconds to keep value on the clipboard (0 = forever, default from config)."`
	Field   string `short:"F" default:"password" help:"Field to copy (path|title|username|password|url|notes|otp or custom field)."`
	JSON    bool   `help:"Output as JSON."`
}

func (cmd *CopyCmd) Run(c *ctx) error {
	timeout := config.DefaultClipboardTimeout
	if cmd.Timeout != nil {
		timeout = *cmd.Timeout
	}

	if err := c.openDatabase(); err != nil {
		return err
	}
	entry, err := c.db.ResolveEntry(cmd.Entry)
	if err != nil {
		return err
	}
	value, err := entry.GetAttribute(cmd.Field)
	if err != nil {
		return &UserError{Msg: err.Error()}
	}
	if isOtpField(cmd.Field) {
		code, err := otpCode(value)
		if err != nil {
			return &UserError{Msg: err.Error()}
		}
		value = code
	}

	if err := clipboardWrite(value, timeout); err != nil {
		return &UserError{Msg: err.Error()}
	}

	if cmd.JSON {
		data, _ := json.Marshal(map[string]any{
			"status":  "ok",
			"path":    entry.DisplayPath(),
			"field":   cmd.Field,
			"timeout": timeout,
		})
		fmt.Fprintln(c.out, string(data))
		return nil
	}

	fmt.Fprintf(c.out, "%s %s %s %s %s\n",
		color.Green("Copied"), color.Bold(cmd.Field),
		color.Faint("for"), color.Bold(entry.DisplayPath()),
		color.Faint("to clipboard"))
	return nil
}
