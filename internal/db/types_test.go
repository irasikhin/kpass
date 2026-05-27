package db

import (
	"slices"
	"sort"
	"testing"
)

func TestEntryAccessors(t *testing.T) {
	d := seedDB(t)
	entries := d.SortedEntries()
	if len(entries) == 0 {
		t.Fatal("expected entries")
	}
	var email *Entry
	for _, e := range entries {
		if e.DisplayPath() == "work/email" {
			email = e
			break
		}
	}
	if email == nil {
		t.Fatal("work/email not found")
	}
	if email.Title() != "email" {
		t.Errorf("Title = %q", email.Title())
	}
	if email.ParentPath() != "work" {
		t.Errorf("ParentPath = %q", email.ParentPath())
	}
	if email.Raw() == nil || email.Parent() == nil {
		t.Error("Raw/Parent should be non-nil")
	}
}

func TestEntryParentPath_RootLevel(t *testing.T) {
	d := seedDB(t)
	var gh *Entry
	for _, e := range d.SortedEntries() {
		if e.DisplayPath() == "github" {
			gh = e
		}
	}
	if gh == nil {
		t.Fatal("github entry missing")
	}
	if gh.ParentPath() != "" {
		t.Errorf("root entry ParentPath = %q, want empty", gh.ParentPath())
	}
}

func TestEntryTitle_EmptyPath(t *testing.T) {
	e := &Entry{Path: nil}
	if e.Title() != "" {
		t.Errorf("Title empty = %q", e.Title())
	}
}

func TestGroupAccessors(t *testing.T) {
	d := seedDB(t)
	groups := d.SortedGroups()
	names := make([]string, 0, len(groups))
	for _, g := range groups {
		names = append(names, g.Name())
	}
	sort.Strings(names)
	wantContains := []string{"work", "personal", "empty", "nested"}
	for _, w := range wantContains {
		if !slices.Contains(names, w) {
			t.Errorf("group %q missing, got %v", w, names)
		}
	}
}

func TestGroup_NameEmpty(t *testing.T) {
	g := &Group{}
	if g.Name() != "" {
		t.Errorf("empty group Name = %q", g.Name())
	}
	if g.DisplayPath() != "" {
		t.Errorf("empty group DisplayPath = %q", g.DisplayPath())
	}
}

func TestSetTags_FiltersBlanksAndTrims(t *testing.T) {
	d := seedDB(t)
	var e *Entry
	for _, x := range d.SortedEntries() {
		if x.DisplayPath() == "work/chat" {
			e = x
		}
	}
	if e == nil {
		t.Fatal("work/chat missing")
	}
	e.SetTags([]string{" alpha ", "", "beta", "   "})
	tags := e.Tags()
	if len(tags) != 2 || tags[0] != "alpha" || tags[1] != "beta" {
		t.Errorf("tags = %v", tags)
	}
}

func TestAllTags_Counts(t *testing.T) {
	d := seedDB(t)
	counts := d.AllTags()
	// work/email is seeded with "personal;hot".
	if counts["personal"] != 1 || counts["hot"] != 1 {
		t.Errorf("counts = %v", counts)
	}
}

func TestAllTags_NoEntries(t *testing.T) {
	d := &DB{}
	if got := d.AllTags(); len(got) != 0 {
		t.Errorf("empty DB tags = %v", got)
	}
}

func TestEntryTags_EmptyAndOnly(t *testing.T) {
	d := seedDB(t)
	var chat *Entry
	for _, e := range d.SortedEntries() {
		if e.DisplayPath() == "work/chat" {
			chat = e
		}
	}
	if got := chat.Tags(); got != nil {
		t.Errorf("untagged entry tags = %v", got)
	}
}
