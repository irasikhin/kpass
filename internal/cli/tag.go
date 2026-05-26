package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/irasikhin/kpass/internal/color"
	"github.com/irasikhin/kpass/internal/db"
)

// TagCmd is the noun-of-actions for bulk tag operations on entries.
type TagCmd struct {
	Add    TagAddCmd    `cmd:"" help:"Add a tag to one or more entries."`
	Remove TagRemoveCmd `cmd:"" aliases:"rm" help:"Remove a tag from one or more entries."`
	Rename TagRenameCmd `cmd:"" aliases:"mv" help:"Rename a tag everywhere it occurs."`
}

// Help returns extended help for tag.
func (TagCmd) Help() string {
	return `Bulk tag operations on entries.

  tag add <tag> <entry>...    – Attach a tag (case-insensitive dedup).
  tag remove <tag> <entry>... – Strip a tag from the listed entries.
  tag rename <old> <new>      – Rename a tag across the whole database.

Tags are stored as semicolon-separated strings in KeePass. Use
` + "`kpass tags`" + ` to list all unique tags with entry counts.
Filter entries by tag with --tag / --tag-any on ls, search, pick,
remove, generate --all, and export.`
}

// TagAddCmd attaches a tag to every entry given.
type TagAddCmd struct {
	Tag     string   `arg:"" help:"Tag to add."`
	Entries []string `arg:"" help:"One or more entry paths." name:"entry"`
}

func (cmd *TagAddCmd) Run(c *ctx) error {
	if err := c.openDatabase(); err != nil {
		return err
	}
	changed := 0
	for _, ep := range cmd.Entries {
		entry, err := c.db.ResolveEntry(ep)
		if err != nil {
			return err
		}
		if addTag(entry, cmd.Tag) {
			changed++
		}
	}
	if changed == 0 {
		fmt.Fprintln(c.out, color.Faint("No entries needed updates."))
		return nil
	}
	if err := c.db.Save(); err != nil {
		return &UserError{Msg: err.Error()}
	}
	fmt.Fprintf(c.out, "%s %s %s %d %s\n",
		color.Green("Added"), color.Bold(cmd.Tag),
		color.Faint("to"), changed, color.Faint("entries"))
	return nil
}

// TagRemoveCmd strips a tag from the listed entries.
type TagRemoveCmd struct {
	Tag     string   `arg:"" help:"Tag to remove."`
	Entries []string `arg:"" help:"One or more entry paths." name:"entry"`
}

func (cmd *TagRemoveCmd) Run(c *ctx) error {
	if err := c.openDatabase(); err != nil {
		return err
	}
	changed := 0
	for _, ep := range cmd.Entries {
		entry, err := c.db.ResolveEntry(ep)
		if err != nil {
			return err
		}
		if removeTag(entry, cmd.Tag) {
			changed++
		}
	}
	if changed == 0 {
		fmt.Fprintln(c.out, color.Faint("No entries had that tag."))
		return nil
	}
	if err := c.db.Save(); err != nil {
		return &UserError{Msg: err.Error()}
	}
	fmt.Fprintf(c.out, "%s %s %s %d %s\n",
		color.Yellow("Removed"), color.Bold(cmd.Tag),
		color.Faint("from"), changed, color.Faint("entries"))
	return nil
}

// TagRenameCmd renames a tag database-wide.
type TagRenameCmd struct {
	Old string `arg:"" help:"Existing tag name."`
	New string `arg:"" help:"Replacement tag name."`
}

func (cmd *TagRenameCmd) Run(c *ctx) error {
	if err := c.openDatabase(); err != nil {
		return err
	}
	if strings.TrimSpace(cmd.New) == "" {
		return &UserError{Msg: "New tag name cannot be empty."}
	}
	if strings.EqualFold(cmd.Old, cmd.New) {
		return &UserError{Msg: "Old and new tag are the same."}
	}
	changed := 0
	for _, e := range c.db.SortedEntries() {
		tags := e.Tags()
		if len(tags) == 0 {
			continue
		}
		hit := false
		out := make([]string, 0, len(tags))
		seen := map[string]bool{}
		for _, t := range tags {
			if strings.EqualFold(t, cmd.Old) {
				hit = true
				key := strings.ToLower(cmd.New)
				if seen[key] {
					continue
				}
				seen[key] = true
				out = append(out, cmd.New)
				continue
			}
			key := strings.ToLower(t)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, t)
		}
		if hit {
			e.SetTags(out)
			changed++
		}
	}
	if changed == 0 {
		fmt.Fprintln(c.out, color.Faint("No entries had that tag."))
		return nil
	}
	if err := c.db.Save(); err != nil {
		return &UserError{Msg: err.Error()}
	}
	fmt.Fprintf(c.out, "%s %s %s %s %s %d %s\n",
		color.Green("Renamed"), color.Bold(cmd.Old),
		color.Faint("→"), color.Bold(cmd.New),
		color.Faint("in"), changed, color.Faint("entries"))
	return nil
}

// addTag returns true if the tag was newly added (case-insensitive de-dup).
func addTag(e *db.Entry, tag string) bool {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return false
	}
	tags := e.Tags()
	for _, t := range tags {
		if strings.EqualFold(t, tag) {
			return false
		}
	}
	e.SetTags(append(tags, tag))
	return true
}

// removeTag returns true if the tag was present and is now stripped.
func removeTag(e *db.Entry, tag string) bool {
	tags := e.Tags()
	out := make([]string, 0, len(tags))
	hit := false
	for _, t := range tags {
		if strings.EqualFold(t, tag) {
			hit = true
			continue
		}
		out = append(out, t)
	}
	if !hit {
		return false
	}
	e.SetTags(out)
	return true
}

// sortedUniqueTags is exposed for tests / completion that want a flat list.
func sortedUniqueTags(d *db.DB) []string {
	counts := d.AllTags()
	out := make([]string, 0, len(counts))
	for t := range counts {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}
