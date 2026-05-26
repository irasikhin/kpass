package db

import (
	"strings"

	"github.com/tobischo/gokeepasslib/v3"

	"github.com/irasikhin/kpass/internal/runtimex"
)

// DB wraps a gokeepasslib database with the filesystem context required to
// re-save it. Protected entries are unlocked while DB is "open" and re-locked
// in Save.
type DB struct {
	Path             string
	KeyFile          string
	Raw              *gokeepasslib.Database
	BackupKeep       int // max .bak files to keep (0 = unlimited)
	BackupMaxAgeDays int // delete .bak files older than N days (0 = forever)
}

// Entry is a wrapper that carries the entry's full path so callers don't have
// to walk the group tree to display it. e and parent point into d.Raw.
type Entry struct {
	d      *DB
	e      *gokeepasslib.Entry
	parent *gokeepasslib.Group
	Path   []string
}

// Group similarly carries its full path. g may be nil for the synthetic root.
type Group struct {
	d    *DB
	g    *gokeepasslib.Group
	Path []string
}

func (e *Entry) Raw() *gokeepasslib.Entry    { return e.e }
func (e *Entry) Parent() *gokeepasslib.Group { return e.parent }
func (e *Entry) DisplayPath() string         { return runtimex.JoinPath(e.Path) }
func (e *Entry) ParentPath() string {
	if len(e.Path) <= 1 {
		return ""
	}
	return runtimex.JoinPath(e.Path[:len(e.Path)-1])
}
func (e *Entry) Title() string {
	if len(e.Path) == 0 {
		return ""
	}
	return e.Path[len(e.Path)-1]
}

func (g *Group) Raw() *gokeepasslib.Group { return g.g }
func (g *Group) DisplayPath() string      { return runtimex.JoinPath(g.Path) }
func (g *Group) Name() string {
	if len(g.Path) == 0 {
		return ""
	}
	return g.Path[len(g.Path)-1]
}

func entryGet(e *gokeepasslib.Entry, key string) string {
	return e.GetContent(key)
}

func entrySet(e *gokeepasslib.Entry, key, value string, protected bool) {
	if i := e.GetIndex(key); i >= 0 {
		e.Values[i].Value.Content = value
		return
	}
	v := gokeepasslib.ValueData{Key: key, Value: gokeepasslib.V{Content: value}}
	if protected {
		v.Value.Protected = newBoolWrapper(true)
	}
	e.Values = append(e.Values, v)
}

// EntryTitle gets the title; pykeepass treats it specially.
func entryTitle(e *gokeepasslib.Entry) string { return e.GetContent("Title") }

// Tags returns the tags on this entry.
func (e *Entry) Tags() []string { return entryTags(e.e) }

// SetTags replaces the tags on this entry.
func (e *Entry) SetTags(tags []string) {
	filtered := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t != "" {
			filtered = append(filtered, t)
		}
	}
	e.e.Tags = strings.Join(filtered, ";")
}

// AllTags walks every entry and tallies how many entries carry each tag.
// Returns a map keyed by tag (case preserved as written by the user); the
// value is the entry count.
func (d *DB) AllTags() map[string]int {
	counts := map[string]int{}
	for _, e := range d.SortedEntries() {
		for _, t := range e.Tags() {
			counts[t]++
		}
	}
	return counts
}

// EntryTags returns the semicolon-joined Tags field split.
func entryTags(e *gokeepasslib.Entry) []string {
	if e.Tags == "" {
		return nil
	}
	parts := strings.Split(e.Tags, ";")
	out := parts[:0]
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
