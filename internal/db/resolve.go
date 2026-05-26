package db

import (
	"fmt"
	"strings"

	"github.com/irasikhin/kpass/internal/runtimex"
)

// MatchError signals an entry/group lookup miss or ambiguity. cli converts
// these into exit-code-1 user errors with the same Msg as Python's MatchError.
type MatchError struct{ Msg string }

func (e *MatchError) Error() string { return e.Msg }

// ResolveEntry mirrors Python resolve_entry: exact path → exact title →
// case-insensitive substring on path or title.
func (d *DB) ResolveEntry(query string) (*Entry, error) {
	normalized := runtimex.NormalizePath(query)
	if normalized == "" {
		return nil, &MatchError{Msg: "Entry path cannot be empty."}
	}

	entries := d.SortedEntries()
	pathMatches := filterEntries(entries, func(e *Entry) bool {
		return e.DisplayPath() == normalized
	})
	if len(pathMatches) > 0 {
		return uniqueOrErrorEntry(pathMatches, query, "entry")
	}

	titleMatches := filterEntries(entries, func(e *Entry) bool {
		return e.Title() == normalized
	})
	if len(titleMatches) > 0 {
		return uniqueOrErrorEntry(titleMatches, query, "entry")
	}

	needle := strings.ToLower(normalized)
	partial := filterEntries(entries, func(e *Entry) bool {
		return strings.Contains(strings.ToLower(e.DisplayPath()), needle) ||
			strings.Contains(strings.ToLower(e.Title()), needle)
	})
	return uniqueOrErrorEntry(partial, query, "entry")
}

// FindEntryByExactPath returns the entry with the given exact path, or nil.
func (d *DB) FindEntryByExactPath(path string) *Entry {
	normalized := runtimex.NormalizePath(path)
	if normalized == "" {
		return nil
	}
	for _, e := range d.SortedEntries() {
		if e.DisplayPath() == normalized {
			return e
		}
	}
	return nil
}

// ResolveGroup mirrors Python resolve_group; an empty query returns the root.
func (d *DB) ResolveGroup(query string) (*Group, error) {
	normalized := runtimex.NormalizePath(query)
	if normalized == "" {
		return d.RootGroup(), nil
	}
	groups := d.SortedGroups()
	pathMatches := filterGroups(groups, func(g *Group) bool {
		return g.DisplayPath() == normalized
	})
	if len(pathMatches) > 0 {
		return uniqueOrErrorGroup(pathMatches, query, "group")
	}
	nameMatches := filterGroups(groups, func(g *Group) bool {
		return g.Name() == normalized
	})
	if len(nameMatches) > 0 {
		return uniqueOrErrorGroup(nameMatches, query, "group")
	}
	needle := strings.ToLower(normalized)
	partial := filterGroups(groups, func(g *Group) bool {
		return strings.Contains(strings.ToLower(g.DisplayPath()), needle) ||
			strings.Contains(strings.ToLower(g.Name()), needle)
	})
	return uniqueOrErrorGroup(partial, query, "group")
}

func filterEntries(in []*Entry, pred func(*Entry) bool) []*Entry {
	out := in[:0:0]
	for _, e := range in {
		if pred(e) {
			out = append(out, e)
		}
	}
	return out
}

func filterGroups(in []*Group, pred func(*Group) bool) []*Group {
	out := in[:0:0]
	for _, g := range in {
		if pred(g) {
			out = append(out, g)
		}
	}
	return out
}

func uniqueOrErrorEntry(matches []*Entry, query, kind string) (*Entry, error) {
	if len(matches) == 0 {
		return nil, &MatchError{Msg: fmt.Sprintf("%s not found: %s", capitalize(kind), query)}
	}
	if len(matches) > 1 {
		rendered := make([]string, 0, len(matches))
		for i, m := range matches {
			if i == 8 {
				break
			}
			rendered = append(rendered, m.DisplayPath())
		}
		suffix := ""
		if len(matches) > 8 {
			suffix = " ..."
		}
		return nil, &MatchError{Msg: fmt.Sprintf("Ambiguous %s '%s': %s%s", kind, query, strings.Join(rendered, ", "), suffix)}
	}
	return matches[0], nil
}

func uniqueOrErrorGroup(matches []*Group, query, kind string) (*Group, error) {
	if len(matches) == 0 {
		return nil, &MatchError{Msg: fmt.Sprintf("%s not found: %s", capitalize(kind), query)}
	}
	if len(matches) > 1 {
		rendered := make([]string, 0, len(matches))
		for i, m := range matches {
			if i == 8 {
				break
			}
			rendered = append(rendered, m.DisplayPath())
		}
		suffix := ""
		if len(matches) > 8 {
			suffix = " ..."
		}
		return nil, &MatchError{Msg: fmt.Sprintf("Ambiguous %s '%s': %s%s", kind, query, strings.Join(rendered, ", "), suffix)}
	}
	return matches[0], nil
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
