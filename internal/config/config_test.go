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

func TestLoad_RejectsLegacyDefaultDatabaseKey(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	writeTOML(t, cfgPath, `
default_database = "main"

[databases.main]
database = "~/vaults/main.kdbx"
`)

	_, _, err := Load(cfgPath)
	if err == nil || !strings.Contains(err.Error(), "default_database") {
		t.Errorf("expected unsupported-key error for default_database, got: %v", err)
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

func TestLoad_PathIsDirectory(t *testing.T) {
	dir := t.TempDir()
	_, _, err := Load(dir)
	if err == nil || !strings.Contains(err.Error(), "not a file") {
		t.Errorf("expected 'not a file' error, got %v", err)
	}
}

func TestLoad_MalformedTOML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	writeTOML(t, cfgPath, "default = \n[")
	_, _, err := Load(cfgPath)
	if err == nil || !strings.Contains(err.Error(), "parse") {
		t.Errorf("expected parse error, got %v", err)
	}
}

func TestLoad_DefaultEmpty(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	writeTOML(t, cfgPath, `default = ""

[databases.main]
database = "x"
`)
	_, _, err := Load(cfgPath)
	if err == nil || !strings.Contains(err.Error(), "non-empty") {
		t.Errorf("expected non-empty error, got %v", err)
	}
}

func TestParseProfile_BadCacheTTL(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	writeTOML(t, cfgPath, `
default = "main"

[databases.main]
database = "x"
session_ttl = "not-int"
`)
	_, _, err := Load(cfgPath)
	if err == nil || !strings.Contains(err.Error(), "session_ttl") {
		t.Errorf("expected session_ttl error, got %v", err)
	}
}

func TestParseProfile_BadCacheTTLLegacy(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	writeTOML(t, cfgPath, `
default = "main"

[databases.main]
database = "x"
cache_ttl = "not-int"
`)
	_, _, err := Load(cfgPath)
	if err == nil || !strings.Contains(err.Error(), "session_ttl") {
		t.Errorf("expected session_ttl error for legacy cache_ttl, got %v", err)
	}
}

func TestParseProfile_BadNoCache(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	writeTOML(t, cfgPath, `
default = "main"

[databases.main]
database = "x"
no_session = "yes"
`)
	_, _, err := Load(cfgPath)
	if err == nil || !strings.Contains(err.Error(), "no_session") {
		t.Errorf("expected no_session error, got %v", err)
	}
}

func TestParseProfile_BadNoCacheLegacy(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	writeTOML(t, cfgPath, `
default = "main"

[databases.main]
database = "x"
no_cache = "nope"
`)
	_, _, err := Load(cfgPath)
	if err == nil {
		t.Error("expected no_cache type error")
	}
}

func TestParseProfile_NonStringField(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	writeTOML(t, cfgPath, `
default = "main"

[databases.main]
database = "x"
password_file = 42
`)
	_, _, err := Load(cfgPath)
	if err == nil || !strings.Contains(err.Error(), "password_file") {
		t.Errorf("expected password_file string error, got %v", err)
	}
}

func TestParseProfile_NotATable(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	writeTOML(t, cfgPath, `
default = "main"

[databases]
main = "scalar"
`)
	_, _, err := Load(cfgPath)
	if err == nil {
		t.Error("expected error when profile is not a table")
	}
}

func TestParseProfile_UnknownKey(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	writeTOML(t, cfgPath, `
default = "main"

[databases.main]
database = "x"
unknown_thing = "y"
`)
	_, _, err := Load(cfgPath)
	if err == nil || !strings.Contains(err.Error(), "unknown_thing") {
		t.Errorf("expected unknown-key error, got %v", err)
	}
}

func TestParseProfile_PartialPasswordDB(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	writeTOML(t, cfgPath, `
default = "main"

[databases.main]
database = "x"
password_database = "other"
`)
	_, _, err := Load(cfgPath)
	if err == nil || !strings.Contains(err.Error(), "together") {
		t.Errorf("expected together error, got %v", err)
	}
}

func TestParseProfile_BackupKeepNegative(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	writeTOML(t, cfgPath, `
default = "main"

[databases.main]
database = "x"
backup_keep = -1
`)
	_, _, err := Load(cfgPath)
	if err == nil || !strings.Contains(err.Error(), "backup_keep") {
		t.Errorf("expected backup_keep error, got %v", err)
	}
}

func TestParseProfile_BackupMaxAgeNegative(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	writeTOML(t, cfgPath, `
default = "main"

[databases.main]
database = "x"
backup_max_age_days = -5
`)
	_, _, err := Load(cfgPath)
	if err == nil || !strings.Contains(err.Error(), "backup_max_age_days") {
		t.Errorf("expected backup_max_age_days error, got %v", err)
	}
}

func TestResolveProfile_FetcherError(t *testing.T) {
	fc := FileConfig{
		Databases: map[string]Profile{
			"main": {
				Database:         "/main.kdbx",
				PasswordDatabase: "other",
				PasswordEntry:    "x",
			},
			"other": {Database: "/other.kdbx"},
		},
	}
	fetcher := func(Config, string) (string, error) {
		return "", os.ErrPermission
	}
	if _, err := ResolveProfile(fc, "main", fetcher, nil); err == nil {
		t.Error("expected fetcher error to bubble")
	}
}

func TestResolveProfile_FetcherEmptyPassword(t *testing.T) {
	fc := FileConfig{
		Databases: map[string]Profile{
			"main": {
				Database:         "/main.kdbx",
				PasswordDatabase: "other",
				PasswordEntry:    "x",
			},
			"other": {Database: "/other.kdbx"},
		},
	}
	fetcher := func(Config, string) (string, error) { return "", nil }
	_, err := ResolveProfile(fc, "main", fetcher, nil)
	if err == nil || !strings.Contains(err.Error(), "does not contain a password") {
		t.Errorf("expected empty-password error, got %v", err)
	}
}

func TestResolveProfile_ChainedFromUnknownSource(t *testing.T) {
	fc := FileConfig{
		Databases: map[string]Profile{
			"main": {
				Database:         "/main.kdbx",
				PasswordDatabase: "ghost",
				PasswordEntry:    "x",
			},
		},
	}
	fetcher := func(Config, string) (string, error) { return "pw", nil }
	if _, err := ResolveProfile(fc, "main", fetcher, nil); err == nil {
		t.Error("expected unknown source error")
	}
}

func TestResolveProfile_LogWritten(t *testing.T) {
	fc := FileConfig{
		Databases: map[string]Profile{
			"main": {
				Database:         "/main.kdbx",
				PasswordDatabase: "other",
				PasswordEntry:    "vaults/main",
			},
			"other": {Database: "/other.kdbx"},
		},
	}
	fetcher := func(Config, string) (string, error) { return "pw", nil }
	var log strings.Builder
	if _, err := ResolveProfile(fc, "main", fetcher, &log); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(log.String(), "vaults/main") {
		t.Errorf("log = %q, want lookup line", log.String())
	}
}

func TestResolveRuntime_EnvFallbacks(t *testing.T) {
	t.Setenv("KEEPASS_DB_PATH", "/from-env.kdbx")
	t.Setenv("KPASS_PASSWORD_FILE", "/from-env.pw")
	t.Setenv("KPASS_KEY_FILE", "/from-env.key")
	t.Setenv("KPASS_SESSION_TTL", "")
	t.Setenv("KPASS_CACHE_TTL", "")

	cfg, err := ResolveRuntime(FileConfig{}, "", RuntimeFlags{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Database != "/from-env.kdbx" {
		t.Errorf("database = %q, want from env", cfg.Database)
	}
	if cfg.PasswordFile != "/from-env.pw" {
		t.Errorf("password_file = %q", cfg.PasswordFile)
	}
	if cfg.KeyFile != "/from-env.key" {
		t.Errorf("key_file = %q", cfg.KeyFile)
	}
	if cfg.CacheTTL != DefaultCacheTTL {
		t.Errorf("ttl = %d, want %d", cfg.CacheTTL, DefaultCacheTTL)
	}
}

func TestResolveRuntime_EnvCacheTTL(t *testing.T) {
	t.Setenv("KPASS_SESSION_TTL", "777")
	cfg, err := ResolveRuntime(FileConfig{}, "", RuntimeFlags{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CacheTTL != 777 {
		t.Errorf("ttl = %d, want 777", cfg.CacheTTL)
	}
}

func TestResolveRuntime_EnvCacheTTLInvalid(t *testing.T) {
	t.Setenv("KPASS_SESSION_TTL", "nope")
	if _, err := ResolveRuntime(FileConfig{}, "", RuntimeFlags{}, nil, nil); err == nil {
		t.Error("expected invalid TTL error")
	}
}

func TestResolveRuntime_FlagCacheTTLOverridesEnv(t *testing.T) {
	t.Setenv("KPASS_SESSION_TTL", "777")
	five := 5
	cfg, err := ResolveRuntime(FileConfig{}, "", RuntimeFlags{CacheTTL: &five}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CacheTTL != 5 {
		t.Errorf("flag TTL should win, got %d", cfg.CacheTTL)
	}
}

func TestResolveRuntime_ProfileDefaults(t *testing.T) {
	ttl := 60
	yes := true
	fc := FileConfig{
		DefaultDatabase: "main",
		Databases: map[string]Profile{
			"main": {
				Database:         "/from-profile.kdbx",
				PasswordFile:     "/from-profile.pw",
				KeyFile:          "/from-profile.key",
				CacheTTL:         &ttl,
				NoCache:          &yes,
				BackupKeep:       4,
				BackupMaxAgeDays: 7,
			},
		},
	}
	t.Setenv("KEEPASS_DB_PATH", "")
	t.Setenv("KPASS_PASSWORD_FILE", "")
	t.Setenv("KPASS_KEY_FILE", "")
	t.Setenv("KPASS_SESSION_TTL", "")
	t.Setenv("KPASS_CACHE_TTL", "")

	cfg, err := ResolveRuntime(fc, "", RuntimeFlags{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Database != "/from-profile.kdbx" || cfg.PasswordFile != "/from-profile.pw" || cfg.KeyFile != "/from-profile.key" {
		t.Errorf("profile defaults not applied: %+v", cfg)
	}
	if cfg.CacheTTL != 60 || !cfg.NoCache {
		t.Errorf("profile cache settings: ttl=%d, no_cache=%t", cfg.CacheTTL, cfg.NoCache)
	}
	if cfg.BackupKeep != 4 || cfg.BackupMaxAgeDays != 7 {
		t.Errorf("backup defaults: keep=%d, age=%d", cfg.BackupKeep, cfg.BackupMaxAgeDays)
	}
}

func TestResolveRuntime_FlagNoCacheOverridesProfile(t *testing.T) {
	yes := true
	no := false
	fc := FileConfig{
		DefaultDatabase: "main",
		Databases: map[string]Profile{
			"main": {Database: "/x.kdbx", NoCache: &yes},
		},
	}
	cfg, err := ResolveRuntime(fc, "", RuntimeFlags{NoCache: &no}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.NoCache {
		t.Error("flag false should override profile true")
	}
}

func TestResolveRuntime_UnknownSelector(t *testing.T) {
	fc := FileConfig{
		DefaultDatabase: "main",
		Databases: map[string]Profile{
			"main": {Database: "/x.kdbx"},
		},
	}
	if _, err := ResolveRuntime(fc, "ghost", RuntimeFlags{}, nil, nil); err == nil {
		t.Error("expected unknown selector error")
	}
}

func TestResolveRuntime_ResolveProfileErrorPropagates(t *testing.T) {
	fc := FileConfig{
		DefaultDatabase: "main",
		Databases: map[string]Profile{
			"main": {
				Database:         "/main.kdbx",
				PasswordDatabase: "other",
				PasswordEntry:    "x",
			},
			"other": {Database: "/other.kdbx"},
		},
	}
	// nil fetcher with chained profile → resolveProfile returns "no fetcher" error.
	if _, err := ResolveRuntime(fc, "", RuntimeFlags{}, nil, nil); err == nil {
		t.Error("expected chained-resolve error to propagate")
	}
}

func TestReadPasswordFile_Directory(t *testing.T) {
	dir := t.TempDir()
	if _, err := ReadPasswordFile(dir); err == nil {
		t.Error("expected error for directory")
	}
}
