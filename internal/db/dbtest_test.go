package db

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tobischo/gokeepasslib/v3"
	w "github.com/tobischo/gokeepasslib/v3/wrappers"
)

// seedEntry adds a fully-formed entry under g and returns a pointer to it.
func seedEntry(g *gokeepasslib.Group, title, user, pass, url, notes, otp string) *gokeepasslib.Entry {
	e := gokeepasslib.NewEntry()
	e.Values = []gokeepasslib.ValueData{
		{Key: "Title", Value: gokeepasslib.V{Content: title}},
		{Key: "UserName", Value: gokeepasslib.V{Content: user}},
		{Key: "Password", Value: gokeepasslib.V{Content: pass, Protected: w.NewBoolWrapper(true)}},
	}
	if url != "" {
		e.Values = append(e.Values, gokeepasslib.ValueData{Key: "URL", Value: gokeepasslib.V{Content: url}})
	}
	if notes != "" {
		e.Values = append(e.Values, gokeepasslib.ValueData{Key: "Notes", Value: gokeepasslib.V{Content: notes}})
	}
	if otp != "" {
		e.Values = append(e.Values, gokeepasslib.ValueData{Key: "otp", Value: gokeepasslib.V{Content: otp, Protected: w.NewBoolWrapper(true)}})
	}
	g.Entries = append(g.Entries, e)
	return &g.Entries[len(g.Entries)-1]
}

// seedDB builds an in-memory DB with a known tree:
//
//	Root
//	├── work/
//	│   ├── email          (with attachment "doc.txt", custom "Pin"=1234, otp)
//	│   └── chat
//	├── personal/
//	│   └── bank
//	├── github            (root-level entry)
//	└── empty/            (no entries — for EmptyGroups tests)
//	    └── nested/
//
// Returns a *DB whose Path is unset (callers that need to save it must set it).
func seedDB(t *testing.T) *DB {
	t.Helper()
	raw := gokeepasslib.NewDatabase(gokeepasslib.WithDatabaseKDBXVersion4())
	raw.Credentials = gokeepasslib.NewPasswordCredentials("secret")
	root := &raw.Content.Root.Groups[0]
	root.Name = "Root"
	// gokeepasslib seeds a "Sample Entry" by default — wipe it.
	root.Entries = nil
	root.Groups = nil

	work := gokeepasslib.NewGroup()
	work.Name = "work"
	email := seedEntry(&work, "email", "alice@work", "work-pass", "https://mail.example.com", "notes-here", "otpauth://totp/Work:alice?secret=JBSWY3DPEHPK3PXP&issuer=Work")
	email.Values = append(email.Values, gokeepasslib.ValueData{Key: "Pin", Value: gokeepasslib.V{Content: "1234"}})
	email.Tags = "personal;hot"
	// Attach a binary
	bin := raw.AddBinary([]byte("ATTACHMENT-BODY"))
	email.Binaries = append(email.Binaries, bin.CreateReference("doc.txt"))

	seedEntry(&work, "chat", "alice", "chat-pass", "", "", "")

	personal := gokeepasslib.NewGroup()
	personal.Name = "personal"
	seedEntry(&personal, "bank", "alice", "bank-pass", "https://bank.example.com", "", "")

	empty := gokeepasslib.NewGroup()
	empty.Name = "empty"
	nested := gokeepasslib.NewGroup()
	nested.Name = "nested"
	empty.Groups = append(empty.Groups, nested)

	seedEntry(root, "github", "alice", "gh-pass", "https://github.com", "", "")

	root.Groups = append(root.Groups, work, personal, empty)

	return &DB{Raw: raw}
}

// writeKdbx persists d.Raw to a tempfile with given password and returns the
// path. Re-loadable via Open or OpenSimple.
func writeKdbx(t *testing.T, d *DB, password string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.kdbx")
	d.Path = path
	d.Raw.Credentials = gokeepasslib.NewPasswordCredentials(password)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := gokeepasslib.NewEncoder(f).Encode(d.Raw); err != nil {
		t.Fatal(err)
	}
	return path
}
