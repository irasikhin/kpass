package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/irasikhin/kpass/internal/config"
	"github.com/irasikhin/kpass/internal/runtimex"
)

type entry struct {
	ExpiresAt int64  `json:"expires_at"`
	Password  string `json:"password"`
}

// Enabled mirrors Python's cache_enabled.
func Enabled(c config.Config) bool {
	if c.PasswordFile != "" {
		return false
	}
	if c.NoCache {
		return false
	}
	if c.CacheTTL <= 0 {
		return false
	}
	return os.Getenv("XDG_RUNTIME_DIR") != ""
}

// Path returns the cache file path; "" if XDG_RUNTIME_DIR is unset.
// Creates the kpass cache directory with mode 0700.
func Path(database, keyFile string) (string, error) {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		return "", nil
	}
	dbAbs := absResolve(runtimex.ExpandPath(database))
	keyAbs := ""
	if keyFile != "" {
		keyAbs = absResolve(runtimex.ExpandPath(keyFile))
	}
	h := sha256.Sum256([]byte(dbAbs + "::" + keyAbs))
	cacheDir := filepath.Join(runtimeDir, "kpass")
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return "", err
	}
	_ = os.Chmod(cacheDir, 0o700)
	return filepath.Join(cacheDir, hex.EncodeToString(h[:])+".json"), nil
}

func absResolve(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return abs
	}
	return resolved
}

// Load returns the cached password if present, valid, and unexpired.
// On any error or expiry, removes the cache file and returns "".
//
// A missing, unreadable, or corrupt cache file is reported as ("", nil) —
// the caller treats it as a cache miss and re-prompts. We only surface
// errors when even computing the cache path failed.
func Load(database, keyFile string) (string, error) {
	p, err := Path(database, keyFile)
	if err != nil || p == "" {
		return "", err
	}
	info, statErr := os.Stat(p)
	if statErr != nil || info.IsDir() {
		return "", nil //nolint:nilerr // no cache file = miss
	}
	data, err := os.ReadFile(p)
	if err != nil {
		_ = os.Remove(p)
		return "", nil //nolint:nilerr // corrupt cache → miss, not caller error
	}
	var e entry
	if err := json.Unmarshal(data, &e); err != nil {
		_ = os.Remove(p)
		return "", nil //nolint:nilerr // corrupt cache → miss, not caller error
	}
	if e.Password == "" || e.ExpiresAt <= time.Now().Unix() {
		_ = os.Remove(p)
		return "", nil
	}
	return e.Password, nil
}

// Store writes the password and TTL to the cache file with mode 0600.
func Store(database, keyFile, password string, ttl int) error {
	p, err := Path(database, keyFile)
	if err != nil || p == "" {
		return err
	}
	data, err := json.Marshal(entry{ExpiresAt: time.Now().Unix() + int64(ttl), Password: password})
	if err != nil {
		return err
	}
	if err := os.WriteFile(p, data, 0o600); err != nil {
		return err
	}
	return os.Chmod(p, 0o600)
}

// Clear removes the cache file for a specific db; returns true if a file was
// removed.
func Clear(database, keyFile string) (bool, error) {
	p, err := Path(database, keyFile)
	if err != nil || p == "" {
		return false, err
	}
	if _, err := os.Stat(p); err != nil {
		return false, nil //nolint:nilerr // no cache file = nothing to clear
	}
	if err := os.Remove(p); err != nil {
		return false, err
	}
	return true, nil
}

// ClearAll removes every kpass cache file in $XDG_RUNTIME_DIR/kpass/.
func ClearAll() (int, error) {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		return 0, nil
	}
	dir := filepath.Join(runtimeDir, "kpass")
	matches, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil || len(matches) == 0 {
		return 0, nil //nolint:nilerr // glob failure → nothing to clear
	}
	removed := 0
	for _, m := range matches {
		if err := os.Remove(m); err == nil {
			removed++
		}
	}
	return removed, nil
}
