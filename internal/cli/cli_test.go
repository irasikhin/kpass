package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

// --- Helper: config-file generators (port of write_*_config in Python tests).

func writeMultiDBConfig(t *testing.T, f *fixture, opts multiDBConfigOpts) string {
	t.Helper()
	cfg := filepath.Join(f.root, "kpass.toml")
	var b strings.Builder
	if !opts.skipDefault {
		b.WriteString(`default = "main"` + "\n\n")
	}
	b.WriteString("[databases.main]\n")
	b.WriteString(`database = "` + f.dbPath + `"` + "\n")
	if !opts.skipMainPasswordFile {
		b.WriteString(`password_file = "` + f.passwordFile + `"` + "\n")
	}
	b.WriteString("\n[databases.work]\n")
	b.WriteString(`database = "` + f.workDBPath + `"` + "\n")
	if !opts.skipWorkPasswordFile {
		b.WriteString(`password_file = "` + f.workPasswordFile + `"` + "\n")
	}
	b.WriteString("\n")
	writeFile(t, cfg, b.String())
	return cfg
}

type multiDBConfigOpts struct {
	skipDefault          bool
	skipMainPasswordFile bool
	skipWorkPasswordFile bool
}

func writePasswordLookupConfig(t *testing.T, f *fixture) string {
	t.Helper()
	cfg := filepath.Join(f.root, "kpass-password-lookup.toml")
	writeFile(t, cfg, strings.Join([]string{
		`default = "main"`,
		"",
		"[databases.main]",
		`database = "` + f.dbPath + `"`,
		"",
		"[databases.work]",
		`database = "` + f.workDBPath + `"`,
		`password_database = "main"`,
		`password_entry = "db-passwords/work"`,
		"",
	}, "\n"))
	return cfg
}

func writePasswordLookupCycleConfig(t *testing.T, f *fixture) string {
	t.Helper()
	cfg := filepath.Join(f.root, "kpass-password-cycle.toml")
	writeFile(t, cfg, strings.Join([]string{
		`default = "main"`,
		"",
		"[databases.main]",
		`database = "` + f.dbPath + `"`,
		`password_database = "work"`,
		`password_entry = "db-passwords/work"`,
		"",
		"[databases.work]",
		`database = "` + f.workDBPath + `"`,
		`password_database = "main"`,
		`password_entry = "db-passwords/work"`,
		"",
	}, "\n"))
	return cfg
}

// --- Tests --------------------------------------------------------------------

func TestLsListsEntryPaths(t *testing.T) {
	f := newFixture(t)
	stdout, stderr, code := f.runCLI("ls")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	want := strings.Join([]string{
		"Password Store",
		"├── db-passwords (1)",
		"│   └── work 🔑📝",
		"├── internet (1)",
		"│   └── email 🔑🔗📝",
		"├── otp (1)",
		"│   └── sample 🔑⏱",
		"├── simple 🔑",
		"└── work (1)",
		"    └── email 🔑🔗📝",
	}, "\n")
	if strings.TrimSpace(stdout) != want {
		t.Fatalf("stdout mismatch:\n--got--\n%s\n--want--\n%s", stdout, want)
	}
}

func TestLsFlatKeepsPlainPaths(t *testing.T) {
	f := newFixture(t)
	stdout, stderr, code := f.runCLI("ls", "--flat")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	want := []string{"db-passwords/work", "internet/email", "otp/sample", "simple", "work/email"}
	got := strings.Split(strings.TrimSpace(stdout), "\n")
	if !sliceEqual(got, want) {
		t.Fatalf("got=%v want=%v", got, want)
	}
}

func TestGetFieldReturnsPassword(t *testing.T) {
	f := newFixture(t)
	stdout, stderr, code := f.runCLI("get", "internet/email", "--field", "password")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if strings.TrimSpace(stdout) != "pw-email" {
		t.Fatalf("got %q want %q", stdout, "pw-email")
	}
}

func TestSearchLists(t *testing.T) {
	f := newFixture(t)
	stdout, stderr, code := f.runCLI("search", "email")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	want := strings.Join([]string{
		"Search: email",
		"├── internet (1)",
		"│   └── email",
		"└── work (1)",
		"    └── email",
	}, "\n")
	if strings.TrimSpace(stdout) != want {
		t.Fatalf("stdout mismatch:\n--got--\n%s\n--want--\n%s", stdout, want)
	}
}

func TestSearchCaseInsensitive(t *testing.T) {
	f := newFixture(t)
	stdout, _, code := f.runCLI("search", "MAIL", "--flat")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	got := strings.Split(strings.TrimSpace(stdout), "\n")
	want := []string{"internet/email", "work/email"}
	if !sliceEqual(got, want) {
		t.Fatalf("got=%v want=%v", got, want)
	}
}

