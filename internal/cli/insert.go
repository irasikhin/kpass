package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/irasikhin/kpass/internal/db"
	"github.com/irasikhin/kpass/internal/runtimex"
)

// passwordFlags holds the inline / stdin / generate flags shared by insert,
// edit, and generate via the selectPassword helper.
type passwordFlags struct {
	Password      *string `help:"Provide password inline (omit to be prompted)."`
	PasswordStdin bool    `help:"Read password from stdin."`
	Generate      bool    `short:"g" help:"Generate a random password."`
	Length        int     `short:"L" default:"24" help:"Generated password length."`
	Lower         bool    `help:"Allow lowercase in generated password."`
	Upper         bool    `help:"Allow uppercase in generated password."`
	Digits        bool    `help:"Allow digits in generated password."`
	Symbols       bool    `help:"Allow symbols in generated password."`
	NoLower       bool    `help:"Exclude lowercase from generated password (only when no --lower/--upper/--digits/--symbols are set)."`
	NoUpper       bool    `help:"Exclude uppercase from generated password (only when no --lower/--upper/--digits/--symbols are set)."`
	NoDigits      bool    `help:"Exclude digits from generated password (only when no --lower/--upper/--digits/--symbols are set)."`
	NoSymbols     bool    `help:"Exclude symbols from generated password (only when no --lower/--upper/--digits/--symbols are set)."`
	SymbolChars   *string `help:"Custom set of symbol characters to use instead of the default (!@#$%^&*()-_=+[]{}:,.?)." placeholder:"CHARS"`
}

// asOpts adapts to the existing passwordOpts type used by selectPassword.
func (f passwordFlags) asOpts() passwordOpts {
	return passwordOpts{
		provided:      f.Password,
		passwordStdin: f.PasswordStdin,
		generate:      f.Generate,
		length:        f.Length,
		lower:         f.Lower,
		upper:         f.Upper,
		digits:        f.Digits,
		symbols:       f.Symbols,
		noLower:       f.NoLower,
		noUpper:       f.NoUpper,
		noDigits:      f.NoDigits,
		noSymbols:     f.NoSymbols,
		symbolChars:   f.SymbolChars,
	}
}

// touched reports whether any password-related flag was set.
func (f passwordFlags) touched() bool {
	return f.Password != nil || f.PasswordStdin || f.Generate
}

// InsertCmd creates a new entry, optionally replacing an existing one at the
// same path with --force.
type InsertCmd struct {
	Entry    string   `arg:"" help:"New entry path."`
	Username *string  `short:"u" help:"Set username on the new entry."`
	URL      *string  `help:"Set URL on the new entry."`
	Notes    *string  `help:"Set notes on the new entry."`
	OTP      *string  `help:"Set OTP/TOTP URI on the new entry."`
	Tags     []string `help:"Set tags (comma or semicolon separated). Repeatable."`
	Field    []string `short:"F" help:"Set a custom field (key=value). Repeatable."`
	Force    bool     `short:"f" help:"Replace an existing entry at that path."`
	JSON     bool     `help:"Output as JSON."`

	Password passwordFlags `embed:""`
}

func (cmd *InsertCmd) Run(c *ctx) error {
	target := runtimex.NormalizePath(cmd.Entry)
	if target == "" {
		return &UserError{Msg: "Entry path cannot be empty."}
	}

	if err := c.openDatabase(); err != nil {
		return err
	}

	existing := c.db.FindEntryByExactPath(target)
	if existing != nil && !cmd.Force {
		return &UserError{Msg: fmt.Sprintf("Entry already exists: %s", target)}
	}

	password, err := cmd.Password.asOpts().selectPassword(c, "Entry password: ", true)
	if err != nil {
		return wrapForUser(err)
	}

	if existing != nil {
		existing.SetField("password", password)
		c.db.ApplyFields(existing, cmd.Username, cmd.URL, cmd.Notes, cmd.OTP, true)
		if len(cmd.Tags) > 0 {
			existing.SetTags(cmd.Tags)
		}
		if err := applyCustomFields(existing, cmd.Field); err != nil {
			return err
		}
	} else {
		parts := runtimex.SplitPath(target)
		title := parts[len(parts)-1]
		parent := c.db.EnsureGroup(runtimex.JoinPath(parts[:len(parts)-1]))
		entry := c.db.CreateEntry(parent, title, derefOr(cmd.Username, ""), password, derefOr(cmd.URL, ""), derefOr(cmd.Notes, ""), derefOr(cmd.OTP, ""))
		c.db.ApplyFields(entry, cmd.Username, cmd.URL, cmd.Notes, cmd.OTP, true)
		if len(cmd.Tags) > 0 {
			entry.SetTags(cmd.Tags)
		}
		if err := applyCustomFields(entry, cmd.Field); err != nil {
			return err
		}
	}
	if err := c.db.Save(); err != nil {
		return &UserError{Msg: err.Error()}
	}

	if cmd.JSON {
		data, _ := json.Marshal(map[string]any{
			"status": "ok",
			"path":   target,
			"action": "inserted",
		})
		fmt.Fprintln(c.out, string(data))
		return nil
	}

	fmt.Fprintln(c.out, target)
	return nil
}

// applyCustomFields parses key=value pairs from the --field flags and sets
// them on the entry.
func applyCustomFields(entry *db.Entry, fields []string) error {
	for _, f := range fields {
		parts := strings.SplitN(f, "=", 2)
		if len(parts) != 2 || parts[0] == "" {
			return &UserError{Msg: fmt.Sprintf("Invalid custom field format: %q (use key=value)", f)}
		}
		entry.SetField(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
	}
	return nil
}

func derefOr(p *string, fallback string) string {
	if p == nil {
		return fallback
	}
	return *p
}

func wrapForUser(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := err.(*UserError); ok {
		return err
	}
	return &UserError{Msg: err.Error()}
}
