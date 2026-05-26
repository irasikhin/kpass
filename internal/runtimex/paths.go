package runtimex

import (
	"os"
	"path/filepath"
	"strings"
)

const DefaultDBPath = "~/.keepass/keepass.kdbx"
const DefaultConfigPath = "~/.config/kpass/config.toml"

func NormalizePath(s string) string {
	return strings.Trim(strings.TrimSpace(s), "/")
}

func ExpandPath(s string) string {
	if s == "" {
		return s
	}
	if s == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
		return s
	}
	if strings.HasPrefix(s, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, s[2:])
		}
	}
	return s
}

func ConfigFilePath(explicit string) string {
	if explicit == "" {
		explicit = os.Getenv("KPASS_CONFIG")
	}
	if explicit == "" {
		explicit = DefaultConfigPath
	}
	return ExpandPath(explicit)
}

func SplitPath(value string) []string {
	normalized := NormalizePath(value)
	if normalized == "" {
		return nil
	}
	parts := strings.Split(normalized, "/")
	out := parts[:0]
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func JoinPath(parts []string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, "/")
}
