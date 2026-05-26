package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/irasikhin/kpass/internal/color"
	"github.com/irasikhin/kpass/internal/config"
	"github.com/irasikhin/kpass/internal/pwgen"
	"github.com/irasikhin/kpass/internal/runtimex"
)

// passwordStrengthLine returns a one-line strength display for a password.
func passwordStrengthLine(password string) string {
	s := pwgen.Assess(password)
	label := s.Label
	switch {
	case s.Bits < 28:
		label = color.Red(s.Label)
	case s.Bits < 60:
		label = color.Yellow(s.Label)
	default:
		label = color.Green(s.Label)
	}
	return fmt.Sprintf("Strength: %s %s (%.0f bits)", s.Bar, label, s.Bits)
}

// GenerateCmd creates an entry (or replaces its password) with a freshly
// generated password. Multiple entries can be specified for batch generation.
type GenerateCmd struct {
	Entry       []string `arg:"" optional:"" help:"Entry path(s) or glob pattern(s). Use with --all to target all entries."`
	All         bool     `help:"Generate new passwords for all existing entries (optionally filtered by glob pattern)."`
	Timeout     int      `default:"-1" help:"Clipboard timeout when --copy is set (seconds)."`
	Username    *string  `short:"u" help:"Set username on a new entry."`
	URL         *string  `help:"Set URL on a new entry."`
	Notes       *string  `help:"Set notes on a new entry."`
	OTP         *string  `help:"Set OTP/TOTP URI on a new entry."`
	Force       bool     `short:"f" help:"Replace password of existing entries."`
	Length      int      `short:"L" default:"24" help:"Generated password length."`
	Lower       bool     `help:"Allow lowercase in generated password."`
	Upper       bool     `help:"Allow uppercase in generated password."`
	Digits      bool     `help:"Allow digits in generated password."`
	Symbols     bool     `help:"Allow symbols in generated password."`
	NoLower     bool     `help:"Exclude lowercase from generated password (only when no --lower/--upper/--digits/--symbols are set)."`
	NoUpper     bool     `help:"Exclude uppercase from generated password (only when no --lower/--upper/--digits/--symbols are set)."`
	NoDigits    bool     `help:"Exclude digits from generated password (only when no --lower/--upper/--digits/--symbols are set)."`
	NoSymbols   bool     `help:"Exclude symbols from generated password (only when no --lower/--upper/--digits/--symbols are set)."`
	SymbolChars *string  `help:"Custom set of symbol characters to use instead of the default (!@#$%^&*()-_=+[]{}:,.?)." placeholder:"CHARS"`
	Copy        bool     `aliases:"clip" help:"Copy the generated password to the clipboard."`
	JSON        bool     `help:"Output as JSON."`
	Tag         []string `help:"Filter by tag (AND). Repeatable."`
	TagAny      []string `name:"tag-any" help:"Filter by tag (OR — at least one matches). Repeatable."`
}

