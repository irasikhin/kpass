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
	pathMatches := filterSlice(entries, func(e *Entry) bool {
		return e.DisplayPath() == normalized
	})
	if len(pathMatches) > 0 {
		return uniqueOrError(pathMatches, query, "entry")
	}

	titleMatches := filterSlice(entries, func(e *Entry) bool {
		return e.Title() == normalized
	})
	if len(titleMatches) > 0 {
		return uniqueOrError(titleMatches, query, "entry")
	}

	needle := strings.ToLower(normalized)
	partial := filterSlice(entries, func(e *Entry) bool {
		return strings.Contains(strings.ToLower(e.DisplayPath()), needle) ||
			strings.Contains(strings.ToLower(e.Title()), needle)
	})
	return uniqueOrError(partial, query, "entry")
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
	pathMatches := filterSlice(groups, func(g *Group) bool {
		return g.DisplayPath() == normalized
	})
	if len(pathMatches) > 0 {
		return uniqueOrError(pathMatches, query, "group")
	}
	nameMatches := filterSlice(groups, func(g *Group) bool {
		return g.Name() == normalized
	})
	if len(nameMatches) > 0 {
		return uniqueOrError(nameMatches, query, "group")
	}
	needle := strings.ToLower(normalized)
	partial := filterSlice(groups, func(g *Group) bool {
		return strings.Contains(strings.ToLower(g.DisplayPath()), needle) ||
			strings.Contains(strings.ToLower(g.Name()), needle)
	})
	return uniqueOrError(partial, query, "group")
}

func filterSlice[T any](in []T, pred func(T) bool) []T {
	out := in[:0:0]
	for _, x := range in {
		if pred(x) {
			out = append(out, x)
		}
	}
	return out
}

// pathItem covers both *Entry and *Group so resolution errors can render the
// candidate list the same way for either kind.
type pathItem interface{ DisplayPath() string }

func uniqueOrError[T pathItem](matches []T, query, kind string) (T, error) {
	var zero T
	if len(matches) == 0 {
		return zero, &MatchError{Msg: fmt.Sprintf("%s not found: %s", capitalize(kind), query)}
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
		return zero, &MatchError{Msg: fmt.Sprintf("Ambiguous %s '%s': %s%s", kind, query, strings.Join(rendered, ", "), suffix)}
	}
	return matches[0], nil
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
