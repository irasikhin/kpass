package db

import (
	"strings"
	"testing"

	"github.com/tobischo/gokeepasslib/v3"
	w "github.com/tobischo/gokeepasslib/v3/wrappers"
)

// emptyRawEntry exists for fields_test.go callers that need a bare *gokeepasslib.Entry.
func emptyRawEntry() *gokeepasslib.Entry {
	e := gokeepasslib.NewEntry()
	return &e
}

func boolWrap(b bool) w.BoolWrapper { return w.NewBoolWrapper(b) }

func TestSortedEntries(t *testing.T) {
	d := seedDB(t)
	entries := d.SortedEntries()
	if len(entries) != 4 {
		t.Errorf("entry count = %d, want 4", len(entries))
	}
	paths := make([]string, 0, len(entries))
	for _, e := range entries {
		paths = append(paths, e.DisplayPath())
	}
	// Sort is case-insensitive and stable.
	prev := ""
	for _, p := range paths {
		if strings.ToLower(p) < strings.ToLower(prev) {
			t.Errorf("not sorted: %v", paths)
			break
		}
		prev = p
	}
}

func TestSortedEntries_EmptyDB(t *testing.T) {
	if got := (&DB{}).SortedEntries(); got != nil {
		t.Errorf("empty DB SortedEntries = %v", got)
	}
}

func TestSortedGroups(t *testing.T) {
	d := seedDB(t)
	groups := d.SortedGroups()
	if len(groups) == 0 {
		t.Fatal("no groups returned")
	}
	for _, g := range groups {
		if g.DisplayPath() == "" {
			t.Errorf("root group should not be in sorted groups")
		}
	}
}

func TestSortedGroups_EmptyDB(t *testing.T) {
	if got := (&DB{}).SortedGroups(); got != nil {
		t.Errorf("empty DB SortedGroups = %v", got)
	}
}

func TestRootGroup(t *testing.T) {
	d := seedDB(t)
	if r := d.RootGroup(); r == nil {
		t.Fatal("RootGroup nil")
	}
	if r := (&DB{}).RootGroup(); r != nil {
		t.Errorf("empty DB RootGroup = %v", r)
	}
}

func TestRootGroup_NoGroupsInRaw(t *testing.T) {
	raw := gokeepasslib.NewDatabase()
	raw.Content.Root.Groups = nil
	if r := (&DB{Raw: raw}).RootGroup(); r != nil {
		t.Errorf("expected nil for empty raw Groups, got %v", r)
	}
}

func TestResolveEntry_ExactPath(t *testing.T) {
	d := seedDB(t)
	e, err := d.ResolveEntry("work/email")
	if err != nil {
		t.Fatal(err)
	}
	if e.DisplayPath() != "work/email" {
		t.Errorf("path = %q", e.DisplayPath())
	}
}

func TestResolveEntry_ExactTitle(t *testing.T) {
	d := seedDB(t)
	e, err := d.ResolveEntry("github")
	if err != nil {
		t.Fatal(err)
	}
	if e.Title() != "github" {
		t.Errorf("title = %q", e.Title())
	}
}

func TestResolveEntry_PartialCaseInsensitive(t *testing.T) {
	d := seedDB(t)
	e, err := d.ResolveEntry("BaNk")
	if err != nil {
		t.Fatal(err)
	}
	if e.Title() != "bank" {
		t.Errorf("title = %q", e.Title())
	}
}

func TestResolveEntry_Empty(t *testing.T) {
	d := seedDB(t)
	if _, err := d.ResolveEntry(""); err == nil {
		t.Error("expected MatchError for empty query")
	}
}

func TestResolveEntry_NotFound(t *testing.T) {
	d := seedDB(t)
	_, err := d.ResolveEntry("nonexistent-xyz")
	if err == nil {
		t.Fatal("expected MatchError")
	}
	if _, ok := err.(*MatchError); !ok {
		t.Errorf("err type = %T, want *MatchError", err)
	}
}