func TestSearchCanSearchContents(t *testing.T) {
	f := newFixture(t)
	stdout, _, code := f.runCLI("search", "personal", "--flat", "--field", "notes")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	got := strings.Split(strings.TrimSpace(stdout), "\n")
	if !sliceEqual(got, []string{"internet/email"}) {
		t.Fatalf("got=%v", got)
	}

	stdout, _, code = f.runCLI("search", "PW-WORK", "--flat", "--field", "password")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	got = strings.Split(strings.TrimSpace(stdout), "\n")
	if !sliceEqual(got, []string{"work/email"}) {
		t.Fatalf("got=%v", got)
	}
}

func TestAmbiguousMatch(t *testing.T) {
	f := newFixture(t)
	stdout, stderr, code := f.runCLI("get", "email", "--field", "password")
	if code != 1 {
		t.Fatalf("code=%d", code)
	}
	if stdout != "" {
		t.Fatalf("stdout=%q", stdout)
	}
	if !strings.Contains(stderr, "Ambiguous entry 'email'") {
		t.Fatalf("stderr=%q", stderr)
	}
}

func TestCopyUsesClipboardHelper(t *testing.T) {
	f := newFixture(t)
	var captured struct {
		value   string
		timeout int
	}
	ClipboardWriter = func(v string, to int) error {
		captured.value = v
		captured.timeout = to
		return nil
	}
	stdout, stderr, code := f.runCLI("copy", "internet/email", "0")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if captured.value != "pw-email" || captured.timeout != 0 {
		t.Fatalf("captured=%+v", captured)
	}
	if !strings.Contains(stdout, "Copied password for internet/email to clipboard") {
		t.Fatalf("stdout=%q", stdout)
	}
}

func TestConfigFileSetsDefaultDatabase(t *testing.T) {
	f := newFixture(t)
	cfg := writeMultiDBConfig(t, f, multiDBConfigOpts{})
	stdout, stderr, code := f.runCLIWith(runOpts{
		skipDatabaseArg:  true,
		skipPasswordFile: true,
		configFile:       cfg,
	}, "get", "simple", "--field", "password")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if strings.TrimSpace(stdout) != "pw-root" {
		t.Fatalf("got %q", stdout)
	}
}

func TestDatabaseSelectorUsesNamedDatabase(t *testing.T) {
	f := newFixture(t)
	cfg := writeMultiDBConfig(t, f, multiDBConfigOpts{})
	stdout, stderr, code := f.runCLIWith(runOpts{
		skipDatabaseArg:  true,
		skipPasswordFile: true,
		configFile:       cfg,
	}, "get", "@work", "work-only", "--field", "password")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if strings.TrimSpace(stdout) != "pw-work-root" {
		t.Fatalf("got %q", stdout)
	}
}

func TestUnknownDatabaseSelector(t *testing.T) {
	f := newFixture(t)
	cfg := writeMultiDBConfig(t, f, multiDBConfigOpts{})
	_, stderr, code := f.runCLIWith(runOpts{
		skipDatabaseArg:  true,
		skipPasswordFile: true,
		configFile:       cfg,
	}, "get", "@missing", "simple", "--field", "password")
	if code != 1 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(stderr, "Unknown database profile: missing") {
		t.Fatalf("stderr=%q", stderr)
	}
}

func TestSelectorConflictsWithDatabaseFlag(t *testing.T) {
	f := newFixture(t)
	cfg := writeMultiDBConfig(t, f, multiDBConfigOpts{})
	_, stderr, code := f.runCLIWith(runOpts{
		skipPasswordFile: true,
		configFile:       cfg,
	}, "get", "@work", "work-only", "--field", "password")
	if code != 1 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(stderr, "cannot combine @db with --database.") {
		t.Fatalf("stderr=%q", stderr)
	}
}

func TestConfigRequiresDefault(t *testing.T) {
	f := newFixture(t)
	cfg := writeMultiDBConfig(t, f, multiDBConfigOpts{skipDefault: true})
	_, stderr, code := f.runCLIWith(runOpts{
		skipDatabaseArg:  true,
		skipPasswordFile: true,
		configFile:       cfg,
	}, "get", "simple", "--field", "password")
	if code != 1 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(stderr, "KPass config must define a non-empty top-level 'default'") {
		t.Fatalf("stderr=%q", stderr)
	}
}

