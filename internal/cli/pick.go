package cli

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/irasikhin/kpass/internal/color"
	"github.com/irasikhin/kpass/internal/config"
	"github.com/irasikhin/kpass/internal/db"
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
	if err := c.openDatabase(); err != nil {
		return err
	}
	entry, selected, err := cmd.pickEntry(c)
	if err != nil {
		return err
	}
	switch cmd.Action {
	case "get":
		return cmd.actionGet(c, entry)
	case "copy":
		return cmd.actionCopy(c, entry, selected)
	case "edit":
		return (&EditCmd{Entry: selected}).Run(c)
	case "show":
		fmt.Fprintln(c.out, entry.FormatFullWithPassword(false))
	case "open":
		return cmd.actionOpen(c, entry, selected)
	case "delete":
		return cmd.actionDelete(c, entry)
	case "otp":
		return cmd.actionOtp(c, entry, selected)
	}
	return nil
}

// pickEntry collects the candidate lines, runs the picker, and resolves the
// selection back to a *db.Entry. selected is the chosen path (without any
// preview metadata).
func (cmd *PickCmd) pickEntry(c *ctx) (*db.Entry, string, error) {
	var lines []string
	for _, e := range c.db.SortedEntries() {
		if !matchTagFilter(e, cmd.Tag, cmd.TagAny) {
			continue
		}
		path := e.DisplayPath()
		if cmd.Preview {
			lines = append(lines, path+"\t"+e.Raw().GetContent("UserName")+"\t"+e.Raw().GetContent("URL"))
		} else {
			lines = append(lines, path)
		}
	}
	if len(lines) == 0 {
		return nil, "", &UserError{Msg: "No entries match the tag filter."}
	}

	var opts picker.PickOpts
	if cmd.Preview {
		opts = picker.PickOpts{
			Preview:   `printf 'Path:     {1}\nUsername:  {2}\nURL:       {3}'`,
			Delimiter: "\t",
			WithNth:   "1",
		}
	}

	selected, err := picker.Pick(lines, cmd.Query, opts)
	if err != nil {
		return nil, "", &UserError{Msg: err.Error()}
	}
	if i := strings.IndexByte(selected, '\t'); i >= 0 {
		selected = selected[:i]
	}
	entry, err := c.db.ResolveEntry(selected)
	if err != nil {
		return nil, "", err
	}
	return entry, selected, nil
}

func (cmd *PickCmd) actionGet(c *ctx, entry *db.Entry) error {
	if cmd.Field == "" {
		fmt.Fprintln(c.out, entry.FormatFull())
		return nil
	}
	v, err := entry.GetAttribute(cmd.Field)
	if err != nil {
		return &UserError{Msg: err.Error()}
	}
	v, err = resolveFieldValue(cmd.Field, v)
	if err != nil {
		return &UserError{Msg: err.Error()}
	}
	fmt.Fprintln(c.out, v)
	return nil
}

func (cmd *PickCmd) actionCopy(c *ctx, entry *db.Entry, selected string) error {
	f := cmd.Field
	if f == "" {
		f = "password"
	}
	v, err := entry.GetAttribute(f)
	if err != nil {
		return &UserError{Msg: err.Error()}
	}
	v, err = resolveFieldValue(f, v)
	if err != nil {
		return &UserError{Msg: err.Error()}
	}
	if err := clipboardWrite(v, cmd.copyTimeout()); err != nil {
		return &UserError{Msg: err.Error()}
	}
	fmt.Fprintf(c.out, "%s %s %s %s %s\n",
		color.Green("Copied"), color.Bold(f),
		color.Faint("for"), color.Bold(selected),
		color.Faint("to clipboard"))
	return nil
}

func (cmd *PickCmd) actionOpen(c *ctx, entry *db.Entry, selected string) error {
	url := entry.Raw().GetContent("URL")
	if url == "" {
		return &UserError{Msg: fmt.Sprintf("Entry '%s' has no URL.", selected)}
	}
	argv := append(openCommand(), url)
	fmt.Fprintf(c.out, "%s %s\n", color.Faint("Opening"), color.Bold(url))
	if err := exec.Command(argv[0], argv[1:]...).Start(); err != nil {
		return &UserError{Msg: fmt.Sprintf("Failed to open URL: %v", err)}
	}
	return nil
}

func (cmd *PickCmd) actionDelete(c *ctx, entry *db.Entry) error {
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
	return nil
}

func (cmd *PickCmd) actionOtp(c *ctx, entry *db.Entry, selected string) error {
	uri := entry.OtpURI()
	if uri == "" {
		return &UserError{Msg: fmt.Sprintf("Entry '%s' has no TOTP.", selected)}
	}
	code, err := otpCode(uri)
	if err != nil {
		return &UserError{Msg: err.Error()}
	}
	if err := clipboardWrite(code, cmd.copyTimeout()); err != nil {
		return &UserError{Msg: err.Error()}
	}
	fmt.Fprintf(c.out, "%s %s %s %s\n",
		color.Green("Copied TOTP"),
		color.Faint("for"), color.Bold(selected),
		color.Faint("to clipboard"))
	return nil
}

// copyTimeout resolves the --timeout flag against the default clipboard
// timeout for copy/otp actions. A negative value (default) means "use config".
func (cmd *PickCmd) copyTimeout() int {
	if cmd.Timeout < 0 {
		return config.DefaultClipboardTimeout
	}
	return cmd.Timeout
}
