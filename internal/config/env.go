package config

import (
	"fmt"
	"os"
	"strconv"
)

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
		return 0, fmt.Errorf("KPASS_SESSION_TTL must be an integer.")
	}
	return v, nil
}
