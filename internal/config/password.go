package config

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/irasikhin/kpass/internal/runtimex"
)

// PasswordFetcher fetches a password from `src` by opening the source DB and
// looking up `entryPath`. cli wires this to db.Open + db.GetPassword.
type PasswordFetcher func(src Config, entryPath string) (string, error)

// ResolveProfile returns a Config for the named profile, recursively
// resolving any password_database chain via fetcher. log receives a status
// line per chained lookup (sent to stderr in production).
func ResolveProfile(fc FileConfig, name string, fetcher PasswordFetcher, log io.Writer) (Config, error) {
	return resolveProfile(fc, name, fetcher, log, nil)
}

func resolveProfile(fc FileConfig, name string, fetcher PasswordFetcher, log io.Writer, resolving []string) (Config, error) {
	for _, n := range resolving {
		if n == name {
			loop := strings.Join(append(append([]string{}, resolving...), name), " -> ")
			return Config{}, fmt.Errorf("Database password resolution loop detected: %s", loop)
		}
	}

	profile, ok := fc.Databases[name]
	if !ok {
		return Config{}, fmt.Errorf("Unknown database profile: %s", name)
	}

	var password string
	if profile.PasswordDatabase != "" && profile.PasswordEntry != "" {
		if log != nil {
			fmt.Fprintf(log, "Using password for database profile '%s' from '%s:%s'\n", name, profile.PasswordDatabase, profile.PasswordEntry)
		}
		src, err := resolveProfile(fc, profile.PasswordDatabase, fetcher, log, append(resolving, name))
		if err != nil {
			return Config{}, err
		}
		if fetcher == nil {
			return Config{}, fmt.Errorf("no password fetcher available.")
		}
		pw, err := fetcher(src, profile.PasswordEntry)
		if err != nil {
			return Config{}, err
		}
		if pw == "" {
			return Config{}, fmt.Errorf("password entry '%s' in database profile '%s' does not contain a password.", profile.PasswordEntry, profile.PasswordDatabase)
		}
		password = pw
	}

	ttl := DefaultCacheTTL
	if profile.CacheTTL != nil {
		ttl = *profile.CacheTTL
	}
	noCache := false
	if profile.NoCache != nil {
		noCache = *profile.NoCache
	}

	return Config{
		Database:         profile.Database,
		PasswordFile:     profile.PasswordFile,
		Password:         password,
		KeyFile:          profile.KeyFile,
		CacheTTL:         ttl,
		NoCache:          noCache,
		BackupKeep:       profile.BackupKeep,
		BackupMaxAgeDays: profile.BackupMaxAgeDays,
	}, nil
}

// RuntimeFlags carries the global CLI flag overrides.
type RuntimeFlags struct {
	Database     string
	PasswordFile string
	KeyFile      string
	CacheTTL     *int
	NoCache      *bool
}

// ResolveRuntime merges CLI flags, env vars, and the selected profile into
// the final Config. Mirrors Python's resolve_runtime_config.
func ResolveRuntime(fc FileConfig, selectedDatabase string, flags RuntimeFlags, fetcher PasswordFetcher, log io.Writer) (Config, error) {
	if selectedDatabase != "" && flags.Database != "" {
		return Config{}, fmt.Errorf("cannot combine @db with --database.")
	}

	profileName := selectedDatabase
	if profileName == "" {
		profileName = fc.DefaultDatabase
	}

	var profileConfig *Config
	if profileName != "" {
		if _, ok := fc.Databases[profileName]; ok {
			c, err := resolveProfile(fc, profileName, fetcher, log, nil)
			if err != nil {
				return Config{}, err
			}
			profileConfig = &c
		} else if selectedDatabase != "" {
			return Config{}, fmt.Errorf("Unknown database profile: %s", selectedDatabase)
		}
	}

	database := flags.Database
	if database == "" {
		database = os.Getenv("KEEPASS_DB_PATH")
	}
	if database == "" && profileConfig != nil {
		database = profileConfig.Database
	}
	if database == "" {
		database = runtimex.ExpandPath(runtimex.DefaultDBPath)
	}

	passwordFile := flags.PasswordFile
	if passwordFile == "" {
		passwordFile = os.Getenv("KPASS_PASSWORD_FILE")
	}
	if passwordFile == "" && profileConfig != nil {
		passwordFile = profileConfig.PasswordFile
	}

	keyFile := flags.KeyFile
	if keyFile == "" {
		keyFile = os.Getenv("KPASS_KEY_FILE")
	}
	if keyFile == "" && profileConfig != nil {
		keyFile = profileConfig.KeyFile
	}

	var password string
	if passwordFile == "" && profileConfig != nil {
		password = profileConfig.Password
	}

	ttlOverride, err := EnvCacheTTL()
	if err != nil {
		return Config{}, err
	}
	var ttl int
	switch {
	case flags.CacheTTL != nil:
		ttl = *flags.CacheTTL
	case ttlOverride >= 0:
		ttl = ttlOverride
	case profileConfig != nil:
		ttl = profileConfig.CacheTTL
	default:
		ttl = DefaultCacheTTL
	}

	noCache := false
	switch {
	case flags.NoCache != nil:
		noCache = *flags.NoCache
	case profileConfig != nil:
		noCache = profileConfig.NoCache
	}

	var backupKeep, backupMaxAge int
	if profileConfig != nil {
		backupKeep = profileConfig.BackupKeep
		backupMaxAge = profileConfig.BackupMaxAgeDays
	}

	return Config{
		Database:         database,
		PasswordFile:     passwordFile,
		Password:         password,
		KeyFile:          keyFile,
		CacheTTL:         ttl,
		NoCache:          noCache,
		BackupKeep:       backupKeep,
		BackupMaxAgeDays: backupMaxAge,
	}, nil
}

// ReadPasswordFile returns the first line of the password file (empty if file
// is empty). Returns error if the file is missing.
func ReadPasswordFile(path string) (string, error) {
	expanded := runtimex.ExpandPath(path)
	info, err := os.Stat(expanded)
	if err != nil || info.IsDir() {
		return "", fmt.Errorf("password file not found: %s", expanded)
	}
	data, err := os.ReadFile(expanded)
	if err != nil {
		return "", fmt.Errorf("password file not found: %s", expanded)
	}
	s := string(data)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i], nil
	}
	return s, nil
}
