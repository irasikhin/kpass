package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/irasikhin/kpass/internal/color"
	"github.com/irasikhin/kpass/internal/db"
	"github.com/irasikhin/kpass/internal/tree"
)

// SearchCmd finds entries by matching against one or more fields.
type SearchCmd struct {
	Term       string   `arg:"" help:"Term to match."`
	Field      []string `short:"F" help:"Field to search (path|title|username|password|url|notes|otp or custom field). Repeatable. Default: path,title."`
	ShowField  bool     `help:"Show which fields matched for each result."`
	Format     string   `short:"o" default:"tree" enum:"tree,flat,verbose" help:"Output format: tree (default), flat, or verbose."`
	IgnoreCase bool     `short:"i" help:"Case-insensitive match (default)." default:"true"`
	Tag        []string `help:"Filter by tag (AND). Repeatable."`
	TagAny     []string `name:"tag-any" help:"Filter by tag (OR — at least one matches). Repeatable."`
	JSON       bool     `help:"Output as JSON."`
}

func (cmd *SearchCmd) Run(c *ctx) error {
	fields := normalizeFields(cmd.Field)
	if len(fields) == 0 {
		fields = []string{"path", "title"}
	}

	if err := c.openDatabase(); err != nil {
		return err
	}

	needle := cmd.Term
	if cmd.IgnoreCase {
		needle = strings.ToLower(cmd.Term)
	}

	type match struct {
		entry     *db.Entry
		hitFields []string
	}

	var matches []match
	for _, e := range c.db.SortedEntries() {
		if !matchTagFilter(e, cmd.Tag, cmd.TagAny) {
			continue
		}

		var hitFields []string
		for _, f := range fields {
			hay := e.SearchableField(f)
			if cmd.IgnoreCase {
				hay = strings.ToLower(hay)
			}
			if strings.Contains(hay, needle) {
				hitFields = append(hitFields, f)
			}
		}
		if len(hitFields) > 0 {
			matches = append(matches, match{entry: e, hitFields: hitFields})
		}
	}

	if cmd.JSON {
		type searchResult struct {
			Path      string   `json:"path"`
			HitFields []string `json:"hit_fields,omitempty"`
		}
		results := make([]searchResult, len(matches))
		for i, m := range matches {
			results[i] = searchResult{Path: m.entry.DisplayPath()}
			if cmd.ShowField {
				results[i].HitFields = m.hitFields
			}
		}
		data, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return &UserError{Msg: err.Error()}
		}
		fmt.Fprintln(c.out, string(data))
		return nil
	}

	switch cmd.Format {
	case "flat":
		for _, m := range matches {
			if cmd.ShowField {
				fmt.Fprintf(c.out, "%s %s\n", m.entry.DisplayPath(), color.Faint(fmt.Sprintf("(%s)", strings.Join(m.hitFields, ", "))))
			} else {
				fmt.Fprintln(c.out, m.entry.DisplayPath())
			}
		}
	case "verbose":
		for i, m := range matches {
			if i > 0 {
				fmt.Fprintln(c.out)
			}
			fmt.Fprintln(c.out, color.Bold(m.entry.DisplayPath()))
			if cmd.ShowField {
				fmt.Fprintf(c.out, "  %s %s\n", color.Faint("matched:"), strings.Join(m.hitFields, ", "))
			}
			for _, f := range m.hitFields {
				v := m.entry.SearchableField(f)
				fmt.Fprintf(c.out, "  %s %s\n", color.Cyan(f+":"), v)
			}
		}
	default:
		// tree
		entries := make([]*tree.EntryInfo, len(matches))
		for i, m := range matches {
			info := &tree.EntryInfo{Path: m.entry.DisplayPath()}
			if cmd.ShowField {
				info.Suffix = fmt.Sprintf(" (%s)", strings.Join(m.hitFields, ", "))
			}
			entries[i] = info
		}
		fmt.Fprintln(c.out, tree.RenderRich(entries, "Search: "+cmd.Term, 0))
	}
	return nil
}
