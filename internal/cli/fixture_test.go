package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tobischo/gokeepasslib/v3"
	w "github.com/tobischo/gokeepasslib/v3/wrappers"
)

type fixture struct {
	t                  *testing.T
	root               string
	runtimeDir         string
	dbPath             string
	passwordFile       string
	workDBPath         string
	workPasswordFile   string
	sourceDBPath       string
	sourcePasswordFile string
	configEnv          string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	root := t.TempDir()
	f := &fixture{
		t:                  t,
		root:               root,
		runtimeDir:         filepath.Join(root, "runtime"),
		dbPath:             filepath.Join(root, "test.kdbx"),
		passwordFile:       filepath.Join(root, "db.password"),
		workDBPath:         filepath.Join(root, "work.kdbx"),
		workPasswordFile:   filepath.Join(root, "work.password"),
		sourceDBPath:       filepath.Join(root, "source.kdbx"),
		sourcePasswordFile: filepath.Join(root, "source.password"),
		configEnv:          filepath.Join(root, "missing-kpass.toml"),
	}
	if err := os.Mkdir(f.runtimeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, f.passwordFile, "master-password\n")
	writeFile(t, f.workPasswordFile, "work-password\n")
	writeFile(t, f.sourcePasswordFile, "source-password\n")
	seedMainDB(t, f.dbPath)
	seedWorkDB(t, f.workDBPath)
	seedSourceDB(t, f.sourceDBPath)

	t.Setenv("XDG_RUNTIME_DIR", f.runtimeDir)
	t.Setenv("KPASS_CONFIG", f.configEnv)
	// Reset hooks that previous tests may have set.
	t.Cleanup(func() {
		ClipboardWriter = nil
		OtpCoder = nil
		PassDecryptorHook = nil
	})
	return f
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

// runCLI mirrors the Python run_cli helper. By default it prepends
// --database <main> and --password-file <main> so commands open the seeded DB
// without prompting. Returns (stdout, stderr, exitcode).
type runOpts struct {
	stdin            string
	skipDatabaseArg  bool
	skipPasswordFile bool
	configFile       string
	databasePath     string
	passwordFilePath string
}

func (f *fixture) runCLI(args ...string) (string, string, int) {
	return f.runCLIWith(runOpts{}, args...)
}

func (f *fixture) runCLIWith(opts runOpts, args ...string) (string, string, int) {
	f.t.Helper()
	argv := append([]string{}, args...)
	if !opts.skipDatabaseArg {
		dbPath := opts.databasePath
		if dbPath == "" {
			dbPath = f.dbPath
		}
		argv = append([]string{"--database", dbPath}, argv...)
	}
	if opts.configFile != "" {
		argv = append([]string{"--config", opts.configFile}, argv...)
	}
	if !opts.skipPasswordFile {
		insertAt := 0
		if !opts.skipDatabaseArg {
			insertAt = 2
		}
		if opts.configFile != "" {
			insertAt += 2
		}
		pf := opts.passwordFilePath
		if pf == "" {
			pf = f.passwordFile
		}
		argv = insertAt2(argv, insertAt, []string{"--password-file", pf})
	}
	var stdoutBuf, stderrBuf bytes.Buffer
	stdin := strings.NewReader(opts.stdin)
	code := Run(argv, stdin, &stdoutBuf, &stderrBuf)
	return stdoutBuf.String(), stderrBuf.String(), code
}

func insertAt2(slice []string, at int, items []string) []string {
	out := make([]string, 0, len(slice)+len(items))
	out = append(out, slice[:at]...)
	out = append(out, items...)
	out = append(out, slice[at:]...)
	return out
}

// --- KDBX seeding helpers ----------------------------------------------------

func seedMainDB(t *testing.T, path string) {
	t.Helper()
	db, root := newDB(t, "master-password")
	// Pre-allocate all subgroups so pointers stay stable while we add entries.
	addGroup(root, "internet")
	addGroup(root, "work")
	addGroup(root, "otp")
	addGroup(root, "db-passwords")
	internet := &root.Groups[0]
	work := &root.Groups[1]
	otp := &root.Groups[2]
	pwStore := &root.Groups[3]
	addEntry(internet, "email", entryFields{User: "alice", Password: "pw-email", URL: "https://mail.example.com", Notes: "personal"})
	addEntry(work, "email", entryFields{User: "worker", Password: "pw-work", URL: "https://work.example.com", Notes: "corp"})
	addEntry(root, "simple", entryFields{User: "rootuser", Password: "pw-root"})
	addEntry(otp, "sample", entryFields{User: "otp-user", Password: "pw-otp", Otp: "otpauth://totp/Test?secret=GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ&issuer=Demo&digits=8"})
	addEntry(pwStore, "work", entryFields{User: "db-work", Password: "work-password", Notes: "password for work db"})
	writeDB(t, path, db)
}

func seedWorkDB(t *testing.T, path string) {
	t.Helper()
	db, root := newDB(t, "work-password")
	addGroup(root, "shared")
	shared := &root.Groups[0]
	addEntry(shared, "chat", entryFields{User: "work-user", Password: "pw-work-chat", Notes: "work-db"})
	addEntry(root, "work-only", entryFields{User: "work-root", Password: "pw-work-root"})
	writeDB(t, path, db)
}

func seedSourceDB(t *testing.T, path string) {
	t.Helper()
	db, root := newDB(t, "source-password")
	addGroup(root, "internet")
	addGroup(root, "shared")
	internet := &root.Groups[0]
	shared := &root.Groups[1]
	e := addEntry(internet, "email", entryFields{User: "merged-user", Password: "merged-pass", URL: "https://merged.example.com", Notes: "merged-note", Tags: []string{"merged", "email"}})
	addCustomProperty(e, "env", "merged")
	binary := db.AddBinary([]byte("merged-attachment"))
	e.Binaries = append(e.Binaries, binary.CreateReference("merged.txt"))
	addEntry(shared, "chat", entryFields{User: "chat-user", Password: "chat-pass", Notes: "new-entry"})
	writeDB(t, path, db)
}

type entryFields struct {
	User, Password, URL, Notes, Otp string
	Tags                            []string
}

func newDB(t *testing.T, password string) (*gokeepasslib.Database, *gokeepasslib.Group) {
	t.Helper()
	db := gokeepasslib.NewDatabase(gokeepasslib.WithDatabaseKDBXVersion4())
	db.Credentials = gokeepasslib.NewPasswordCredentials(password)
	root := gokeepasslib.NewGroup()
	root.Name = "Root"
	db.Content.Root.Groups = []gokeepasslib.Group{root}
	return db, &db.Content.Root.Groups[0]
}

func addGroup(parent *gokeepasslib.Group, name string) *gokeepasslib.Group {
	g := gokeepasslib.NewGroup()
	g.Name = name
	parent.Groups = append(parent.Groups, g)
	return &parent.Groups[len(parent.Groups)-1]
}

func addEntry(parent *gokeepasslib.Group, title string, f entryFields) *gokeepasslib.Entry {
	e := gokeepasslib.NewEntry()
	e.Values = append(e.Values, kv("Title", title, false))
	e.Values = append(e.Values, kv("UserName", f.User, false))
	e.Values = append(e.Values, kv("Password", f.Password, true))
	if f.URL != "" {
		e.Values = append(e.Values, kv("URL", f.URL, false))
	}
	if f.Notes != "" {
		e.Values = append(e.Values, kv("Notes", f.Notes, false))
	}
	if f.Otp != "" {
		e.Values = append(e.Values, kv("otp", f.Otp, true))
	}
	if len(f.Tags) > 0 {
		e.Tags = strings.Join(f.Tags, ";")
	}
	parent.Entries = append(parent.Entries, e)
	return &parent.Entries[len(parent.Entries)-1]
}

func addCustomProperty(e *gokeepasslib.Entry, key, value string) {
	e.Values = append(e.Values, gokeepasslib.ValueData{
		Key:   key,
		Value: gokeepasslib.V{Content: value},
	})
}

func kv(key, value string, protected bool) gokeepasslib.ValueData {
	v := gokeepasslib.ValueData{Key: key, Value: gokeepasslib.V{Content: value}}
	if protected {
		v.Value.Protected = w.NewBoolWrapper(true)
	}
	return v
}

func writeDB(t *testing.T, path string, db *gokeepasslib.Database) {
	t.Helper()
	if err := db.LockProtectedEntries(); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := gokeepasslib.NewEncoder(f).Encode(db); err != nil {
		t.Fatal(err)
	}
}

// openSeededDB opens a fixture KDBX file in read-only mode so a test can
// assert on entry contents written by a CLI run.
func openSeededDB(t *testing.T, path, password string) *gokeepasslib.Database {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	db := gokeepasslib.NewDatabase()
	db.Credentials = gokeepasslib.NewPasswordCredentials(password)
	if err := gokeepasslib.NewDecoder(f).Decode(db); err != nil {
		t.Fatal(err)
	}
	if err := db.UnlockProtectedEntries(); err != nil {
		t.Fatal(err)
	}
	return db
}

func _unused(_ io.Reader) {}
