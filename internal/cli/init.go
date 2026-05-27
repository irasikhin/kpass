package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/irasikhin/kpass/internal/config"
	"github.com/irasikhin/kpass/internal/db"
	"github.com/irasikhin/kpass/internal/runtimex"
)

// InitCmd bootstraps a new KeePass database and optionally writes a config
// profile so the user is ready to use kpass immediately.
type InitCmd struct {
	Path     string `help:"Path for the new database file." placeholder:"PATH" default:"~/.local/share/kpass/default.kdbx"`
	Force    bool   `short:"f" help:"Overwrite an existing database."`
	NoConfig bool   `help:"Skip writing a config profile — only create the database."`
	JSON     bool   `help:"Output as JSON."`
}

func (cmd *InitCmd) Run(c *ctx) error {
	dbPath := runtimex.ExpandPath(cmd.Path)
	if dbPath == "" {
		return &UserError{Msg: "Database path cannot be empty."}
	}

	if !cmd.Force {
		if _, err := os.Stat(dbPath); err == nil {
			return &UserError{Msg: fmt.Sprintf("Database already exists: %s (use --force to overwrite)", dbPath)}
		}
	}

	// Prompt for master password (twice for confirmation).
	password, err := runtimex.PromptSecret("New master password: ", false)
	if err != nil {
		return err
	}
	confirm, err := runtimex.PromptSecret("Confirm master password: ", false)
	if err != nil {
		return err
	}
	if password != confirm {
		return &UserError{Msg: "Passwords do not match."}
	}

	// Create the database.
	if err := db.Create(dbPath, password, ""); err != nil {
		return &UserError{Msg: err.Error()}
	}

	// Write config if requested.
	profileWritten := false
	configPath := ""
	if !cmd.NoConfig {
		configPath, err = ensureConfig(dbPath, "")
		if err != nil {
			// DB was created; warn but don't fail.
			fmt.Fprintf(c.errw, "Warning: database created but config not written: %v\n", err)
		} else {
			profileWritten = true
		}
	}

	if cmd.JSON {
		data := map[string]any{
			"status":   "ok",
			"database": dbPath,
		}
		if profileWritten {
			data["config"] = configPath
		}
		out, _ := json.Marshal(data)
		fmt.Fprintln(c.out, string(out))
		return nil
	}

	fmt.Fprintf(c.out, "Created %s\n", dbPath)
	if profileWritten {
		fmt.Fprintf(c.out, "Config written to %s\n", configPath)
	}
	fmt.Fprintln(c.out, "\nTry:")
	fmt.Fprintln(c.out, "  kpass ls")
	fmt.Fprintln(c.out, "  kpass insert work/email")
	return nil
}

// ensureConfig loads the existing config (or creates a new one) and adds a
// "default" profile pointing to dbPath. Returns the config path written.
func ensureConfig(dbPath, keyFile string) (string, error) {
	cfgPath := runtimex.ConfigFilePath("")
	dir := filepath.Dir(cfgPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("cannot create config directory: %w", err)
	}

	// Try to load existing config; if missing or broken, start fresh.
	fc := config.FileConfig{
		DefaultDatabase: "default",
		Databases:       map[string]config.Profile{},
	}
	if existing, _, err := config.Load(cfgPath); err == nil {
		fc = existing
	}

	// If "default" profile does not exist, add it.
	if _, ok := fc.Databases["default"]; !ok {
		fc.DefaultDatabase = "default"
		fc.Databases["default"] = config.Profile{
			Database: dbPath,
			KeyFile:  keyFile,
		}
	}

	if err := config.WriteAtomic(cfgPath, fc); err != nil {
		return "", fmt.Errorf("cannot write config: %w", err)
	}
	return cfgPath, nil
}
