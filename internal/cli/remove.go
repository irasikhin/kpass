package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/irasikhin/kpass/internal/color"
	"github.com/irasikhin/kpass/internal/db"
)

// RemoveCmd deletes one or more entries, prompting for confirmation unless -f
// is given. Accepts glob patterns (e.g. "work/*") for batch deletion.
type RemoveCmd struct {
	Entry  []string `arg:"" help:"Entry path(s) or glob pattern(s) to delete."`
	Force  bool     `short:"f" help:"Skip the y/N confirmation prompt."`
	JSON   bool     `help:"Output as JSON."`
	DryRun bool     `name:"dry-run" help:"Show what would be deleted without actually deleting."`
	Tag    []string `help:"Filter by tag (AND). Repeatable."`
	TagAny []string `name:"tag-any" help:"Filter by tag (OR — at least one matches). Repeatable."`
}

func (cmd *RemoveCmd) Run(c *ctx) error {
	if err := c.openDatabase(); err != nil {
		return err
	}

	// Resolve all entries matching the patterns.
	entries, err := expandEntries(c.db, cmd.Entry)
	if err != nil {
		return err
	}

	// Apply tag filter.
	if len(cmd.Tag) > 0 || len(cmd.TagAny) > 0 {
		filtered := entries[:0]
		for _, e := range entries {
			if matchTagFilter(e, cmd.Tag, cmd.TagAny) {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	if len(entries) == 0 {
		return &UserError{Msg: "No matching entries found."}
	}

	if !cmd.Force {
		// Show summary of all entries.
		if len(entries) == 1 {
			printEntryContext(c.out, entries[0])
			ok, err := confirm(c, "Delete", entries[0].DisplayPath())
			if err != nil {
				return &UserError{Msg: err.Error()}
			}
			if !ok {
				return &UserError{Msg: "Aborted."}
			}
		} else {
			details := make([]string, len(entries))
			for i, e := range entries {
				details[i] = e.DisplayPath()
				if u := e.Raw().GetContent("UserName"); u != "" {
					details[i] += color.Faint(" (" + u + ")")
				}
			}
			ok, err := confirm(c, fmt.Sprintf("Delete %d entries", len(entries)), details...)
			if err != nil {
				return &UserError{Msg: err.Error()}
			}
			if !ok {
				return &UserError{Msg: "Aborted."}
			}
		}
	}

	if cmd.DryRun {
		fmt.Fprintf(c.out, "%s %s\n", color.Yellow("Would delete"), color.Bold(fmt.Sprintf("%d entries", len(entries))))
		for _, e := range entries {
			fmt.Fprintf(c.out, "  %s\n", color.Bold(e.DisplayPath()))
		}
		return nil
	}

	// Track parent groups to check for emptiness after deletion.
	parentPaths := map[string]bool{}
	for _, e := range entries {
		if pp := e.ParentPath(); pp != "" {
			parentPaths[pp] = true
		}
		if err := c.db.DeleteEntry(e); err != nil {
			return &UserError{Msg: err.Error()}
		}
	}

	// Check and offer to clean empty groups.
	if !cmd.Force {
		for pp := range parentPaths {
			empty := true
			for _, e := range c.db.SortedEntries() {
				if e.ParentPath() == pp {
					empty = false
					break
				}
			}
			if empty {
				ok, err := confirm(c, "Remove empty group", pp)
				if err != nil {
					return &UserError{Msg: err.Error()}
				}
				if ok {
					if err := c.db.RemoveGroup(pp); err != nil {
						return &UserError{Msg: err.Error()}
					}
				}
			}
		}
	}

	if err := c.db.Save(); err != nil {
		return &UserError{Msg: err.Error()}
	}

	if cmd.JSON {
		paths := make([]string, len(entries))
		for i, e := range entries {
			paths[i] = e.DisplayPath()
		}
		data, _ := json.Marshal(map[string]any{
			"status": "ok",
			"count":  len(entries),
			"paths":  paths,
		})
		fmt.Fprintln(c.out, string(data))
		return nil
	}

	if len(entries) == 1 {
		fmt.Fprintln(c.out, color.Green("Deleted 1 entry."))
	} else {
		fmt.Fprintf(c.out, "%s\n", color.Green(fmt.Sprintf("Deleted %d entries.", len(entries))))
	}
	return nil
}

func printEntryContext(out io.Writer, e *db.Entry) {
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, color.Bold(e.DisplayPath()))
	raw := e.Raw()
	if u := raw.GetContent("UserName"); u != "" {
		fmt.Fprintf(out, "  %s %s\n", color.Cyan("Username:"), u)
	}
	if u := raw.GetContent("URL"); u != "" {
		fmt.Fprintf(out, "  %s %s\n", color.Cyan("URL:"), u)
	}
	if e.OtpURI() != "" {
		fmt.Fprintf(out, "  %s %s\n", color.Cyan("TOTP:"), color.Green("yes"))
	}
	if atts := e.AttachmentList(); len(atts) > 0 {
		fmt.Fprintf(out, "  %s %s\n", color.Cyan("Attachments:"), strings.Join(atts, ", "))
	}
	if n := raw.GetContent("Notes"); n != "" {
		firstLine := strings.SplitN(n, "\n", 2)[0]
		if len(firstLine) > 60 {
			firstLine = firstLine[:57] + "…"
		}
		fmt.Fprintf(out, "  %s %s\n", color.Cyan("Notes:"), firstLine)
	}
}

// expandEntries resolves patterns or paths to a list of entries.
func expandEntries(d *db.DB, patterns []string) ([]*db.Entry, error) {
	var result []*db.Entry
	allEntries := d.SortedEntries()
	for _, pat := range patterns {
		// Try exact match first.
		matched := false
		for _, e := range allEntries {
			if e.DisplayPath() == pat {
				result = append(result, e)
				matched = true
			}
		}
		if matched {
			continue
		}
		// Try glob match.
		globMatched := false
		for _, e := range allEntries {
			if ok, _ := filepath.Match(pat, e.DisplayPath()); ok {
				result = append(result, e)
				globMatched = true
			}
		}
		if !globMatched {
			// Fall back to ResolveEntry for partial matching.
			e, err := d.ResolveEntry(pat)
			if err != nil {
				return nil, err
			}
			result = append(result, e)
		}
	}
	// Deduplicate.
	seen := map[string]bool{}
	unique := result[:0]
	for _, e := range result {
		if !seen[e.DisplayPath()] {
			seen[e.DisplayPath()] = true
			unique = append(unique, e)
		}
	}
	return unique, nil
}
