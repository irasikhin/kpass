package db

import (
	"strings"
	"testing"

	"github.com/tobischo/gokeepasslib/v3"
)

// makeSource builds a small DB suitable for merging into the main fixture.
func makeSource(t *testing.T) *DB {
	t.Helper()
	raw := gokeepasslib.NewDatabase(gokeepasslib.WithDatabaseKDBXVersion4())
	root := &raw.Content.Root.Groups[0]
	root.Name = "Root"
	root.Entries = nil
	root.Groups = nil

	work := gokeepasslib.NewGroup()
	work.Name = "work"
	seedEntry(&work, "email", "imported-user", "imported-pass", "https://imported", "imported-notes", "")
	seedEntry(&work, "new-only", "u", "p", "", "", "")
	bin := raw.AddBinary([]byte("src-attach"))
	work.Entries[1].Binaries = append(work.Entries[1].Binaries, bin.CreateReference("payload.bin"))
	root.Groups = append(root.Groups, work)
	return &DB{Raw: raw}
}

func TestMerge_ImportNew(t *testing.T) {
	dst := seedDB(t)
	src := makeSource(t)
	stats, err := dst.Merge(src, MergeOpts{OnConflict: ConflictSkip})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Imported != 1 {
		t.Errorf("imported = %d, want 1 (new-only)", stats.Imported)
	}
	if stats.Skipped != 1 {
		t.Errorf("skipped = %d, want 1 (work/email conflict)", stats.Skipped)
	}
	if dst.FindEntryByExactPath("work/new-only") == nil {
		t.Error("new-only not imported")
	}
}

func TestMerge_ConflictError(t *testing.T) {
	dst := seedDB(t)
	src := makeSource(t)
	_, err := dst.Merge(src, MergeOpts{OnConflict: ConflictError})
	if err == nil || !strings.Contains(err.Error(), "merge conflict") {
		t.Errorf("expected merge conflict error, got %v", err)
	}
}

func TestMerge_ConflictOverwrite(t *testing.T) {
	dst := seedDB(t)
	src := makeSource(t)
	stats, err := dst.Merge(src, MergeOpts{OnConflict: ConflictOverwrite})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Overwritten != 1 {
		t.Errorf("overwritten = %d, want 1", stats.Overwritten)
	}
	e := dst.FindEntryByExactPath("work/email")
	if e == nil {
		t.Fatal("work/email missing after overwrite")
	}
	if e.Raw().GetPassword() != "imported-pass" {
		t.Errorf("overwrite did not update password, got %q", e.Raw().GetPassword())
	}
}

func TestMerge_ConflictRename(t *testing.T) {
	dst := seedDB(t)
	src := makeSource(t)
	stats, err := dst.Merge(src, MergeOpts{OnConflict: ConflictRename, RenameSuffix: "imported"})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Renamed != 1 {
		t.Errorf("renamed = %d, want 1", stats.Renamed)
	}
	if dst.FindEntryByExactPath("work/email") == nil {
		t.Error("original work/email lost")
	}
	if dst.FindEntryByExactPath("work/email (imported)") == nil {
		t.Error("renamed copy not created")
	}
}

func TestUniqueEntryPath_NoCollision(t *testing.T) {
	d := seedDB(t)
	got := d.UniqueEntryPath("work", "brand-new", "")
	if got != "work/brand-new" {
		t.Errorf("path = %q", got)
	}
}

func TestUniqueEntryPath_WithSuffix(t *testing.T) {
	d := seedDB(t)
	// work/email already exists; suffix path = "work/email (imported)"
	got := d.UniqueEntryPath("work", "email", "imported")
	if got != "work/email (imported)" {
		t.Errorf("path = %q", got)
	}
}

func TestUniqueEntryPath_WithSuffixCollision(t *testing.T) {
	d := seedDB(t)
	// Pre-seed "work/email (imported)" so we need a numeric suffix.
	work := d.EnsureGroup("work")
	d.CreateEntry(work, "email (imported)", "u", "p", "", "", "")
	got := d.UniqueEntryPath("work", "email", "imported")
	if got != "work/email (imported 2)" {
		t.Errorf("path = %q", got)
	}
}

func TestUniqueEntryPath_NumericFallback(t *testing.T) {
	d := seedDB(t)
	work := d.EnsureGroup("work")
	d.CreateEntry(work, "email (2)", "u", "p", "", "", "")
	got := d.UniqueEntryPath("work", "email", "")
	if got != "work/email (3)" {
		t.Errorf("path = %q", got)
	}
}

