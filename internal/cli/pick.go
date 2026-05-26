package cli

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/irasikhin/kpass/internal/color"
	"github.com/irasikhin/kpass/internal/config"
	"github.com/irasikhin/kpass/internal/picker"
)

// PickCmd lets the user interactively pick an entry (via fzf) and then runs
// `action` against it.
type PickCmd struct {
	Query   string   `arg:"" optional:"" help:"Initial query for the picker."`
	Action  string   `default:"copy" enum:"get,copy,edit,open,show,delete,otp" help:"What to do with the selected entry."`
	Field   string   `short:"F" help:"Field to pass to the action (default password for copy)."`
	Timeout int      `default:"-1" help:"Clipboard timeout when action=copy (seconds)."`
	Preview bool     `help:"Show a preview window in fzf with entry details."`
	Tag     []string `help:"Filter by tag (AND). Repeatable."`
	TagAny  []string `name:"tag-any" help:"Filter by tag (OR). Repeatable."`
}

func (cmd *PickCmd) Run(c *ctx) error {
	timeout := cmd.Timeout
	if timeout < 0 {
		timeout = config.DefaultClipboardTimeout
	}

	if err := c.openDatabase(); err != nil {
		return err
	}

	// Build input lines: "path" or "path\tusername\turl" for preview.
	var lines []string
	for _, e := range c.db.SortedEntries() {
		if !matchTagFilter(e, cmd.Tag, cmd.TagAny) {
			continue
		}
		path := e.DisplayPath()
		if cmd.Preview {
			u := e.Raw().GetContent("UserName")
			url := e.Raw().GetContent("URL")
			line := path + "\t" + u + "\t" + url
			lines = append(lines, line)
		} else {
			lines = append(lines, path)
		}
	}
	if len(lines) == 0 {
		return &UserError{Msg: "No entries match the tag filter."}
	}

	var opts []picker.PickOption
	if cmd.Preview {
		opts = append(opts, picker.WithPreview(`printf 'Path:     {1}\nUsername:  {2}\nURL:       {3}'`))
		opts = append(opts, picker.WithDelimiter("\t"))
		opts = append(opts, picker.WithWithNth("1"))
	}

	selected, err := picker.Pick(lines, cmd.Query, opts...)
	if err != nil {
		return &UserError{Msg: err.Error()}
	}
	// Extract just the path (first tab-delimited field).
	if i := strings.IndexByte(selected, '\t'); i >= 0 {
		selected = selected[:i]
	}

	entry, err := c.db.ResolveEntry(selected)
	if err != nil {
		return err
	}

	switch cmd.Action {
	case "get":
		if cmd.Field != "" {
			v, err := entry.GetAttribute(cmd.Field)
			if err != nil {
				return &UserError{Msg: err.Error()}
			}
			if isOtpField(cmd.Field) {
				code, err := otpCode(v)
				if err != nil {
					return &UserError{Msg: err.Error()}
				}
				v = code
			}
			fmt.Fprintln(c.out, v)
		} else {
			fmt.Fprintln(c.out, entry.FormatFull())
		}
	case "copy":
		f := cmd.Field
		if f == "" {
			f = "password"
		}
		v, err := entry.GetAttribute(f)
		if err != nil {
			return &UserError{Msg: err.Error()}
		}
		if isOtpField(f) {
			code, err := otpCode(v)
			if err != nil {
				return &UserError{Msg: err.Error()}
			}
			v = code
		}
		if err := clipboardWrite(v, timeout); err != nil {
			return &UserError{Msg: err.Error()}
		}
		fmt.Fprintf(c.out, "%s %s %s %s %s\n",
			color.Green("Copied"), color.Bold(f),
			color.Faint("for"), color.Bold(selected),
			color.Faint("to clipboard"))
	case "edit":
		ec := EditCmd{Entry: selected}
		return ec.Run(c)
	case "show":
		fmt.Fprintln(c.out, entry.FormatFullWithPassword(false))
	case "open":
		url := entry.Raw().GetContent("URL")
		if url == "" {
			return &UserError{Msg: fmt.Sprintf("Entry '%s' has no URL.", selected)}
		}
		opener := openCommand()
		fmt.Fprintf(c.out, "%s %s\n", color.Faint("Opening"), color.Bold(url))
		if err := exec.Command(opener, url).Start(); err != nil {
			return &UserError{Msg: fmt.Sprintf("Failed to open URL: %v", err)}
		}
	case "delete":
		details := []string{entry.DisplayPath()}
		raw := entry.Raw()
		if u := raw.GetContent("UserName"); u != "" {
			details = append(details, color.Faint("Username: "+u))
		}
		if u := raw.GetContent("URL"); u != "" {
			details = append(details, color.Faint("URL: "+u))
		}
		ok, err := confirm(c, "Delete", details...)
		if err != nil {
			return &UserError{Msg: err.Error()}
		}
		if !ok {
			return &UserError{Msg: "Aborted."}
		}
		if err := c.db.DeleteEntry(entry); err != nil {
			return &UserError{Msg: err.Error()}
		}
		if err := c.db.Save(); err != nil {
			return &UserError{Msg: err.Error()}
		}
		fmt.Fprintln(c.out, color.Green("Deleted."))
	case "otp":
		uri := entry.OtpURI()
		if uri == "" {
			return &UserError{Msg: fmt.Sprintf("Entry '%s' has no TOTP.", selected)}
		}
		code, err := otpCode(uri)
		if err != nil {
			return &UserError{Msg: err.Error()}
		}
		if err := clipboardWrite(code, timeout); err != nil {
			return &UserError{Msg: err.Error()}
		}
		fmt.Fprintf(c.out, "%s %s %s %s\n",
			color.Green("Copied TOTP"),
			color.Faint("for"), color.Bold(selected),
			color.Faint("to clipboard"))
	}
	return nil
}

func isOtpField(field string) bool {
	return field == "otp" || field == "totp" || field == "code"
}
