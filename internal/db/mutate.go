package db

import (
	"fmt"

	"github.com/tobischo/gokeepasslib/v3"

	"github.com/irasikhin/kpass/internal/runtimex"
)

// EnsureGroup walks/creates the group path beneath root. Empty path returns
// the root group. Mirrors Python ensure_group.
func (d *DB) EnsureGroup(path string) *Group {
	root := d.rootGroup()
	parts := runtimex.SplitPath(path)
	current := root
	var currentPath []string
	for _, part := range parts {
		currentPath = append(currentPath, part)
		var found *gokeepasslib.Group
		for i := range current.Groups {
			if current.Groups[i].Name == part {
				found = &current.Groups[i]
				break
			}
		}
		if found == nil {
			g := gokeepasslib.NewGroup()
			g.Name = part
			current.Groups = append(current.Groups, g)
			found = &current.Groups[len(current.Groups)-1]
		}
		current = found
	}
	return &Group{d: d, g: current, Path: append([]string(nil), currentPath...)}
}

// CreateEntry appends a new entry to parent and sets the canonical fields.
// Username/password are always set; URL/Notes/OTP only when non-empty (or
// when force=true).
func (d *DB) CreateEntry(parent *Group, title, username, password, url, notes, otp string) *Entry {
	e := gokeepasslib.NewEntry()
	entrySet(&e, "Title", title)
	entrySet(&e, "UserName", username)
	entrySet(&e, "Password", password)
	if url != "" {
		entrySet(&e, "URL", url)
	}
	if notes != "" {
		entrySet(&e, "Notes", notes)
	}
	if otp != "" {
		entrySet(&e, "otp", otp)
	}
	parent.g.Entries = append(parent.g.Entries, e)
	ref := &parent.g.Entries[len(parent.g.Entries)-1]
	return &Entry{
		d:      d,
		e:      ref,
		parent: parent.g,
		Path:   append(append([]string(nil), parent.Path...), title),
	}
}

// ApplyFields mirrors Python apply_entry_fields. When replaceMissing is true,
// nil pointers clear the field; when false, only non-nil pointers update.
func (d *DB) ApplyFields(e *Entry, username, url, notes, otp *string, replaceMissing bool) {
	if replaceMissing || username != nil {
		val := ""
		if username != nil {
			val = *username
		}
		entrySet(e.e, "UserName", val)
	}
	if replaceMissing || url != nil {
		val := ""
		if url != nil {
			val = *url
		}
		entrySet(e.e, "URL", val)
	}
	if replaceMissing || notes != nil {
		val := ""
		if notes != nil {
			val = *notes
		}
		entrySet(e.e, "Notes", val)
	}
	if replaceMissing || otp != nil {
		val := ""
		if otp != nil {
			val = *otp
		}
		entrySet(e.e, "otp", val)
	}
}

// DeleteEntry removes the entry from its parent group. Mirrors Python kp.delete_entry.
func (d *DB) DeleteEntry(e *Entry) error {
	parent := e.parent
	for i := range parent.Entries {
		if &parent.Entries[i] == e.e {
			parent.Entries = append(parent.Entries[:i], parent.Entries[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("entry not found in parent during delete: %s", e.DisplayPath())
}

// MoveEntry removes e from its current parent and re-attaches it under dst.
// Returns a fresh Entry wrapper because the underlying pointer changes.
func (d *DB) MoveEntry(e *Entry, dst *Group) (*Entry, error) {
	// Snapshot the entry's data before mutation.
	value := *e.e
	if err := d.DeleteEntry(e); err != nil {
		return nil, err
	}
	dst.g.Entries = append(dst.g.Entries, value)
	ref := &dst.g.Entries[len(dst.g.Entries)-1]
	return &Entry{
		d:      d,
		e:      ref,
		parent: dst.g,
		Path:   append(append([]string(nil), dst.Path...), ref.GetTitle()),
	}, nil
}

// SetTitle sets the entry's title and updates the cached path.
func (e *Entry) SetTitle(title string) {
	entrySet(e.e, "Title", title)
	if len(e.Path) == 0 {
		e.Path = []string{title}
	} else {
		e.Path[len(e.Path)-1] = title
	}
}

// CloneEntry deep-copies source into a new entry under dst with the given
// title. Custom properties, attachments, and tags are preserved.
func (d *DB) CloneEntry(src *Entry, dst *Group, title string) *Entry {
	cloned := src.e.Clone()
	entrySet(&cloned, "Title", title)
	dst.g.Entries = append(dst.g.Entries, cloned)
	ref := &dst.g.Entries[len(dst.g.Entries)-1]
	return &Entry{
		d:      d,
		e:      ref,
		parent: dst.g,
		Path:   append(append([]string(nil), dst.Path...), title),
	}
}

// EmptyGroups returns the display paths of all groups that contain no entries
// and have no non-empty subgroups. The root group is never included.
func (d *DB) EmptyGroups() []string {
	root := d.rootGroup()
	if root == nil {
		return nil
	}
	// Build a set of groups that have entries.
	hasEntries := map[*gokeepasslib.Group]bool{}
	for _, e := range d.SortedEntries() {
		hasEntries[e.parent] = true
	}
	var result []string
	collectEmpty(root, nil, hasEntries, &result)
	return result
}

func collectEmpty(g *gokeepasslib.Group, path []string, hasEntries map[*gokeepasslib.Group]bool, out *[]string) {
	groupPath := path
	if len(path) > 0 {
		groupPath = append(append([]string(nil), path...), g.Name)
	}
	childHasContent := false
	for i := range g.Groups {
		sub := &g.Groups[i]
		collectEmpty(sub, groupPath, hasEntries, out)
		if !isEmptyGroup(sub, hasEntries) {
			childHasContent = true
		}
	}
	// Report empty groups (no entries, no non-empty children). Skip root.
	if len(path) > 0 && !hasEntries[g] && !childHasContent {
		*out = append(*out, runtimex.JoinPath(groupPath))
	}
}

func isEmptyGroup(g *gokeepasslib.Group, hasEntries map[*gokeepasslib.Group]bool) bool {
	if hasEntries[g] {
		return false
	}
	for i := range g.Groups {
		if !isEmptyGroup(&g.Groups[i], hasEntries) {
			return false
		}
	}
	return true
}

// RemoveGroup deletes a group (and all its subgroups) identified by its
// display path. The root group cannot be removed.
func (d *DB) RemoveGroup(path string) error {
	normalized := runtimex.NormalizePath(path)
	if normalized == "" {
		return fmt.Errorf("cannot remove root group")
	}
	parts := runtimex.SplitPath(normalized)
	root := d.rootGroup()
	if root == nil {
		return fmt.Errorf("database has no root group")
	}
	return removeGroupAt(root, parts)
}

func removeGroupAt(parent *gokeepasslib.Group, path []string) error {
	if len(path) == 0 {
		return nil
	}
	name := path[0]
	for i := range parent.Groups {
		if parent.Groups[i].Name == name {
			if len(path) == 1 {
				// Remove this group.
				parent.Groups = append(parent.Groups[:i], parent.Groups[i+1:]...)
				return nil
			}
			return removeGroupAt(&parent.Groups[i], path[1:])
		}
	}
	return fmt.Errorf("group not found: %s", name)
}