func TestResolveEntry_Ambiguous(t *testing.T) {
	// Seed two entries with the same title in different groups.
	raw := gokeepasslib.NewDatabase()
	root := &raw.Content.Root.Groups[0]
	root.Name = "Root"
	g1 := gokeepasslib.NewGroup()
	g1.Name = "a"
	seedEntry(&g1, "shared", "u", "p", "", "", "")
	g2 := gokeepasslib.NewGroup()
	g2.Name = "b"
	seedEntry(&g2, "shared", "u", "p", "", "", "")
	root.Groups = append(root.Groups, g1, g2)
	d := &DB{Raw: raw}
	_, err := d.ResolveEntry("shared")
	if err == nil || !strings.Contains(err.Error(), "Ambiguous") {
		t.Errorf("expected ambiguous error, got %v", err)
	}
}

func TestResolveEntry_AmbiguousTruncatedRender(t *testing.T) {
	raw := gokeepasslib.NewDatabase()
	root := &raw.Content.Root.Groups[0]
	root.Name = "Root"
	for i := range 10 {
		g := gokeepasslib.NewGroup()
		g.Name = string(rune('a' + i))
		seedEntry(&g, "shared", "u", "p", "", "", "")
		root.Groups = append(root.Groups, g)
	}
	d := &DB{Raw: raw}
	_, err := d.ResolveEntry("shared")
	if err == nil || !strings.Contains(err.Error(), "...") {
		t.Errorf("expected ' ...' truncation, got %v", err)
	}
}

func TestFindEntryByExactPath(t *testing.T) {
	d := seedDB(t)
	if e := d.FindEntryByExactPath("work/email"); e == nil {
		t.Error("exact path lookup failed")
	}
	if e := d.FindEntryByExactPath(""); e != nil {
		t.Error("empty path should be nil")
	}
	if e := d.FindEntryByExactPath("nonexistent"); e != nil {
		t.Error("missing path should be nil")
	}
}

func TestResolveGroup_Empty(t *testing.T) {
	d := seedDB(t)
	g, err := d.ResolveGroup("")
	if err != nil {
		t.Fatal(err)
	}
	if g.DisplayPath() != "" {
		t.Errorf("empty query should return root, got %q", g.DisplayPath())
	}
}

func TestResolveGroup_ExactPath(t *testing.T) {
	d := seedDB(t)
	g, err := d.ResolveGroup("empty/nested")
	if err != nil {
		t.Fatal(err)
	}
	if g.DisplayPath() != "empty/nested" {
		t.Errorf("path = %q", g.DisplayPath())
	}
}

func TestResolveGroup_NameAndPartial(t *testing.T) {
	d := seedDB(t)
	g, err := d.ResolveGroup("nested")
	if err != nil {
		t.Fatal(err)
	}
	if g.Name() != "nested" {
		t.Errorf("name = %q", g.Name())
	}

	g2, err := d.ResolveGroup("WoRk")
	if err != nil {
		t.Fatal(err)
	}
	if g2.Name() != "work" {
		t.Errorf("partial name = %q", g2.Name())
	}
}

func TestResolveGroup_NotFound(t *testing.T) {
	d := seedDB(t)
	if _, err := d.ResolveGroup("ghost-group-xyz"); err == nil {
		t.Error("expected MatchError")
	}
}

func TestMatchError_Error(t *testing.T) {
	me := &MatchError{Msg: "bad"}
	if me.Error() != "bad" {
		t.Errorf("Error() = %q", me.Error())
	}
}

func TestCapitalize(t *testing.T) {
	if capitalize("") != "" {
		t.Error("empty")
	}
	if capitalize("hello") != "Hello" {
		t.Errorf("got %q", capitalize("hello"))
	}
	if capitalize("A") != "A" {
		t.Errorf("got %q", capitalize("A"))
	}
}
