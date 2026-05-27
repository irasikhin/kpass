package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTOML(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoad_ValidMinimal(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	writeTOML(t, cfgPath, `
default = "main"

[databases.main]
database = "~/vaults/main.kdbx"
`)

	fc, path, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if path != cfgPath {
		t.Errorf("path = %q, want %q", path, cfgPath)
	}
	if fc.DefaultDatabase != "main" {
		t.Errorf("default = %q, want main", fc.DefaultDatabase)
	}
	if len(fc.Databases) != 1 {
		t.Fatalf("databases len = %d, want 1", len(fc.Databases))
	}
	p := fc.Databases["main"]
	if p.Database == "" {
		t.Error("profile database is empty")
	}
}

func TestLoad_MultipleProfiles(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	writeTOML(t, cfgPath, `
default = "work"

[databases.work]
database = "~/vaults/work.kdbx"

[databases.personal]
database = "~/vaults/personal.kdbx"
key_file = "~/vaults/personal.key"
`)

	fc, _, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if len(fc.Databases) != 2 {
		t.Errorf("databases len = %d, want 2", len(fc.Databases))
	}
	if fc.Databases["personal"].KeyFile == "" {
		t.Error("personal key_file is empty")
	}
}

func TestLoad_LegacyDefaultDatabase(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	writeTOML(t, cfgPath, `
default_database = "main"

[databases.main]
database = "~/vaults/main.kdbx"
`)

	fc, _, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if fc.DefaultDatabase != "main" {
		t.Errorf("default = %q, want main", fc.DefaultDatabase)
	}
}

func TestLoad_DefaultDatabaseConflict(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	writeTOML(t, cfgPath, `
default = "main"
default_database = "work"

[databases.main]
database = "~/vaults/main.kdbx"

[databases.work]
database = "~/vaults/work.kdbx"
`)

	_, _, err := Load(cfgPath)
	if err == nil || !strings.Contains(err.Error(), "default_database") {
		t.Errorf("expected default/default_database conflict, got: %v", err)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	fc, _, err := Load("/nonexistent/path/config.toml")
	if err != nil {
		t.Fatalf("Load should return empty config for missing file: %v", err)
	}
	if fc.DefaultDatabase != "" {
		t.Error("expected empty default")
	}
}

func TestLoad_MissingDefault(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	writeTOML(t, cfgPath, `
[databases.main]
database = "~/vaults/main.kdbx"
`)

	_, _, err := Load(cfgPath)
	if err == nil {
		t.Error("expected error for missing default")
	}
}

func TestLoad_MissingDatabases(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	writeTOML(t, cfgPath, `default = "main"`)

	_, _, err := Load(cfgPath)
	if err == nil {
		t.Error("expected error for missing databases section")
	}
}

func TestLoad_UnknownTopLevelKey(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	writeTOML(t, cfgPath, `
default = "main"
foobar = true

[databases.main]
database = "~/vaults/main.kdbx"
`)

	_, _, err := Load(cfgPath)
	if err == nil || !strings.Contains(err.Error(), "foobar") {
		t.Errorf("expected unknown key error, got: %v", err)
	}
}

func TestLoad_DefaultNotInDatabases(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	writeTOML(t, cfgPath, `
default = "missing"

[databases.main]
database = "~/vaults/main.kdbx"
`)

	_, _, err := Load(cfgPath)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestLoad_ProfileMissingDatabase(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	writeTOML(t, cfgPath, `
default = "main"

[databases.main]
key_file = "~/vaults/main.key"
`)

	_, _, err := Load(cfgPath)
	if err == nil || !strings.Contains(err.Error(), "database") {
		t.Errorf("expected missing database error, got: %v", err)
	}
}

func TestLoad_CacheTTL(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	writeTOML(t, cfgPath, `
default = "main"

[databases.main]
database = "~/vaults/main.kdbx"
cache_ttl = 600
`)

	fc, _, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if fc.Databases["main"].CacheTTL == nil || *fc.Databases["main"].CacheTTL != 600 {
		t.Errorf("cache_ttl = %v, want 600", fc.Databases["main"].CacheTTL)
	}
}

func TestLoad_SessionTTLLegacy(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	writeTOML(t, cfgPath, `
default = "main"

[databases.main]
database = "~/vaults/main.kdbx"
session_ttl = 300
`)

	fc, _, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if fc.Databases["main"].CacheTTL == nil || *fc.Databases["main"].CacheTTL != 300 {
		t.Errorf("session_ttl = %v, want 300", fc.Databases["main"].CacheTTL)
	}
}

func TestLoad_NoCache(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	writeTOML(t, cfgPath, `
default = "main"

[databases.main]
database = "~/vaults/main.kdbx"
no_cache = true
`)

	fc, _, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	p := fc.Databases["main"]
	if p.NoCache == nil || !*p.NoCache {
		t.Error("no_cache should be true")
	}
}

func TestLoad_BackupSettings(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	writeTOML(t, cfgPath, `
default = "main"

[databases.main]
database = "~/vaults/main.kdbx"
backup_keep = 10
backup_max_age_days = 30
`)

	fc, _, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	p := fc.Databases["main"]
	if p.BackupKeep != 10 {
		t.Errorf("backup_keep = %d, want 10", p.BackupKeep)
	}
	if p.BackupMaxAgeDays != 30 {
		t.Errorf("backup_max_age = %d, want 30", p.BackupMaxAgeDays)
	}
}

func TestLoad_PasswordFileAndPasswordDBConflict(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	writeTOML(t, cfgPath, `
default = "main"

[databases.main]
database = "~/vaults/main.kdbx"
password_file = "~/vaults/pw.txt"
password_database = "other"
password_entry = "vaults/main"
`)

	_, _, err := Load(cfgPath)
	if err == nil || !strings.Contains(err.Error(), "cannot combine") {
		t.Errorf("expected conflict error, got: %v", err)
	}
}

func TestResolveProfile_Basic(t *testing.T) {
	fc := FileConfig{
		DefaultDatabase: "main",
		Databases: map[string]Profile{
			"main": {Database: "/tmp/test.kdbx"},
		},
	}
	cfg, err := ResolveProfile(fc, "main", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Database != "/tmp/test.kdbx" {
		t.Errorf("database = %q", cfg.Database)
	}
}

func TestResolveProfile_Unknown(t *testing.T) {
	fc := FileConfig{}
	_, err := ResolveProfile(fc, "unknown", nil, nil)
	if err == nil {
		t.Error("expected error for unknown profile")
	}
}

func TestResolveProfile_ChainedPassword(t *testing.T) {
	fc := FileConfig{
		Databases: map[string]Profile{
			"bootstrap": {Database: "/tmp/bootstrap.kdbx"},
			"main": {
				Database:         "/tmp/main.kdbx",
				PasswordDatabase: "bootstrap",
				PasswordEntry:    "vaults/main",
			},
		},
	}

	fetcher := func(src Config, entryPath string) (string, error) {
		return "fetched-password", nil
	}

	cfg, err := ResolveProfile(fc, "main", fetcher, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Password != "fetched-password" {
		t.Errorf("password = %q, want fetched-password", cfg.Password)
	}
}

func TestResolveProfile_LoopDetection(t *testing.T) {
	fc := FileConfig{
		Databases: map[string]Profile{
			"a": {
				Database:         "/tmp/a.kdbx",
				PasswordDatabase: "b",
				PasswordEntry:    "x",
			},
			"b": {
				Database:         "/tmp/b.kdbx",
				PasswordDatabase: "a",
				PasswordEntry:    "y",
			},
		},
	}

	fetcher := func(src Config, entryPath string) (string, error) {
		return "pw", nil
	}

	_, err := ResolveProfile(fc, "a", fetcher, nil)
	if err == nil || !strings.Contains(err.Error(), "loop") {
		t.Errorf("expected loop error, got: %v", err)
	}
}

func TestResolveProfile_NoFetcher(t *testing.T) {
	fc := FileConfig{
		Databases: map[string]Profile{
			"main": {
				Database:         "/tmp/main.kdbx",
				PasswordDatabase: "other",
				PasswordEntry:    "x",
			},
			"other": {Database: "/tmp/other.kdbx"},
		},
	}

	_, err := ResolveProfile(fc, "main", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "no password fetcher") {
		t.Errorf("expected no fetcher error, got: %v", err)
	}
}

func TestResolveRuntime_FlagsOverrideProfile(t *testing.T) {
	fc := FileConfig{
		DefaultDatabase: "main",
		Databases: map[string]Profile{
			"main": {Database: "/tmp/main.kdbx"},
		},
	}
	flags := RuntimeFlags{
		Database: "/custom/db.kdbx",
	}

	cfg, err := ResolveRuntime(fc, "", flags, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Database != "/custom/db.kdbx" {
		t.Errorf("database = %q, want /custom/db.kdbx", cfg.Database)
	}
}

func TestResolveRuntime_DBAndSelectorConflict(t *testing.T) {
	fc := FileConfig{}
	flags := RuntimeFlags{Database: "/tmp/test.kdbx"}

	_, err := ResolveRuntime(fc, "work", flags, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "cannot combine") {
		t.Errorf("expected conflict error, got: %v", err)
	}
}

func TestEnvCacheTTL_Unset(t *testing.T) {
	t.Setenv("KPASS_SESSION_TTL", "")
	t.Setenv("KPASS_CACHE_TTL", "")
	v, err := EnvCacheTTL()
	if err != nil {
		t.Fatal(err)
	}
	if v != -1 {
		t.Errorf("unset ttl = %d, want -1", v)
	}
}

func TestEnvCacheTTL_Set(t *testing.T) {
	t.Setenv("KPASS_SESSION_TTL", "120")
	v, err := EnvCacheTTL()
	if err != nil {
		t.Fatal(err)
	}
	if v != 120 {
		t.Errorf("ttl = %d, want 120", v)
	}
}

func TestEnvCacheTTL_CacheTTLFallback(t *testing.T) {
	t.Setenv("KPASS_SESSION_TTL", "")
	t.Setenv("KPASS_CACHE_TTL", "900")
	v, err := EnvCacheTTL()
	if err != nil {
		t.Fatal(err)
	}
	if v != 900 {
		t.Errorf("ttl = %d, want 900", v)
	}
}

func TestEnvCacheTTL_Invalid(t *testing.T) {
	t.Setenv("KPASS_SESSION_TTL", "not-a-number")
	_, err := EnvCacheTTL()
	if err == nil {
		t.Error("expected error for invalid ttl")
	}
}

func TestReadPasswordFile(t *testing.T) {
	dir := t.TempDir()
	pwPath := filepath.Join(dir, "pw.txt")
	writeTOML(t, pwPath, "my-password\nextra line")

	pw, err := ReadPasswordFile(pwPath)
	if err != nil {
		t.Fatal(err)
	}
	if pw != "my-password" {
		t.Errorf("password = %q, want my-password", pw)
	}
}

func TestReadPasswordFile_NotFound(t *testing.T) {
	_, err := ReadPasswordFile("/nonexistent/pw.txt")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestReadPasswordFile_NoNewline(t *testing.T) {
	dir := t.TempDir()
	pwPath := filepath.Join(dir, "pw.txt")
	writeTOML(t, pwPath, "single-line")

	pw, err := ReadPasswordFile(pwPath)
	if err != nil {
		t.Fatal(err)
	}
	if pw != "single-line" {
		t.Errorf("password = %q, want single-line", pw)
	}
}

func TestTrimSpace(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", ""},
		{"  hello  ", "hello"},
		{"\t\n  world  \n\t", "world"},
	}
	for _, tt := range tests {
		got := strings.TrimSpace(tt.in)
		if got != tt.want {
			t.Errorf("strings.TrimSpace(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestAsInt(t *testing.T) {
	v, err := asInt(int64(42))
	if err != nil || v != 42 {
		t.Errorf("asInt(int64) = %d, err=%v", v, err)
	}
	v, err = asInt(7)
	if err != nil || v != 7 {
		t.Errorf("asInt(int) = %d, err=%v", v, err)
	}
	_, err = asInt("nope")
	if err == nil {
		t.Error("expected error for non-int")
	}
}

func TestAsBool(t *testing.T) {
	b, err := asBool(true)
	if err != nil || !b {
		t.Errorf("asBool(true) = %v, err=%v", b, err)
	}
	_, err = asBool(1)
	if err == nil {
		t.Error("expected error for non-bool")
	}
}

func TestJoin(t *testing.T) {
	if got := strings.Join([]string{"a", "b", "c"}, ", "); got != "a, b, c" {
		t.Errorf("strings.Join = %q", got)
	}
	if got := strings.Join([]string{"x"}, ", "); got != "x" {
		t.Errorf("strings.Join = %q", got)
	}
	if got := strings.Join(nil, ", "); got != "" {
		t.Errorf("strings.Join(nil) = %q", got)
	}
}
