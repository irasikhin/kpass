package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/irasikhin/kpass/internal/db"
	"github.com/irasikhin/kpass/internal/tree"
)

// LsCmd lists entries (or groups) under an optional group prefix.
type LsCmd struct {
	Group  string   `arg:"" optional:"" help:"Limit to this group path."`
	Flat   bool     `help:"Print plain entry paths instead of a tree."`
	Groups bool     `help:"List groups only."`
	Long   bool     `short:"l" help:"Table format with username, URL, TOTP, and attachment columns."`
	Depth  int      `help:"Limit tree depth (0 = unlimited)."`
	Tag    []string `help:"Filter by tag (AND). Repeatable."`
	TagAny []string `name:"tag-any" help:"Filter by tag (OR — at least one matches). Repeatable."`
	JSON   bool     `help:"Output as JSON."`
}

func (cmd *LsCmd) Run(c *ctx) error {
	if err := c.openDatabase(); err != nil {
		return err
	}

	var prefix string
	if cmd.Group != "" {
		g, err := c.db.ResolveGroup(cmd.Group)
		if err != nil {
			return err
		}
		prefix = g.DisplayPath()
	}

	if cmd.JSON {
		if cmd.Groups {
			var groups []string
			for _, g := range c.db.SortedGroups() {
				p := g.DisplayPath()
				if prefix == "" || p == prefix || strings.HasPrefix(p, prefix+"/") {
					groups = append(groups, p)
				}
			}
			data, err := json.MarshalIndent(groups, "", "  ")
			if err != nil {
				return &UserError{Msg: err.Error()}
			}
			fmt.Fprintln(c.out, string(data))
			return nil
		}
		var infos []db.EntryInfoJSON
		for _, e := range c.db.SortedEntries() {
			p := e.DisplayPath()
			if prefix != "" && p != prefix && !strings.HasPrefix(p, prefix+"/") {
				continue
			}
			if !matchTagFilter(e, cmd.Tag, cmd.TagAny) {
				continue
			}
			infos = append(infos, e.ToInfoJSON())
		}
		data, err := json.MarshalIndent(infos, "", "  ")
		if err != nil {
			return &UserError{Msg: err.Error()}
		}
		fmt.Fprintln(c.out, string(data))
		return nil
	}

	if cmd.Groups {
		for _, g := range c.db.SortedGroups() {
			p := g.DisplayPath()
			if prefix == "" || p == prefix || strings.HasPrefix(p, prefix+"/") {
				fmt.Fprintln(c.out, p)
			}
		}
		return nil
	}

	entries := []*tree.EntryInfo{}
	for _, e := range c.db.SortedEntries() {
		p := e.DisplayPath()
		if prefix != "" && p != prefix && !strings.HasPrefix(p, prefix+"/") {
			continue
		}
		if !matchTagFilter(e, cmd.Tag, cmd.TagAny) {
			continue
		}
		entries = append(entries, treeEntryInfo(e))
	}

	if cmd.Flat {
		for _, e := range entries {
			fmt.Fprintln(c.out, e.Path)
		}
		return nil
	}

	if cmd.Long {
		fmt.Fprint(c.out, tree.RenderLong(entries))
		return nil
	}

	rootLabel := prefix
	if rootLabel == "" {
		rootLabel = "Password Store"
	}
	fmt.Fprintln(c.out, tree.RenderRich(entries, rootLabel, cmd.Depth))
	return nil
}

// treeEntryInfo builds an EntryInfo from a db.Entry wrapper.
func treeEntryInfo(e *db.Entry) *tree.EntryInfo {
	info := &tree.EntryInfo{Path: e.DisplayPath()}
	if e.Raw().GetPassword() != "" {
		info.HasPassword = true
	}
	if e.Raw().GetContent("URL") != "" {
		info.HasURL = true
	}
	if e.Raw().GetContent("Notes") != "" {
		info.HasNotes = true
	}
	if len(e.Raw().Binaries) > 0 {
		info.AttachCount = len(e.Raw().Binaries)
	}
	if e.OtpURI() != "" {
		info.HasOTP = true
	}
	info.Username = e.Raw().GetContent("UserName")
	info.URL = e.Raw().GetContent("URL")
	if tags := e.Tags(); len(tags) > 0 {
		info.Tags = strings.Join(tags, ",")
	}
	return info
}