func (cmd *GenerateCmd) Run(c *ctx) error {
	// If no actionable arguments, show help.
	if len(cmd.Entry) == 0 && !cmd.All {
		return errHelpRequested
	}

	timeout := config.DefaultClipboardTimeout
	if cmd.Timeout >= 0 {
		timeout = cmd.Timeout
	}

	if err := c.openDatabase(); err != nil {
		return err
	}

	// Determine target entries.
	var targets []string
	if cmd.All {
		// Regenerate all existing entries (optionally filtered).
		filter := ""
		if len(cmd.Entry) > 0 {
			filter = cmd.Entry[0]
		}
		for _, e := range c.db.SortedEntries() {
			if filter != "" && !matchGlob(filter, e.DisplayPath()) {
				continue
			}
			if !matchTagFilter(e, cmd.Tag, cmd.TagAny) {
				continue
			}
			targets = append(targets, e.DisplayPath())
		}
		if len(targets) == 0 {
			return &UserError{Msg: "No matching entries found."}
		}
		// Batch confirmation.
		ok, err := confirm(c, fmt.Sprintf("Regenerate passwords for %d entries", len(targets)), targets...)
		if err != nil {
			return &UserError{Msg: err.Error()}
		}
		if !ok {
			return &UserError{Msg: "Aborted."}
		}
	} else {
		targets = cmd.Entry
	}

	// Resolve character set: --no-* flags only apply when no --lower/--upper/--digits/--symbols are set.
	lower, upper, digits, symbols := resolveCharsetFlags(cmd.Lower, cmd.Upper, cmd.Digits, cmd.Symbols,
		cmd.NoLower, cmd.NoUpper, cmd.NoDigits, cmd.NoSymbols)

	symbolChars := ""
	if cmd.SymbolChars != nil {
		symbolChars = *cmd.SymbolChars
	}

	generated := 0
	for _, target := range targets {
		t := runtimex.NormalizePath(target)
		if t == "" {
			continue
		}

		entry := c.db.FindEntryByExactPath(t)
		if entry == nil {
			if cmd.All {
				continue // skip non-existent in --all mode
			}
			parts := runtimex.SplitPath(t)
			title := parts[len(parts)-1]
			parent := c.db.EnsureGroup(runtimex.JoinPath(parts[:len(parts)-1]))
			entry = c.db.CreateEntry(parent, title, "", "", "", "", "")
			c.db.ApplyFields(entry, cmd.Username, cmd.URL, cmd.Notes, cmd.OTP, true)
		} else if !cmd.Force && !cmd.All {
			return &UserError{Msg: fmt.Sprintf("Entry already exists: %s. Use --force to replace its password.", t)}
		} else if cmd.Force || cmd.All {
			c.db.ApplyFields(entry, cmd.Username, cmd.URL, cmd.Notes, cmd.OTP, false)
		}

		password, err := pwgen.Generate(cmd.Length, lower, upper, digits, symbols, symbolChars)
		if err != nil {
			return &UserError{Msg: err.Error()}
		}
		entry.SetField("password", password)
		generated++

		if cmd.Copy && len(targets) == 1 {
			if err := clipboardWrite(password, timeout); err != nil {
				return &UserError{Msg: err.Error()}
			}
			fmt.Fprintf(c.out, "%s %s %s\n",
				color.Green("Generated new password for"),
				color.Bold(entry.DisplayPath()),
				color.Faint("and copied it to clipboard"))
			fmt.Fprintln(c.out, color.Faint(passwordStrengthLine(password)))
		} else {
			if len(targets) == 1 {
				fmt.Fprintln(c.out, entry.DisplayPath())
				fmt.Fprintln(c.out, color.Faint(passwordStrengthLine(password)))
			} else {
				fmt.Fprintf(c.out, "  %s %s %s\n",
					color.Green("✓"),
					color.Bold(entry.DisplayPath()),
					color.Faint(passwordStrengthLine(password)))
			}
		}
	}

	if err := c.db.Save(); err != nil {
		return &UserError{Msg: err.Error()}
	}

	if cmd.JSON {
		data, _ := json.Marshal(map[string]any{
			"status":    "ok",
			"count":     generated,
			"generated": generated,
		})
		fmt.Fprintln(c.out, string(data))
		return nil
	}

	if len(targets) > 1 {
		fmt.Fprintf(c.out, "\n%s\n", color.Green(fmt.Sprintf("Generated %d passwords.", generated)))
	}
	return nil
}

// resolveCharsetFlags determines which character classes to use.
// If any include flag (--lower, --upper, --digits, --symbols) is set, use
// opt-in mode: only those explicitly included are used. Otherwise, start with
// all classes and remove any that are excluded via --no-* flags.
func resolveCharsetFlags(lower, upper, digits, symbols bool, noLower, noUpper, noDigits, noSymbols bool) (bool, bool, bool, bool) {
	if lower || upper || digits || symbols {
		// Opt-in mode: use only what was explicitly requested.
		return lower, upper, digits, symbols
	}
	// Opt-out mode: start with all, remove excluded.
	return !noLower, !noUpper, !noDigits, !noSymbols
}

// matchGlob returns true if the path matches the glob pattern.
func matchGlob(pattern, path string) bool {
	ok, _ := filepath.Match(pattern, path)
	return ok
}
