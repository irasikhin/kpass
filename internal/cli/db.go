package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/irasikhin/kpass/internal/color"
	"github.com/irasikhin/kpass/internal/config"
	"github.com/irasikhin/kpass/internal/runtimex"
)

// DbCmd groups subcommands for managing profiles inside the TOML config.
type DbCmd struct {
	Ls      DbLsCmd      `cmd:"" help:"List configured database profiles."`
	Add     DbAddCmd     `cmd:"" help:"Register a new database profile in the config."`
	Rm      DbRmCmd      `cmd:"" help:"Remove a database profile."`
	Default DbDefaultCmd `cmd:"" help:"Show or change the default profile."`
}

type DbLsCmd struct {
	JSON bool `help:"Output as JSON."`
}

func (cmd *DbLsCmd) Run(c *ctx) error {
	if cmd.JSON {
		type profileJSON struct {
			Name         string `json:"name"`
			Database     string `json:"database"`
			PasswordFile string `json:"password_file,omitempty"`
			KeyFile      string `json:"key_file,omitempty"`
			PasswordSrc  string `json:"password_source,omitempty"`
		}
		type dbListJSON struct {
			Default  string        `json:"default"`
			Profiles []profileJSON `json:"profiles"`
		}
		output := dbListJSON{Default: c.fileConfig.DefaultDatabase}
		names := sortedKeys(c.fileConfig.Databases)
		for _, n := range names {
			p := c.fileConfig.Databases[n]
			pj := profileJSON{
				Name:         n,
				Database:     p.Database,
				PasswordFile: p.PasswordFile,
				KeyFile:      p.KeyFile,
			}
			if p.PasswordDatabase != "" {
				pj.PasswordSrc = p.PasswordDatabase + ":" + p.PasswordEntry
			}
			output.Profiles = append(output.Profiles, pj)
		}
		data, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return &UserError{Msg: err.Error()}
		}
		fmt.Fprintln(c.out, string(data))
		return nil
	}

	if len(c.fileConfig.Databases) == 0 {
		fmt.Fprintln(c.out, color.Yellow("No database profiles configured"))
		return nil
	}
	if c.fileConfig.DefaultDatabase != "" {
		fmt.Fprintf(c.out, "%s %s\n", color.Cyan("Default:"), c.fileConfig.DefaultDatabase)
	}
	names := sortedKeys(c.fileConfig.Databases)
	for _, n := range names {
		p := c.fileConfig.Databases[n]
		marker := " "
		if n == c.fileConfig.DefaultDatabase {
			marker = color.Green("*")
		}
		fmt.Fprintf(c.out, "%s %s: %s [%s]\n", marker, color.Bold(n), p.Database, color.Faint(passwordSourceDescription(p)))
	}
	return nil
}

// DbAddCmd registers a new profile. The password / key / cache-TTL options are
// read from the root globals (they share the same flag names) to avoid kong's
// flag-name collision check: `kpass db add NAME PATH --password-file FOO` is
// effectively `kpass --password-file FOO db add NAME PATH`.
type DbAddCmd struct {
	Name             string `arg:"" help:"Profile name (e.g. work)."`
	DatabasePath     string `arg:"" help:"Path to the KDBX file."`
	PasswordDatabase string `help:"Source profile name for password lookup." placeholder:"NAME"`
	PasswordEntry    string `help:"Source entry path for password lookup." placeholder:"PATH"`
	Default          bool   `help:"Make this profile the default."`
}

