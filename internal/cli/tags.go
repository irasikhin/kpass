package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/irasikhin/kpass/internal/color"
)

// TagsCmd lists all unique tags in the database with entry counts.
type TagsCmd struct {
	Sort  string `default:"count" enum:"count,name" help:"Sort by count (desc) or name (asc)."`
	Names bool   `help:"Print just tag names, no counts."`
	JSON  bool   `help:"Output as JSON."`
}

func (cmd *TagsCmd) Run(c *ctx) error {
	if err := c.openDatabase(); err != nil {
		return err
	}

	counts := c.db.AllTags()
	type pair struct {
		Tag   string `json:"tag"`
		Count int    `json:"count"`
	}
	pairs := make([]pair, 0, len(counts))
	for t, n := range counts {
		pairs = append(pairs, pair{Tag: t, Count: n})
	}
	switch cmd.Sort {
	case "name":
		sort.Slice(pairs, func(i, j int) bool { return pairs[i].Tag < pairs[j].Tag })
	default:
		sort.Slice(pairs, func(i, j int) bool {
			if pairs[i].Count != pairs[j].Count {
				return pairs[i].Count > pairs[j].Count
			}
			return pairs[i].Tag < pairs[j].Tag
		})
	}

	if cmd.JSON {
		data, err := json.MarshalIndent(pairs, "", "  ")
		if err != nil {
			return &UserError{Msg: err.Error()}
		}
		fmt.Fprintln(c.out, string(data))
		return nil
	}

	if cmd.Names {
		for _, p := range pairs {
			fmt.Fprintln(c.out, p.Tag)
		}
		return nil
	}

	if len(pairs) == 0 {
		fmt.Fprintln(c.out, color.Faint("No tags."))
		return nil
	}

	width := 0
	for _, p := range pairs {
		if len(p.Tag) > width {
			width = len(p.Tag)
		}
	}
	for _, p := range pairs {
		fmt.Fprintf(c.out, "%-*s  %s\n", width, p.Tag, color.Faint(fmt.Sprintf("%d", p.Count)))
	}
	return nil
}
