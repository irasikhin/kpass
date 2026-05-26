package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// WriteAtomic serializes fc to path. Mirrors Python's dump_file_config.
func WriteAtomic(path string, fc FileConfig) error {
	if fc.DefaultDatabase == "" {
		return fmt.Errorf("KPass config must define a default database profile name.")
	}
	if len(fc.Databases) == 0 {
		return fmt.Errorf("KPass config must define at least one [databases.<name>] profile.")
	}
	body := Dump(fc)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(body), 0o600)
}

func Dump(fc FileConfig) string {
	var b strings.Builder
	fmt.Fprintf(&b, "default = %s\n\n", tomlQuote(fc.DefaultDatabase))
	names := make([]string, 0, len(fc.Databases))
	for n := range fc.Databases {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		p := fc.Databases[name]
		fmt.Fprintf(&b, "[databases.%s]\n", tomlKey(name))
		fmt.Fprintf(&b, "database = %s\n", tomlQuote(p.Database))
		if p.PasswordFile != "" {
			fmt.Fprintf(&b, "password_file = %s\n", tomlQuote(p.PasswordFile))
		}
		if p.PasswordDatabase != "" {
			fmt.Fprintf(&b, "password_database = %s\n", tomlQuote(p.PasswordDatabase))
		}
		if p.PasswordEntry != "" {
			fmt.Fprintf(&b, "password_entry = %s\n", tomlQuote(p.PasswordEntry))
		}
		if p.KeyFile != "" {
			fmt.Fprintf(&b, "key_file = %s\n", tomlQuote(p.KeyFile))
		}
		if p.CacheTTL != nil {
			fmt.Fprintf(&b, "session_ttl = %d\n", *p.CacheTTL)
		}
		if p.NoCache != nil {
			if *p.NoCache {
				b.WriteString("no_session = true\n")
			} else {
				b.WriteString("no_session = false\n")
			}
		}
		if p.BackupKeep > 0 {
			fmt.Fprintf(&b, "backup_keep = %d\n", p.BackupKeep)
		}
		if p.BackupMaxAgeDays > 0 {
			fmt.Fprintf(&b, "backup_max_age_days = %d\n", p.BackupMaxAgeDays)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func tomlQuote(s string) string {
	out, _ := json.Marshal(s)
	return string(out)
}

func tomlKey(k string) string {
	if k == "" {
		return tomlQuote(k)
	}
	if !isAlpha(k[0]) {
		return tomlQuote(k)
	}
	for i := 0; i < len(k); i++ {
		c := k[i]
		if !isAlphaNum(c) && c != '-' && c != '_' {
			return tomlQuote(k)
		}
	}
	return k
}

func isAlpha(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isAlphaNum(c byte) bool {
	return isAlpha(c) || (c >= '0' && c <= '9')
}
