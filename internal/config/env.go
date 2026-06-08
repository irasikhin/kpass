package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// EnvUseKeyring reads KPASS_USE_KEYRING. Returns nil if unset, otherwise a
// pointer to the parsed truthy value ("1"/"true"/"yes"/"on" => true).
func EnvUseKeyring() *bool {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv("KPASS_USE_KEYRING")))
	if raw == "" {
		return nil
	}
	b := raw == "1" || raw == "true" || raw == "yes" || raw == "on"
	return &b
}

// EnvCacheTTL reads KPASS_SESSION_TTL / KPASS_CACHE_TTL. Returns -1 if unset.
func EnvCacheTTL() (int, error) {
	raw := os.Getenv("KPASS_SESSION_TTL")
	if raw == "" {
		raw = os.Getenv("KPASS_CACHE_TTL")
	}
	if raw == "" {
		return -1, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid KPASS_SESSION_TTL: must be an integer")
	}
	return v, nil
}
