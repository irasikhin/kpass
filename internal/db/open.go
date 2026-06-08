package db

import (
	"errors"
	"fmt"

	"github.com/tobischo/gokeepasslib/v3"

	"github.com/irasikhin/kpass/internal/cache"
	"github.com/irasikhin/kpass/internal/config"
	"github.com/irasikhin/kpass/internal/keyring"
	"github.com/irasikhin/kpass/internal/runtimex"
)

// OpenHook lets tests intercept Open and inject errors/databases without
// touching the filesystem.
var OpenHook func(cfg config.Config) (*DB, error)

// PasswordPrompter is the function used to ask the user for a password.
// Defaults to runtimex.PromptSecret. Tests override via runtimex.PromptHook.
var PasswordPrompter = runtimex.PromptSecret

// Seams for the OS keyring; tests override these because no Secret Service
// backend exists in CI.
var (
	keyringGetFn    = keyring.Get
	keyringSetFn    = keyring.Set
	keyringDeleteFn = keyring.Delete
	keyringAccount  = keyring.Account
)

// Open mirrors Python open_database. Tries cached password (if cache enabled),
// then falls back to file/inline/prompt. On successful open, stores the
// password if cache is enabled.
func Open(cfg config.Config) (*DB, error) {
	if OpenHook != nil {
		return OpenHook(cfg)
	}
	dbPath := runtimex.ExpandPath(cfg.Database)
	info, err := osStatFn(dbPath)
	if err != nil || info.IsDir() {
		return nil, fmt.Errorf("KeePass database not found: %s", dbPath)
	}

	tryOpen := func(password string) (*DB, error) {
		creds, err := buildCreds(password, cfg.KeyFile)
		if err != nil {
			return nil, err
		}
		f, err := osOpenFn(dbPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open KeePass database: %w", err)
		}
		defer f.Close()
		raw := gokeepasslib.NewDatabase()
		raw.Credentials = creds
		if err := gokeepasslib.NewDecoder(f).Decode(raw); err != nil {
			return nil, err
		}
		if err := unlockProtectedFn(raw); err != nil {
			return nil, err
		}
		return &DB{Path: dbPath, KeyFile: cfg.KeyFile, Raw: raw, BackupKeep: cfg.BackupKeep, BackupMaxAgeDays: cfg.BackupMaxAgeDays}, nil
	}

	// Keyring is consulted only when the password would otherwise be prompted:
	// a password_file or a chained password_database value takes precedence and
	// makes the keyring (and the plaintext cache) moot.
	keyringOK := cfg.UseKeyring && cfg.PasswordFile == "" && cfg.Password == ""
	acct := ""
	if keyringOK {
		acct = keyringAccount(dbPath, cfg.KeyFile)
		if cached, err := keyringGetFn(acct); err == nil && cached != "" {
			if db, err := tryOpen(cached); err == nil {
				return db, nil
			}
			_ = keyringDeleteFn(acct) // stale secret; drop and re-prompt
		}
	} else if cache.Enabled(cfg) {
		if cached, _ := cache.Load(dbPath, cfg.KeyFile); cached != "" {
			db, err := tryOpen(cached)
			if err == nil {
				if cfg.CacheTTL > 0 {
					_ = cache.Store(dbPath, cfg.KeyFile, cached, cfg.CacheTTL)
				}
				return db, nil
			}
			_, _ = cache.Clear(dbPath, cfg.KeyFile)
		}
	}

	password, err := obtainPassword(cfg, dbPath)
	if err != nil {
		return nil, err
	}
	db, err := tryOpen(password)
	if err != nil {
		return nil, fmt.Errorf("failed to open KeePass database: %w", err)
	}
	if password != "" {
		switch {
		case keyringOK:
			_ = keyringSetFn(acct, password) // OS-encrypted, persistent
		case cache.Enabled(cfg):
			_ = cache.Store(dbPath, cfg.KeyFile, password, cfg.CacheTTL)
		}
	}
	return db, nil
}

func obtainPassword(cfg config.Config, dbPath string) (string, error) {
	if cfg.Password != "" {
		return cfg.Password, nil
	}
	if cfg.PasswordFile != "" {
		return config.ReadPasswordFile(cfg.PasswordFile)
	}
	return PasswordPrompter(fmt.Sprintf("KeePass password for %s: ", dbPath), false)
}

func buildCreds(password, keyFile string) (*gokeepasslib.DBCredentials, error) {
	expandedKey := runtimex.ExpandPath(keyFile)
	switch {
	case password != "" && expandedKey != "":
		return gokeepasslib.NewPasswordAndKeyCredentials(password, expandedKey)
	case expandedKey != "":
		return gokeepasslib.NewKeyCredentials(expandedKey)
	case password != "":
		return gokeepasslib.NewPasswordCredentials(password), nil
	default:
		return nil, errors.New("no credentials provided")
	}
}
