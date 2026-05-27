package config

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/irasikhin/kpass/internal/runtimex"
)

var profileKeys = map[string]bool{
	"database":            true,
	"password_file":       true,
	"password_database":   true,
	"password_entry":      true,
	"key_file":            true,
	"session_ttl":         true,
	"cache_ttl":           true,
	"no_session":          true,
	"no_cache":            true,
	"backup_keep":         true,
	"backup_max_age_days": true,
}

var topLevelKeys = map[string]bool{
	"default":          true,
	"default_database": true,
	"databases":        true,
}

// Load parses the config file at the given path (or the default if empty).
// Returns an empty FileConfig if the file does not exist.
func Load(explicit string) (FileConfig, string, error) {
	path := runtimex.ConfigFilePath(explicit)
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return FileConfig{}, path, nil
		}
		return FileConfig{}, path, fmt.Errorf("kpass config path is not accessible: %s", path)
	}
	if info.IsDir() {
		return FileConfig{}, path, fmt.Errorf("kpass config path is not a file: %s", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return FileConfig{}, path, fmt.Errorf("failed to read KPass config %s: %v", path, err)
	}

	var raw map[string]any
	if _, err := toml.Decode(string(data), &raw); err != nil {
		return FileConfig{}, path, fmt.Errorf("failed to parse KPass config %s: %v", path, err)
	}

	for k := range raw {
		if !topLevelKeys[k] {
			unknown := sortedUnknown(raw, topLevelKeys)
			return FileConfig{}, path, fmt.Errorf("unsupported KPass config key(s): %s", strings.Join(unknown, ", "))
		}
	}

	defaultDB, err := defaultDatabase(raw)
	if err != nil {
		return FileConfig{}, path, err
	}
	if defaultDB == "" || strings.TrimSpace(defaultDB) == "" {
		return FileConfig{}, path, fmt.Errorf("kpass config must define a non-empty top-level 'default' database profile name")
	}

	dbsRaw, _ := raw["databases"].(map[string]any)
	if len(dbsRaw) == 0 {
		return FileConfig{}, path, fmt.Errorf("kpass config must define at least one [databases.<name>] profile")
	}

	names := make([]string, 0, len(dbsRaw))
	for name := range dbsRaw {
		names = append(names, name)
	}
	sort.Strings(names)

	dbs := make(map[string]Profile, len(names))
	for _, name := range names {
		p, err := parseProfile(name, dbsRaw[name], path)
		if err != nil {
			return FileConfig{}, path, err
		}
		dbs[name] = p
	}

	if _, ok := dbs[defaultDB]; !ok {
		return FileConfig{}, path, fmt.Errorf("default database profile not found: %s", defaultDB)
	}

	return FileConfig{DefaultDatabase: defaultDB, Databases: dbs}, path, nil
}

func defaultDatabase(raw map[string]any) (string, error) {
	def, hasDefault := raw["default"].(string)
	legacy, hasLegacy := raw["default_database"].(string)
	if hasDefault && hasLegacy && def != legacy {
		return "", fmt.Errorf("kpass config cannot define both 'default' and legacy 'default_database' with different values")
	}
	if hasDefault {
		return def, nil
	}
	if hasLegacy {
		return legacy, nil
	}
	return "", nil
}

func parseProfile(name string, raw any, path string) (Profile, error) {
	data, ok := raw.(map[string]any)
	if !ok {
		return Profile{}, fmt.Errorf("kpass database profile '%s' must be a TOML table: %s", name, path)
	}

	var unknown []string
	for k := range data {
		if !profileKeys[k] {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return Profile{}, fmt.Errorf("unsupported KPass config key(s) in profile '%s': %s", name, strings.Join(unknown, ", "))
	}

	database, ok := data["database"].(string)
	if !ok || strings.TrimSpace(database) == "" {
		return Profile{}, fmt.Errorf("kpass database profile '%s' must define a non-empty 'database' path", name)
	}

	var ttl *int
	if v, ok := data["session_ttl"]; ok {
		t, err := asInt(v)
		if err != nil {
			return Profile{}, fmt.Errorf("kpass config key 'session_ttl' in profile '%s' must be an integer", name)
		}
		ttl = &t
	} else if v, ok := data["cache_ttl"]; ok {
		t, err := asInt(v)
		if err != nil {
			return Profile{}, fmt.Errorf("kpass config key 'session_ttl' in profile '%s' must be an integer", name)
		}
		ttl = &t
	}

	var noCache *bool
	if v, ok := data["no_session"]; ok {
		b, err := asBool(v)
		if err != nil {
			return Profile{}, fmt.Errorf("kpass config key 'no_session' in profile '%s' must be a boolean", name)
		}
		noCache = &b
	} else if v, ok := data["no_cache"]; ok {
		b, err := asBool(v)
		if err != nil {
			return Profile{}, fmt.Errorf("kpass config key 'no_session' in profile '%s' must be a boolean", name)
		}
		noCache = &b
	}

	for _, k := range []string{"password_file", "key_file", "password_database", "password_entry"} {
		if v, ok := data[k]; ok {
			if _, ok := v.(string); !ok {
				return Profile{}, fmt.Errorf("kpass config key '%s' in profile '%s' must be a string", k, name)
			}
		}
	}

	passwordFile, _ := data["password_file"].(string)
	passwordDB, _ := data["password_database"].(string)
	passwordEntry, _ := data["password_entry"].(string)
	keyFile, _ := data["key_file"].(string)

	if passwordFile != "" && (passwordDB != "" || passwordEntry != "") {
		return Profile{}, fmt.Errorf("kpass database profile '%s' cannot combine 'password_file' with password lookup from another database", name)
	}
	if (passwordDB == "") != (passwordEntry == "") {
		return Profile{}, fmt.Errorf("kpass database profile '%s' must set both 'password_database' and 'password_entry' together", name)
	}

	var backupKeep int
	if v, ok := data["backup_keep"]; ok {
		n, err := asInt(v)
		if err != nil || n < 0 {
			return Profile{}, fmt.Errorf("kpass config key 'backup_keep' in profile '%s' must be a non-negative integer", name)
		}
		backupKeep = n
	}
	var backupMaxAge int
	if v, ok := data["backup_max_age_days"]; ok {
		n, err := asInt(v)
		if err != nil || n < 0 {
			return Profile{}, fmt.Errorf("kpass config key 'backup_max_age_days' in profile '%s' must be a non-negative integer", name)
		}
		backupMaxAge = n
	}

	return Profile{
		Database:         runtimex.ExpandPath(database),
		PasswordFile:     runtimex.ExpandPath(passwordFile),
		PasswordDatabase: passwordDB,
		PasswordEntry:    passwordEntry,
		KeyFile:          runtimex.ExpandPath(keyFile),
		CacheTTL:         ttl,
		NoCache:          noCache,
		BackupKeep:       backupKeep,
		BackupMaxAgeDays: backupMaxAge,
	}, nil
}

func sortedUnknown(m map[string]any, known map[string]bool) []string {
	var out []string
	for k := range m {
		if !known[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func asInt(v any) (int, error) {
	switch t := v.(type) {
	case int64:
		return int(t), nil
	case int:
		return t, nil
	}
	return 0, fmt.Errorf("not int")
}

func asBool(v any) (bool, error) {
	b, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("not bool")
	}
	return b, nil
}
