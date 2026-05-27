package cli

import (
	"encoding/json"
	"fmt"

	"github.com/irasikhin/kpass/internal/db"
	"github.com/irasikhin/kpass/internal/editor"
)

// EditCmd updates an entry, either through $EDITOR (no field flags) or by
// applying direct field updates from flags.
type EditCmd struct {
	Entry    string   `arg:"" help:"Entry path or partial path."`
	Editor   string   `help:"Editor command to use instead of $EDITOR." placeholder:"CMD"`
	Rename   *string  `help:"Rename the entry (new title)."`
	Username *string  `short:"u" help:"Set username."`
	URL      *string  `help:"Set URL."`
	Notes    *string  `help:"Set notes."`
	OTP      *string  `help:"Set OTP/TOTP URI."`
	Tags     []string `help:"Set tags (comma or semicolon separated). Repeatable."`
	Field    []string `short:"F" help:"Set a custom field (key=value). Repeatable."`
	Clear    []string `help:"Clear a field (username|password|url|notes|otp or custom field). Repeatable." enum:"username,password,url,notes,otp"`
	JSON     bool     `help:"Output as JSON."`

	Password passwordFlags `embed:""`
}

func (cmd *EditCmd) Run(c *ctx) error {
	directChanges := cmd.hasDirectChanges()
	if cmd.Editor != "" && directChanges {
		return &UserError{Msg: "--editor cannot be combined with direct field updates."}
	}

	if err := c.openDatabase(); err != nil {
		return err
	}
	entry, err := c.db.ResolveEntry(cmd.Entry)
	if err != nil {
		return err
	}

	var changed bool
	if directChanges {
		changed, err = cmd.editViaFlags(c, entry)
	} else {
		changed, err = cmd.editViaEditor(entry)
	}
	if err != nil {
		return err
	}

	if !changed {
		return &UserError{Msg: "No changes requested."}
	}
	if err := c.db.Save(); err != nil {
		return &UserError{Msg: err.Error()}
	}
	return cmd.report(c, entry)
}

func (cmd *EditCmd) hasDirectChanges() bool {
	return cmd.Rename != nil || cmd.Username != nil || cmd.URL != nil ||
		cmd.Notes != nil || cmd.OTP != nil || len(cmd.Tags) > 0 ||
		len(cmd.Clear) > 0 || len(cmd.Field) > 0 || cmd.Password.touched()
}

// editViaEditor opens $EDITOR with the entry serialised, parses the result,
// and applies any field that the user changed.
func (cmd *EditCmd) editViaEditor(entry *db.Entry) (bool, error) {
	raw := entry.Raw()
	initial := editor.Serialize(
		entry.DisplayPath(),
		raw.GetTitle(),
		raw.GetContent("UserName"),
		raw.GetPassword(),
		raw.GetContent("URL"),
		raw.GetContent("otp"),
		raw.GetContent("Notes"),
	)
	body, err := editor.Edit(initial, cmd.Editor)
	if err != nil {
		return false, &UserError{Msg: err.Error()}
	}
	fields, err := editor.Parse(body)
	if err != nil {
		return false, &UserError{Msg: err.Error()}
	}
	original := map[string]string{
		"title":    raw.GetTitle(),
		"username": raw.GetContent("UserName"),
		"password": raw.GetPassword(),
		"url":      raw.GetContent("URL"),
		"otp":      raw.GetContent("otp"),
		"notes":    raw.GetContent("Notes"),
	}
	changed := false
	applyIfChanged := func(name, value string) {
		if original[name] != value {
			entry.SetField(name, value)
			changed = true
		}
	}
	applyIfChanged("title", fields.Title)
	applyIfChanged("username", fields.Username)
	applyIfChanged("password", fields.Password)
	applyIfChanged("url", fields.URL)
	applyIfChanged("otp", fields.OTP)
	applyIfChanged("notes", fields.Notes)
	return changed, nil
}

// editViaFlags applies the per-flag updates declared on cmd.
func (cmd *EditCmd) editViaFlags(c *ctx, entry *db.Entry) (bool, error) {
	changed := false
	if cmd.Rename != nil {
		entry.SetTitle(*cmd.Rename)
		changed = true
	}
	if cmd.Username != nil {
		entry.SetField("username", *cmd.Username)
		changed = true
	}
	if cmd.URL != nil {
		entry.SetField("url", *cmd.URL)
		changed = true
	}
	if cmd.Notes != nil {
		entry.SetField("notes", *cmd.Notes)
		changed = true
	}
	if cmd.OTP != nil {
		entry.SetField("otp", *cmd.OTP)
		changed = true
	}
	for _, f := range cmd.Clear {
		entry.SetField(f, "")
		changed = true
	}
	if len(cmd.Field) > 0 {
		if err := applyCustomFields(entry, cmd.Field); err != nil {
			return false, err
		}
		changed = true
	}
	if len(cmd.Tags) > 0 {
		entry.SetTags(cmd.Tags)
		changed = true
	}
	if cmd.Password.touched() {
		newPw, err := cmd.Password.selectPassword(c, "New password: ", true)
		if err != nil {
			return false, wrapForUser(err)
		}
		entry.SetField("password", newPw)
		changed = true
	}
	return changed, nil
}

func (cmd *EditCmd) report(c *ctx, entry *db.Entry) error {
	if cmd.JSON {
		data, _ := json.Marshal(map[string]any{
			"status": "ok",
			"path":   entry.DisplayPath(),
			"action": "edited",
		})
		fmt.Fprintln(c.out, string(data))
		return nil
	}
	fmt.Fprintln(c.out, entry.DisplayPath())
	return nil
}