func (cmd *DbAddCmd) Run(c *ctx) error {
	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		return &UserError{Msg: "Database profile name cannot be empty."}
	}
	if _, exists := c.fileConfig.Databases[name]; exists {
		return &UserError{Msg: fmt.Sprintf("Database profile already exists: %s", name)}
	}

	passwordDatabase := strings.TrimSpace(cmd.PasswordDatabase)
	if passwordDatabase != "" {
		if passwordDatabase == name {
			return &UserError{Msg: "Database profile cannot use itself as a password source."}
		}
		if _, ok := c.fileConfig.Databases[passwordDatabase]; !ok {
			return &UserError{Msg: fmt.Sprintf("Password database profile not found: %s", passwordDatabase)}
		}
	}

	profile := config.Profile{
		Database:         runtimex.ExpandPath(cmd.DatabasePath),
		PasswordFile:     runtimex.ExpandPath(c.gf.passwordFile),
		PasswordDatabase: passwordDatabase,
		PasswordEntry:    cmd.PasswordEntry,
		KeyFile:          runtimex.ExpandPath(c.gf.keyFile),
	}
	if c.gf.cacheTTL != nil {
		v := *c.gf.cacheTTL
		profile.CacheTTL = &v
	}
	if c.gf.noCache != nil {
		t := *c.gf.noCache
		profile.NoCache = &t
	}
	if profile.PasswordFile != "" && (profile.PasswordDatabase != "" || profile.PasswordEntry != "") {
		return &UserError{Msg: fmt.Sprintf("KPass database profile '%s' cannot combine 'password_file' with password lookup from another database.", name)}
	}
	if (profile.PasswordDatabase == "") != (profile.PasswordEntry == "") {
		return &UserError{Msg: fmt.Sprintf("KPass database profile '%s' must set both 'password_database' and 'password_entry' together.", name)}
	}

	updated := config.FileConfig{
		DefaultDatabase: c.fileConfig.DefaultDatabase,
		Databases:       cloneProfiles(c.fileConfig.Databases),
	}
	updated.Databases[name] = profile
	if updated.DefaultDatabase == "" || cmd.Default {
		updated.DefaultDatabase = name
	}
	if err := config.WriteAtomic(c.configPath, updated); err != nil {
		return &UserError{Msg: err.Error()}
	}
	fmt.Fprintf(c.out, "%s %s\n", color.Green("Added database profile"), color.Bold(name))
	if updated.DefaultDatabase == name {
		fmt.Fprintf(c.out, "%s %s\n", color.Faint("Default database profile is now"), color.Bold(name))
	}
	return nil
}

type DbRmCmd struct {
	Name string `arg:"" help:"Profile name to remove."`
}

func (cmd *DbRmCmd) Run(c *ctx) error {
	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		return &UserError{Msg: "Database profile name cannot be empty."}
	}
	if _, ok := c.fileConfig.Databases[name]; !ok {
		return &UserError{Msg: fmt.Sprintf("Database profile not found: %s", name)}
	}
	if len(c.fileConfig.Databases) == 1 {
		return &UserError{Msg: "Cannot remove the last database profile."}
	}
	updated := config.FileConfig{
		DefaultDatabase: c.fileConfig.DefaultDatabase,
		Databases:       cloneProfiles(c.fileConfig.Databases),
	}
	delete(updated.Databases, name)
	var switched string
	if updated.DefaultDatabase == name {
		names := sortedKeys(updated.Databases)
		switched = names[0]
		updated.DefaultDatabase = switched
	}
	for profileName, p := range updated.Databases {
		if p.PasswordDatabase == name {
			return &UserError{Msg: fmt.Sprintf("Cannot remove database profile '%s' because '%s' depends on it for password lookup.", name, profileName)}
		}
	}
	if err := config.WriteAtomic(c.configPath, updated); err != nil {
		return &UserError{Msg: err.Error()}
	}
	fmt.Fprintf(c.out, "%s %s\n", color.Green("Removed database profile"), color.Bold(name))
	if switched != "" {
		fmt.Fprintf(c.out, "%s %s\n", color.Faint("Default database profile is now"), color.Bold(switched))
	}
	return nil
}

type DbDefaultCmd struct {
	Name string `arg:"" optional:"" help:"Profile name to make default (omit to print current)."`
}

func (cmd *DbDefaultCmd) Run(c *ctx) error {
	if cmd.Name == "" {
		if c.fileConfig.DefaultDatabase == "" {
			return &UserError{Msg: "No default database configured."}
		}
		fmt.Fprintln(c.out, c.fileConfig.DefaultDatabase)
		return nil
	}
	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		return &UserError{Msg: "Database profile name cannot be empty."}
	}
	if _, ok := c.fileConfig.Databases[name]; !ok {
		return &UserError{Msg: fmt.Sprintf("Database profile not found: %s", name)}
	}
	updated := config.FileConfig{
		DefaultDatabase: name,
		Databases:       cloneProfiles(c.fileConfig.Databases),
	}
	if err := config.WriteAtomic(c.configPath, updated); err != nil {
		return &UserError{Msg: err.Error()}
	}
	fmt.Fprintf(c.out, "%s %s\n", color.Faint("Default database profile is now"), color.Bold(name))
	return nil
}

func sortedKeys(m map[string]config.Profile) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func cloneProfiles(in map[string]config.Profile) map[string]config.Profile {
	out := make(map[string]config.Profile, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
