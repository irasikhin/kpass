package db

import (
	"sort"
	"strings"

	"github.com/tobischo/gokeepasslib/v3"
)

// SortedEntries walks every entry beneath the (single) root group, returning
// wrappers sorted by lowercase display path. Mirrors Python sorted_entries.
func (d *DB) SortedEntries() []*Entry {
	root := d.rootGroup()
	if root == nil {
		return nil
	}
	var out []*Entry
	d.collectEntries(root, nil, &out)
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].DisplayPath()) < strings.ToLower(out[j].DisplayPath())
	})
	return out
}

// SortedGroups mirrors Python sorted_groups: every group except root.
func (d *DB) SortedGroups() []*Group {
	root := d.rootGroup()
	if root == nil {
		return nil
	}
	var out []*Group
	d.collectGroups(root, nil, &out)
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].DisplayPath()) < strings.ToLower(out[j].DisplayPath())
	})
	return out
}

// RootGroup returns the wrapped root group with empty path.
func (d *DB) RootGroup() *Group {
	r := d.rootGroup()
	if r == nil {
		return nil
	}
	return &Group{d: d, g: r}
}

func (d *DB) rootGroup() *gokeepasslib.Group {
	if d.Raw == nil || d.Raw.Content == nil || d.Raw.Content.Root == nil {
		return nil
	}
	if len(d.Raw.Content.Root.Groups) == 0 {
		return nil
	}
	return &d.Raw.Content.Root.Groups[0]
}

func (d *DB) collectEntries(g *gokeepasslib.Group, prefix []string, out *[]*Entry) {
	// Skip root group's own name; pykeepass display_path joins entry.path which
	// does not include the root.
	groupPath := prefix
	if len(prefix) > 0 || g != d.rootGroup() {
		groupPath = append(append([]string{}, prefix...), g.Name)
	}
	for i := range g.Entries {
		e := &g.Entries[i]
		path := append(append([]string{}, groupPath...), e.GetTitle())
		*out = append(*out, &Entry{d: d, e: e, parent: g, Path: path})
	}
	for i := range g.Groups {
		d.collectEntries(&g.Groups[i], groupPath, out)
	}
}

func (d *DB) collectGroups(g *gokeepasslib.Group, prefix []string, out *[]*Group) {
	groupPath := prefix
	if len(prefix) > 0 || g != d.rootGroup() {
		groupPath = append(append([]string{}, prefix...), g.Name)
		*out = append(*out, &Group{d: d, g: g, Path: groupPath})
	}
	for i := range g.Groups {
		d.collectGroups(&g.Groups[i], groupPath, out)
	}
}