func TestUniqueEntryPath_RootLevel(t *testing.T) {
	d := seedDB(t)
	// "github" exists at root.
	got := d.UniqueEntryPath("", "github", "")
	if got != "github (2)" {
		t.Errorf("path = %q", got)
	}
}

func TestEntryPathExists(t *testing.T) {
	d := seedDB(t)
	if !d.entryPathExists("work/email") {
		t.Error("work/email should exist")
	}
	if d.entryPathExists("ghost") {
		t.Error("ghost should not exist")
	}
}

func TestMerge_PreservesAttachments(t *testing.T) {
	dst := seedDB(t)
	src := makeSource(t)
	if _, err := dst.Merge(src, MergeOpts{OnConflict: ConflictSkip}); err != nil {
		t.Fatal(err)
	}
	imported := dst.FindEntryByExactPath("work/new-only")
	if imported == nil {
		t.Fatal("new-only not imported")
	}
	if !imported.AttachmentExists("payload.bin") {
		t.Errorf("attachment not carried over: %v", imported.AttachmentList())
	}
	got, err := imported.AttachmentContent("payload.bin")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "src-attach" {
		t.Errorf("attachment content = %q", got)
	}
}

func TestMerge_OverwritePreservesAttachmentsFromSource(t *testing.T) {
	dst := seedDB(t)
	src := makeSource(t)
	// Add an attachment to the source's email so overwrite carries it over.
	for i := range src.Raw.Content.Root.Groups[0].Groups[0].Entries {
		if src.Raw.Content.Root.Groups[0].Groups[0].Entries[i].GetTitle() == "email" {
			ref := src.Raw.AddBinary([]byte("overwrite-attach"))
			src.Raw.Content.Root.Groups[0].Groups[0].Entries[i].Binaries = append(
				src.Raw.Content.Root.Groups[0].Groups[0].Entries[i].Binaries,
				ref.CreateReference("ow.bin"),
			)
		}
	}
	if _, err := dst.Merge(src, MergeOpts{OnConflict: ConflictOverwrite}); err != nil {
		t.Fatal(err)
	}
	e := dst.FindEntryByExactPath("work/email")
	if e == nil {
		t.Fatal("email gone")
	}
	if !e.AttachmentExists("ow.bin") {
		t.Errorf("attachment not carried over on overwrite: %v", e.AttachmentList())
	}
	// Old doc.txt should be gone (overwrite replaces attachments).
	if e.AttachmentExists("doc.txt") {
		t.Error("old attachment should be removed on overwrite")
	}
}

func TestOpenSimple_RequiresCreds(t *testing.T) {
	if _, err := OpenSimple("ignored.kdbx", "", "", ""); err == nil {
		t.Error("expected creds-required error")
	}
}

func TestOpenSimple_RoundTrip(t *testing.T) {
	d := seedDB(t)
	path := writeKdbx(t, d, "rt-secret")
	loaded, err := OpenSimple(path, "", "", "rt-secret")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.FindEntryByExactPath("work/email") == nil {
		t.Error("entries not present after round-trip")
	}
}

func TestOpenSimple_WrongPassword(t *testing.T) {
	d := seedDB(t)
	path := writeKdbx(t, d, "right")
	if _, err := OpenSimple(path, "", "", "wrong"); err == nil {
		t.Error("expected wrong-password error")
	}
}

func TestOpenSimple_MissingFile(t *testing.T) {
	if _, err := OpenSimple("/nonexistent.kdbx", "", "", "x"); err == nil {
		t.Error("expected missing-file error")
	}
}

func TestOpenSimple_PasswordFile(t *testing.T) {
	d := seedDB(t)
	path := writeKdbx(t, d, "pwfile-secret")
	pwPath := path + ".pw"
	if err := writeStringTest(t, pwPath, "pwfile-secret\n"); err != nil {
		t.Fatal(err)
	}
	loaded, err := OpenSimple(path, pwPath, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.FindEntryByExactPath("github") == nil {
		t.Error("entries missing")
	}
}

func TestOpenSimple_PasswordFileMissing(t *testing.T) {
	if _, err := OpenSimple("/whatever.kdbx", "/no/such/pw.txt", "", ""); err == nil {
		t.Error("expected pwfile error")
	}
}
