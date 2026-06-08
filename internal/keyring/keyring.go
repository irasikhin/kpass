// Package keyring stores the KeePass master password in the operating
// system's secret store (Linux Secret Service incl. gnome-keyring, macOS
// Keychain, Windows Credential Manager) via the pure-Go zalando/go-keyring.
//
// It is an opt-in alternative to the plaintext TTL cache (internal/cache):
// when a profile enables it, the master password is persisted OS-encrypted
// rather than written to $XDG_RUNTIME_DIR.
package keyring

import (
	"errors"
	"path/filepath"

	gokeyring "github.com/zalando/go-keyring"

	"github.com/irasikhin/kpass/internal/runtimex"
)

// Service is the keyring service name under which all kpass secrets are stored.
const Service = "kpass"

// Seams for tests; defaults call the real OS keyring.
var (
	backendSet    = gokeyring.Set
	backendGet    = gokeyring.Get
	backendDelete = gokeyring.Delete
)

// ErrNotFound is returned by Get when no secret is stored for the account.
var ErrNotFound = gokeyring.ErrNotFound

// Account derives the keyring account key for a database + key-file pair so
// each db/keyfile combination gets a distinct secret. The value is the
// resolved absolute database path (human-readable in `secret-tool` listings),
// with "::<keyfile>" appended when a key file is set. Resolution mirrors the
// cache identity (expand ~, make absolute, follow symlinks).
func Account(database, keyFile string) string {
	acct := absResolve(runtimex.ExpandPath(database))
	if keyFile != "" {
		acct += "::" + absResolve(runtimex.ExpandPath(keyFile))
	}
	return acct
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

// Set stores password for the given account in the system keyring.
func Set(account, password string) error {
	return backendSet(Service, account, password)
}

// Get returns the password stored for account, or ErrNotFound if none.
func Get(account string) (string, error) {
	return backendGet(Service, account)
}

// Delete removes the stored secret for account. Deleting a missing account is
// not treated as an error.
func Delete(account string) error {
	err := backendDelete(Service, account)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

// Available best-effort probes whether a usable keyring backend exists. A
// lookup of a sentinel account that returns ErrNotFound means the backend is
// reachable; any other error means it is unavailable (no Secret Service
// provider running, unsupported platform, etc.).
func Available() error {
	_, err := backendGet(Service, "__kpass_probe__")
	if err == nil || errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}
