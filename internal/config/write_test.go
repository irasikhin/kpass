package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteAtomic_MissingDefault(t *testing.T) {
	dir := t.TempDir()
	err := WriteAtomic(filepath.Join(dir, "config.toml"), FileConfig{
		Databases: map[string]Profile{"main": {Database: "/x.kdbx"}},
	})
	if err == nil || !strings.Contains(err.Error(), "default database") {
		t.Errorf("expected missing-default error, got %v", err)
	}
}

func TestWriteAtomic_MissingDatabases(t *testing.T) {
	dir := t.TempDir()
	err := WriteAtomic(filepath.Join(dir, "config.toml"), FileConfig{
		DefaultDatabase: "main",
	})
	if err == nil || !strings.Contains(err.Error(), "[databases.") {
		t.Errorf("expected missing-databases error, got %v", err)
	}
}

func TestWriteAtomic_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "config.toml")
	ttl := 600
	yes := true
	no := false
	fc := FileConfig{
		DefaultDatabase: "main",
		Databases: map[string]Profile{
			"main": {
				Database:         "/vault/main.kdbx",
				PasswordFile:     "/vault/main.pw",
				KeyFile:          "/vault/main.key",
				CacheTTL:         &ttl,
				NoCache:          &yes,
				BackupKeep:       5,
				BackupMaxAgeDays: 30,
			},
			"chained": {
				Database:         "/vault/work.kdbx",
				PasswordDatabase: "main",
				PasswordEntry:    "vaults/work",
				NoCache:          &no,
			},
		},
	}

	if err := WriteAtomic(path, fc); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("file mode = %#o, want 0600", info.Mode().Perm())
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if dirInfo.Mode().Perm() != 0o700 {
		t.Errorf("dir mode = %#o, want 0700", dirInfo.Mode().Perm())
	}

	loaded, _, err := Load(path)
	if err != nil {
		t.Fatalf("re-Load: %v", err)
	}
	if loaded.DefaultDatabase != "main" {
		t.Errorf("default = %q", loaded.DefaultDatabase)
	}
	main := loaded.Databases["main"]
	if main.Database != "/vault/main.kdbx" || main.PasswordFile != "/vault/main.pw" || main.KeyFile != "/vault/main.key" {
		t.Errorf("main profile fields mismatch: %+v", main)
	}
	if main.CacheTTL == nil || *main.CacheTTL != 600 {
		t.Errorf("cache_ttl roundtrip: %v", main.CacheTTL)
	}
	if main.NoCache == nil || !*main.NoCache {
		t.Errorf("no_cache roundtrip: %v", main.NoCache)
	}
	if main.BackupKeep != 5 || main.BackupMaxAgeDays != 30 {
		t.Errorf("backup fields: keep=%d, age=%d", main.BackupKeep, main.BackupMaxAgeDays)
	}

	chained := loaded.Databases["chained"]
	if chained.PasswordDatabase != "main" || chained.PasswordEntry != "vaults/work" {
		t.Errorf("chained password fields: %+v", chained)
	}
	if chained.NoCache == nil || *chained.NoCache {
		t.Errorf("chained no_cache should be false-pointer, got %v", chained.NoCache)
	}
}

func TestDump_SortedDeterministic(t *testing.T) {
	fc := FileConfig{
		DefaultDatabase: "alpha",
		Databases: map[string]Profile{
			"zulu":  {Database: "/z"},
			"alpha": {Database: "/a"},
			"mike":  {Database: "/m"},
		},
	}
	a := Dump(fc)
	b := Dump(fc)
	if a != b {
		t.Errorf("Dump not deterministic:\n--- a ---\n%s--- b ---\n%s", a, b)
	}
	if !strings.Contains(a, "default = \"alpha\"") {
		t.Errorf("missing default line:\n%s", a)
	}
	iA := strings.Index(a, "[databases.alpha]")
	iM := strings.Index(a, "[databases.mike]")
	iZ := strings.Index(a, "[databases.zulu]")
	if iA < 0 || iM < iA || iZ < iM {
		t.Errorf("profiles not in sorted order:\n%s", a)
	}
}

func TestDump_OmitsZeroFields(t *testing.T) {
	fc := FileConfig{
		DefaultDatabase: "main",
		Databases: map[string]Profile{
			"main": {Database: "/x.kdbx"},
		},
	}
	out := Dump(fc)
	for _, k := range []string{"password_file", "password_database", "password_entry", "key_file", "session_ttl", "no_session", "backup_keep", "backup_max_age_days"} {
		if strings.Contains(out, k) {
			t.Errorf("zero-valued %q should be omitted:\n%s", k, out)
		}
	}
}

func TestTomlKey_QuoteRules(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"main", "main"},
		{"work_db", "work_db"},
		{"work-db", "work-db"},
		{"db1", "db1"},
		{"", `""`},
		{"1leading-digit", `"1leading-digit"`},
		{"has space", `"has space"`},
		{"with.dot", `"with.dot"`},
		{"ютф", `"ютф"`},
	}
	for _, tc := range cases {
		if got := tomlKey(tc.in); got != tc.want {
			t.Errorf("tomlKey(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestTomlQuote_EscapesSpecials(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"plain", `"plain"`},
		{`has"quote`, `"has\"quote"`},
		{"line\nbreak", `"line\nbreak"`},
		{`back\slash`, `"back\\slash"`},
	}
	for _, tc := range cases {
		if got := tomlQuote(tc.in); got != tc.want {
			t.Errorf("tomlQuote(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestIsAlphaAlphaNum(t *testing.T) {
	for _, c := range []byte("aZmK") {
		if !isAlpha(c) {
			t.Errorf("isAlpha(%q) = false, want true", c)
		}
		if !isAlphaNum(c) {
			t.Errorf("isAlphaNum(%q) = false, want true", c)
		}
	}
	for _, c := range []byte("0159") {
		if isAlpha(c) {
			t.Errorf("isAlpha(%q) = true, want false", c)
		}
		if !isAlphaNum(c) {
			t.Errorf("isAlphaNum(%q) = false, want true", c)
		}
	}
	for _, c := range []byte("-_.@! ") {
		if isAlpha(c) {
			t.Errorf("isAlpha(%q) = true, want false", c)
		}
		if isAlphaNum(c) {
			t.Errorf("isAlphaNum(%q) = true, want false", c)
		}
	}
}