func TestPasswordLookupCycle(t *testing.T) {
	f := newFixture(t)
	cfg := writePasswordLookupCycleConfig(t, f)
	_, stderr, code := f.runCLIWith(runOpts{
		skipDatabaseArg:  true,
		skipPasswordFile: true,
		configFile:       cfg,
	}, "get", "@work", "work-only", "--field", "password")
	if code != 1 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(stderr, "Database password resolution loop detected") {
		t.Fatalf("stderr=%q", stderr)
	}
}

func TestPasswordSessionReusesPrompted(t *testing.T) {
	f := newFixture(t)
	callCount := 0
	if !setPromptHook(t, "master-password", &callCount) {
		t.Fatal("could not set prompt hook")
	}
	stdout, stderr, code := f.runCLIWith(runOpts{
		skipPasswordFile: true,
	}, "get", "simple", "--field", "password")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if strings.TrimSpace(stdout) != "pw-root" {
		t.Fatalf("got %q", stdout)
	}
	if callCount != 1 {
		t.Fatalf("prompt called %d times", callCount)
	}

	// Second run: cached password must avoid the prompt.
	setPromptHookFail(t)
	stdout, _, code = f.runCLIWith(runOpts{
		skipPasswordFile: true,
	}, "get", "simple", "--field", "password")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	if strings.TrimSpace(stdout) != "pw-root" {
		t.Fatalf("got %q", stdout)
	}
}

func TestDoctorReportsHealthy(t *testing.T) {
	f := newFixture(t)
	cfg := writeMultiDBConfig(t, f, multiDBConfigOpts{})
	stdout, stderr, code := f.runCLIWith(runOpts{
		skipDatabaseArg:  true,
		skipPasswordFile: true,
		configFile:       cfg,
	}, "doctor")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "OK main:") {
		t.Fatalf("stdout=%q", stdout)
	}
	if !strings.Contains(stdout, "OK work:") {
		t.Fatalf("stdout=%q", stdout)
	}
	if !strings.Contains(stdout, "Doctor found no issues") {
		t.Fatalf("stdout=%q", stdout)
	}
}

func TestDoctorReportsBrokenCycle(t *testing.T) {
	f := newFixture(t)
	cfg := writePasswordLookupCycleConfig(t, f)
	stdout, _, code := f.runCLIWith(runOpts{
		skipDatabaseArg:  true,
		skipPasswordFile: true,
		configFile:       cfg,
	}, "doctor")
	if code != 1 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(stdout, "ERROR main:") {
		t.Fatalf("stdout=%q", stdout)
	}
	if !strings.Contains(stdout, "Database password resolution loop detected") {
		t.Fatalf("stdout=%q", stdout)
	}
}

// Removed-command migration hints --------------------------------------------
// rm/mv/cp are now live aliases (remove/move/clone), so they no longer appear
// here. show/pass/clip/otp/grep/open/close stay as removed-with-hint stubs.

func TestRemovedHints(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"show", "simple"}, "show was removed; use: kpass get"},
		{[]string{"pass", "simple"}, "pass was removed; use: kpass get"},
		{[]string{"clip", "simple"}, "clip was removed; use: kpass copy"},
		{[]string{"otp", "otp/sample"}, "otp was removed; use: kpass get"},
		{[]string{"grep", "personal"}, "grep was removed; use: kpass search"},
		{[]string{"close"}, "close was removed; session handling is automatic."},
	}
	for _, tc := range cases {
		t.Run(tc.args[0], func(t *testing.T) {
			f := newFixture(t)
			stdout, stderr, code := f.runCLI(tc.args...)
			if code != 1 {
				t.Fatalf("code=%d", code)
			}
			if stdout != "" {
				t.Fatalf("stdout=%q", stdout)
			}
			if !strings.Contains(stderr, tc.want) {
				t.Fatalf("stderr=%q want %q", stderr, tc.want)
			}
		})
	}
}

func TestMissingRequiredArgument(t *testing.T) {
	f := newFixture(t)
	stdout, stderr, code := f.runCLI("get")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr=%q", stderr)
	}
	// Missing required argument now shows the command's --help output on stdout.
	if !strings.Contains(stdout, "Usage:") || !strings.Contains(stdout, "<entry>") {
		t.Fatalf("stdout=%q", stdout)
	}
}

func TestUnknownCommand(t *testing.T) {
	f := newFixture(t)
	stdout, stderr, code := f.runCLI("nosuch")
	if code != 1 {
		t.Fatalf("code=%d", code)
	}
	if stdout != "" {
		t.Fatalf("stdout=%q", stdout)
	}
	if !strings.Contains(stderr, "kpass: argument command: invalid choice: 'nosuch'") {
		t.Fatalf("stderr=%q", stderr)
	}
	if !strings.Contains(stderr, "Use 'kpass --help' for usage.") {
		t.Fatalf("stderr=%q", stderr)
	}
}

func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
