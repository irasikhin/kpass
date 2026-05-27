package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/irasikhin/kpass/internal/color"
	"github.com/irasikhin/kpass/internal/config"
	"github.com/irasikhin/kpass/internal/db"
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

// charset captures the four-class booleans + custom symbol chars used by
// pwgen.Generate. Resolved once per Run from the user's flag mix.
type charset struct {
	lower, upper, digits, symbols bool
	symbolChars                   string
}

func (cmd *GenerateCmd) Run(c *ctx) error {
	if len(cmd.Entry) == 0 && !cmd.All {
		return errHelpRequested
	}
	if err := c.openDatabase(); err != nil {
		return err
	}

	targets, err := cmd.resolveTargets(c)
	if err != nil {
		return err
	}

	cs := cmd.charset()
	generated := 0
	for _, target := range targets {
		entry, password, err := cmd.applyGeneration(c, target, cs)
		if err != nil {
			return err
		}
		if entry == nil {
			continue
		}
		if err := cmd.printEntryResult(c, entry, password, len(targets) == 1); err != nil {
			return err
		}
		generated++
	}

	if err := c.db.Save(); err != nil {
		return &UserError{Msg: err.Error()}
	}
	return cmd.reportFinal(c, generated, len(targets))
}

func (cmd *GenerateCmd) resolveTargets(c *ctx) ([]string, error) {
	if !cmd.All {
		return cmd.Entry, nil
	}
	filter := ""
	if len(cmd.Entry) > 0 {
		filter = cmd.Entry[0]
	}
	var targets []string
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
		return nil, &UserError{Msg: "No matching entries found."}
	}
	ok, err := confirm(c, fmt.Sprintf("Regenerate passwords for %d entries", len(targets)), targets...)
	if err != nil {
		return nil, &UserError{Msg: err.Error()}
	}
	if !ok {
		return nil, &UserError{Msg: "Aborted."}
	}
	return targets, nil
}

func (cmd *GenerateCmd) charset() charset {
	lower, upper, digits, symbols := resolveCharsetFlags(cmd.Lower, cmd.Upper, cmd.Digits, cmd.Symbols,
		cmd.NoLower, cmd.NoUpper, cmd.NoDigits, cmd.NoSymbols)
	symChars := ""
	if cmd.SymbolChars != nil {
		symChars = *cmd.SymbolChars
	}
	return charset{lower: lower, upper: upper, digits: digits, symbols: symbols, symbolChars: symChars}
}

// applyGeneration finds or creates the entry, generates a password, applies
// it, and returns (entry, password). entry is nil when the target should be
// skipped (e.g. --all over a non-existent path).
func (cmd *GenerateCmd) applyGeneration(c *ctx, target string, cs charset) (*db.Entry, string, error) {
	t := runtimex.NormalizePath(target)
	if t == "" {
		return nil, "", nil
	}
	entry := c.db.FindEntryByExactPath(t)
	switch {
	case entry == nil && cmd.All:
		return nil, "", nil
	case entry == nil:
		parts := runtimex.SplitPath(t)
		title := parts[len(parts)-1]
		parent := c.db.EnsureGroup(runtimex.JoinPath(parts[:len(parts)-1]))
		entry = c.db.CreateEntry(parent, title, "", "", "", "", "")
		c.db.ApplyFields(entry, cmd.Username, cmd.URL, cmd.Notes, cmd.OTP, true)
	case !cmd.Force && !cmd.All:
		return nil, "", &UserError{Msg: fmt.Sprintf("Entry already exists: %s. Use --force to replace its password.", t)}
	default:
		c.db.ApplyFields(entry, cmd.Username, cmd.URL, cmd.Notes, cmd.OTP, false)
	}

	password, err := pwgen.Generate(cmd.Length, cs.lower, cs.upper, cs.digits, cs.symbols, cs.symbolChars)
	if err != nil {
		return nil, "", &UserError{Msg: err.Error()}
	}
	entry.SetField("password", password)
	return entry, password, nil
}

func (cmd *GenerateCmd) printEntryResult(c *ctx, entry *db.Entry, password string, singleTarget bool) error {
	if cmd.Copy && singleTarget {
		timeout := config.DefaultClipboardTimeout
		if cmd.Timeout >= 0 {
			timeout = cmd.Timeout
		}
		if err := clipboardWrite(password, timeout); err != nil {
			return &UserError{Msg: err.Error()}
		}
		fmt.Fprintf(c.out, "%s %s %s\n",
			color.Green("Generated new password for"),
			color.Bold(entry.DisplayPath()),
			color.Faint("and copied it to clipboard"))
		fmt.Fprintln(c.out, color.Faint(passwordStrengthLine(password)))
		return nil
	}
	if singleTarget {
		fmt.Fprintln(c.out, entry.DisplayPath())
		fmt.Fprintln(c.out, color.Faint(passwordStrengthLine(password)))
		return nil
	}
	fmt.Fprintf(c.out, "  %s %s %s\n",
		color.Green("✓"),
		color.Bold(entry.DisplayPath()),
		color.Faint(passwordStrengthLine(password)))
	return nil
}

func (cmd *GenerateCmd) reportFinal(c *ctx, generated, total int) error {
	if cmd.JSON {
		data, _ := json.Marshal(map[string]any{
			"status":    "ok",
			"count":     generated,
			"generated": generated,
		})
		fmt.Fprintln(c.out, string(data))
		return nil
	}
	if total > 1 {
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
		return lower, upper, digits, symbols
	}
	return !noLower, !noUpper, !noDigits, !noSymbols
}

// matchGlob returns true if the path matches the glob pattern.
func matchGlob(pattern, path string) bool {
	ok, _ := filepath.Match(pattern, path)
	return ok
}
