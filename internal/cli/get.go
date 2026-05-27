package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/irasikhin/kpass/internal/color"
	"github.com/irasikhin/kpass/internal/db"
)

// GetCmd shows one entry or specific fields.
type GetCmd struct {
	Entry        string   `arg:"" help:"Entry path or partial path."`
	Field        []string `short:"F" help:"Field to print (path|title|username|password|url|notes|otp or custom field). Repeatable."`
	All          bool     `short:"a" help:"Show all fields including password."`
	ShowPassword bool     `short:"P" help:"Show only the password (shortcut for --field password)."`
	Mask         bool     `short:"m" help:"Mask password characters (shows first 2 and last 2 chars)."`
	JSON         bool     `help:"Output as JSON."`
	Glob         bool     `help:"Treat <entry> as a glob pattern (e.g. 'work/*')."`
}

func (cmd *GetCmd) Run(c *ctx) error {
	fields := normalizeFields(cmd.Field)

	// --show-password is a shortcut for --field password.
	if cmd.ShowPassword {
		fields = []string{"password"}
	}

	if err := c.openDatabase(); err != nil {
		return err
	}

	// Resolve entries: single match or glob pattern.
	var entries []*db.Entry
	if cmd.Glob {
		for _, e := range c.db.SortedEntries() {
			if ok, _ := filepath.Match(cmd.Entry, e.DisplayPath()); ok {
				entries = append(entries, e)
			}
		}
		if len(entries) == 0 {
			return &UserError{Msg: fmt.Sprintf("No entries match pattern: %s", cmd.Entry)}
		}
	} else {
		entry, err := c.db.ResolveEntry(cmd.Entry)
		if err != nil {
			return err
		}
		entries = []*db.Entry{entry}
	}

	if cmd.JSON {
		return cmd.runJSON(c, entries, fields)
	}

	// Plain text output.
	for i, entry := range entries {
		if i > 0 {
			fmt.Fprintln(c.out)
		}
		if len(entries) > 1 {
			fmt.Fprintln(c.out, color.Bold(entry.DisplayPath()))
		}
		if len(fields) == 0 {
			if cmd.All {
				fmt.Fprintln(c.out, entry.FormatFullWithPassword(cmd.Mask))
			} else {
				fmt.Fprintln(c.out, entry.FormatFull())
			}
		} else {
			for _, f := range fields {
				v, err := entry.GetAttribute(f)
				if err != nil {
					return &UserError{Msg: err.Error()}
				}
				v, err = resolveFieldValue(f, v)
				if err != nil {
					return &UserError{Msg: err.Error()}
				}
				if (f == "password" || f == "pass") && cmd.Mask {
					v = maskPassword(v)
				}
				if len(entries) > 1 && len(fields) == 1 {
					fmt.Fprintf(c.out, "%s: %s\n", color.Bold(entry.DisplayPath()), v)
				} else {
					fmt.Fprintln(c.out, v)
				}
			}
		}
	}
	return nil
}

func (cmd *GetCmd) runJSON(c *ctx, entries []*db.Entry, fields []string) error {
	if len(entries) == 1 {
		entry := entries[0]
		if len(fields) == 0 {
			data, err := json.MarshalIndent(entry.ToJSON(), "", "  ")
			if err != nil {
				return &UserError{Msg: err.Error()}
			}
			fmt.Fprintln(c.out, string(data))
			return nil
		}
		if len(fields) == 1 {
			v, err := entry.GetAttribute(fields[0])
			if err != nil {
				return &UserError{Msg: err.Error()}
			}
			v, err = resolveFieldValue(fields[0], v)
			if err != nil {
				return &UserError{Msg: err.Error()}
			}
			data, err := json.Marshal(v)
			if err != nil {
				return &UserError{Msg: err.Error()}
			}
			fmt.Fprintln(c.out, string(data))
			return nil
		}
		obj := make(map[string]string, len(fields))
		for _, f := range fields {
			v, err := entry.GetAttribute(f)
			if err != nil {
				return &UserError{Msg: err.Error()}
			}
			v, err = resolveFieldValue(f, v)
			if err != nil {
				return &UserError{Msg: err.Error()}
			}
			obj[f] = v
		}
		data, err := json.MarshalIndent(obj, "", "  ")
		if err != nil {
			return &UserError{Msg: err.Error()}
		}
		fmt.Fprintln(c.out, string(data))
		return nil
	}

	// Multiple entries (glob): JSON array.
	if len(fields) == 0 {
		items := make([]db.EntryJSON, len(entries))
		for i, e := range entries {
			items[i] = e.ToJSON()
		}
		data, err := json.MarshalIndent(items, "", "  ")
		if err != nil {
			return &UserError{Msg: err.Error()}
		}
		fmt.Fprintln(c.out, string(data))
		return nil
	}
	// Single field across multiple entries: map[path]value.
	if len(fields) == 1 {
		obj := make(map[string]string, len(entries))
		for _, e := range entries {
			v, err := e.GetAttribute(fields[0])
			if err != nil {
				return &UserError{Msg: err.Error()}
			}
			v, err = resolveFieldValue(fields[0], v)
			if err != nil {
				return &UserError{Msg: err.Error()}
			}
			obj[e.DisplayPath()] = v
		}
		data, err := json.MarshalIndent(obj, "", "  ")
		if err != nil {
			return &UserError{Msg: err.Error()}
		}
		fmt.Fprintln(c.out, string(data))
		return nil
	}
	// Multiple fields across multiple entries: array of objects.
	items := make([]map[string]string, len(entries))
	for i, e := range entries {
		obj := make(map[string]string, len(fields)+1)
		obj["path"] = e.DisplayPath()
		for _, f := range fields {
			v, err := e.GetAttribute(f)
			if err != nil {
				return &UserError{Msg: err.Error()}
			}
			v, err = resolveFieldValue(f, v)
			if err != nil {
				return &UserError{Msg: err.Error()}
			}
			obj[f] = v
		}
		items[i] = obj
	}
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return &UserError{Msg: err.Error()}
	}
	fmt.Fprintln(c.out, string(data))
	return nil
}

// maskPassword returns a masked version showing first 2 and last 2 characters.
func maskPassword(pw string) string {
	if len(pw) <= 4 {
		return "****"
	}
	return pw[:2] + strings.Repeat("•", len(pw)-4) + pw[len(pw)-2:]
}
