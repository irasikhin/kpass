package cli

import (
	"encoding/json"
	"fmt"

	"github.com/irasikhin/kpass/internal/color"
	"github.com/irasikhin/kpass/internal/config"
	"github.com/irasikhin/kpass/internal/db"
	"github.com/irasikhin/kpass/internal/keyring"
	"github.com/irasikhin/kpass/internal/runtimex"
)

// Seams for tests; defaults call the real OS keyring / prompt.
var (
	keyringSetFn       = keyring.Set
	keyringGetFn       = keyring.Get
	keyringDeleteFn    = keyring.Delete
	keyringAvailableFn = keyring.Available
	keyringAccountFn   = keyring.Account
	keyringPromptFn    = runtimex.PromptSecret
)

// KeyringCmd groups subcommands for managing OS keyring storage of the master
// password.
type KeyringCmd struct {
	Set    KeyringSetCmd    `cmd:"" help:"Store the master password in the OS keyring and enable use_keyring for the profile."`
	Rm     KeyringRmCmd     `cmd:"" help:"Remove the stored master password and disable use_keyring."`
	Status KeyringStatusCmd `cmd:"" help:"Show keyring backend availability and stored/config state."`
}

// Help returns extended help for keyring.
func (KeyringCmd) Help() string {
	return `Store the KeePass master password in the system keyring
(gnome-keyring / Secret Service, macOS Keychain, Windows Credential Manager)
instead of typing it each time.

  keyring set [@profile]     – prompt, verify, store, and enable use_keyring.
  keyring rm [@profile]      – delete the stored secret and disable use_keyring.
  keyring status [@profile]  – show backend availability and whether stored.

When a profile uses the keyring, the plaintext password cache
($XDG_RUNTIME_DIR/kpass/*.json) is not written. A Secret Service provider must
be running for storage to succeed.`
}

// keyringProfileName returns the profile the keyring command targets: the
// @selector if given, otherwise the configured default.
func (c *ctx) keyringProfileName() string {
	if c.selector != "" {
		return c.selector
	}
	return c.fileConfig.DefaultDatabase
}

// setProfileKeyring flips the use_keyring flag for the named profile in the
// config file. Returns false (without error) when the name does not match a
// configured profile (e.g. a --database override), so the caller can warn.
func setProfileKeyring(c *ctx, name string, on bool) (bool, error) {
	if name == "" {
		return false, nil
	}
	p, ok := c.fileConfig.Databases[name]
	if !ok {
		return false, nil
	}
	if p.UseKeyring == on {
		return true, nil
	}
	p.UseKeyring = on
	updated := config.FileConfig{
		DefaultDatabase: c.fileConfig.DefaultDatabase,
		Databases:       cloneProfiles(c.fileConfig.Databases),
	}
	updated.Databases[name] = p
	if err := config.WriteAtomic(c.configPath, updated); err != nil {
		return false, &UserError{Msg: err.Error()}
	}
	return true, nil
}

type KeyringSetCmd struct{}

func (cmd *KeyringSetCmd) Run(c *ctx) error {
	cfg, err := c.resolveRuntime()
	if err != nil {
		return err
	}
	if cfg.PasswordFile != "" {
		return &UserError{Msg: "Cannot use the keyring with a password_file profile."}
	}
	if cfg.Password != "" {
		return &UserError{Msg: "Cannot use the keyring with a password_database profile (its password comes from another database)."}
	}

	pw, err := keyringPromptFn(fmt.Sprintf("KeePass password for %s: ", cfg.Database), false)
	if err != nil {
		return &UserError{Msg: err.Error()}
	}
	if pw == "" {
		return &UserError{Msg: "Password cannot be empty."}
	}

	// Verify the password actually unlocks the database before storing it.
	verify := cfg
	verify.Password = pw
	verify.PasswordFile = ""
	verify.UseKeyring = false
	verify.NoCache = true
	if _, err := db.Open(verify); err != nil {
		return &UserError{Msg: "Password did not unlock the database; nothing stored."}
	}

	acct := keyringAccountFn(cfg.Database, cfg.KeyFile)
	if err := keyringSetFn(acct, pw); err != nil {
		return &UserError{Msg: "Failed to store password in keyring: " + err.Error()}
	}

	name := c.keyringProfileName()
	enabled, err := setProfileKeyring(c, name, true)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.out, "%s %s\n", color.Green("Stored master password in keyring for"), color.Bold(cfg.Database))
	if enabled {
		fmt.Fprintf(c.out, "%s %s\n", color.Faint("Enabled use_keyring for profile"), color.Bold(name))
	} else {
		fmt.Fprintln(c.out, color.Yellow("Note: no matching profile in config; use_keyring was not enabled."))
	}
	return nil
}

type KeyringRmCmd struct{}

func (cmd *KeyringRmCmd) Run(c *ctx) error {
	cfg, err := c.resolveRuntime()
	if err != nil {
		return err
	}
	acct := keyringAccountFn(cfg.Database, cfg.KeyFile)
	if err := keyringDeleteFn(acct); err != nil {
		return &UserError{Msg: "Failed to remove password from keyring: " + err.Error()}
	}
	name := c.keyringProfileName()
	disabled, err := setProfileKeyring(c, name, false)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.out, "%s %s\n", color.Green("Removed master password from keyring for"), color.Bold(cfg.Database))
	if disabled {
		fmt.Fprintf(c.out, "%s %s\n", color.Faint("Disabled use_keyring for profile"), color.Bold(name))
	}
	return nil
}

type KeyringStatusCmd struct {
	JSON bool `help:"Output as JSON."`
}

type keyringStatusJSON struct {
	Profile    string `json:"profile"`
	Database   string `json:"database"`
	Available  bool   `json:"backend_available"`
	Stored     bool   `json:"password_stored"`
	UseKeyring bool   `json:"use_keyring"`
	Error      string `json:"error,omitempty"`
}

func (cmd *KeyringStatusCmd) Run(c *ctx) error {
	cfg, err := c.resolveRuntime()
	if err != nil {
		return err
	}
	name := c.keyringProfileName()
	st := keyringStatusJSON{
		Profile:    name,
		Database:   cfg.Database,
		UseKeyring: c.fileConfig.Databases[name].UseKeyring,
	}
	if availErr := keyringAvailableFn(); availErr != nil {
		st.Error = availErr.Error()
	} else {
		st.Available = true
		acct := keyringAccountFn(cfg.Database, cfg.KeyFile)
		if pw, err := keyringGetFn(acct); err == nil && pw != "" {
			st.Stored = true
		}
	}

	if cmd.JSON {
		data, err := json.MarshalIndent(st, "", "  ")
		if err != nil {
			return &UserError{Msg: err.Error()}
		}
		fmt.Fprintln(c.out, string(data))
		return nil
	}

	fmt.Fprintf(c.out, "%s %s\n", color.Cyan("Database:"), st.Database)
	if st.Available {
		fmt.Fprintf(c.out, "%s %s\n", color.Cyan("Backend:"), color.Green("available"))
	} else {
		fmt.Fprintf(c.out, "%s %s (%s)\n", color.Cyan("Backend:"), color.Red("unavailable"), color.Faint(st.Error))
	}
	stored := color.Yellow("no")
	if st.Stored {
		stored = color.Green("yes")
	}
	fmt.Fprintf(c.out, "%s %s\n", color.Cyan("Password stored:"), stored)
	fmt.Fprintf(c.out, "%s %t\n", color.Cyan("use_keyring (config):"), st.UseKeyring)
	return nil
}
