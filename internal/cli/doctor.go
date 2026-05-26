package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/irasikhin/kpass/internal/color"
	"github.com/irasikhin/kpass/internal/config"
	"github.com/irasikhin/kpass/internal/runtimex"
)

// DoctorCmd validates the loaded config and every configured database profile.
type DoctorCmd struct {
	JSON bool `help:"Output as JSON."`
}

func (cmd *DoctorCmd) Run(c *ctx) error {
	fc := c.fileConfig
	if len(fc.Databases) == 0 {
		if cmd.JSON {
			data, _ := json.MarshalIndent(map[string]any{
				"config":   c.configPath,
				"default":  "",
				"profiles": []any{},
				"error":    fmt.Sprintf("No database profiles configured in %s", c.configPath),
			}, "", "  ")
			fmt.Fprintln(c.out, string(data))
			return nil
		}
		return &UserError{Msg: fmt.Sprintf("No database profiles configured in %s", c.configPath)}
	}

	if cmd.JSON {
		type profileStatus struct {
			Name   string   `json:"name"`
			Status string   `json:"status"`
			Issues []string `json:"issues,omitempty"`
		}
		type doctorOutput struct {
			Config      string          `json:"config"`
			Default     string          `json:"default"`
			Profiles    []profileStatus `json:"profiles"`
			IssuesCount int             `json:"issues_count"`
		}
		names := make([]string, 0, len(fc.Databases))
		for n := range fc.Databases {
			names = append(names, n)
		}
		sort.Strings(names)

		var out doctorOutput
		out.Config = c.configPath
		out.Default = fc.DefaultDatabase
		for _, name := range names {
			p := fc.Databases[name]
			ps := profileStatus{Name: name, Status: "OK"}
			if !fileExists(p.Database) {
				ps.Issues = append(ps.Issues, "database not found: "+p.Database)
			}
			if p.PasswordFile != "" && !fileExists(p.PasswordFile) {
				ps.Issues = append(ps.Issues, "password file not found: "+p.PasswordFile)
			}
			if p.KeyFile != "" && !fileExists(p.KeyFile) {
				ps.Issues = append(ps.Issues, "key file not found: "+p.KeyFile)
			}
			if p.PasswordDatabase != "" {
				if _, ok := fc.Databases[p.PasswordDatabase]; !ok {
					ps.Issues = append(ps.Issues, "password database profile not found: "+p.PasswordDatabase)
				}
			}
			if len(ps.Issues) == 0 {
				if _, err := config.ResolveProfile(fc, name, passwordFetcher, c.errw); err != nil {
					ps.Issues = append(ps.Issues, err.Error())
				}
			}
			if len(ps.Issues) > 0 {
				ps.Status = "ERROR"
				out.IssuesCount += len(ps.Issues)
			}
			out.Profiles = append(out.Profiles, ps)
		}
		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return &UserError{Msg: err.Error()}
		}
		fmt.Fprintln(c.out, string(data))
		return nil
	}

	fmt.Fprintf(c.out, "%s %s\n", color.Cyan("Config:"), c.configPath)
	fmt.Fprintf(c.out, "%s %s\n", color.Cyan("Default:"), fc.DefaultDatabase)

	names := make([]string, 0, len(fc.Databases))
	for n := range fc.Databases {
		names = append(names, n)
	}
	sort.Strings(names)

	issues := 0
	for _, name := range names {
		p := fc.Databases[name]
		var profileIssues []string
		if !fileExists(p.Database) {
			profileIssues = append(profileIssues, "database not found: "+p.Database)
		}
		if p.PasswordFile != "" && !fileExists(p.PasswordFile) {
			profileIssues = append(profileIssues, "password file not found: "+p.PasswordFile)
		}
		if p.KeyFile != "" && !fileExists(p.KeyFile) {
			profileIssues = append(profileIssues, "key file not found: "+p.KeyFile)
		}
		if p.PasswordDatabase != "" {
			if _, ok := fc.Databases[p.PasswordDatabase]; !ok {
				profileIssues = append(profileIssues, "password database profile not found: "+p.PasswordDatabase)
			}
		}
		if len(profileIssues) == 0 {
			if _, err := config.ResolveProfile(fc, name, passwordFetcher, c.errw); err != nil {
				profileIssues = append(profileIssues, err.Error())
			}
		}
		status := color.Green("OK")
		if len(profileIssues) > 0 {
			status = color.Red("ERROR")
		}
		fmt.Fprintf(c.out, "%s %s: database=%s password=%s\n", status, color.Bold(name), p.Database, passwordSourceDescription(p))
		for _, issue := range profileIssues {
			fmt.Fprintf(c.out, "  %s %s\n", color.Red("-"), issue)
		}
		issues += len(profileIssues)
	}

	if issues > 0 {
		fmt.Fprintf(c.out, "%s %s\n", color.Red("Doctor"), color.Red(fmt.Sprintf("found %d issue(s)", issues)))
		return &UserError{Msg: ""}
	}
	fmt.Fprintln(c.out, color.Green("Doctor found no issues"))
	return nil
}

func passwordSourceDescription(p config.Profile) string {
	if p.PasswordFile != "" {
		return "file:" + p.PasswordFile
	}
	if p.PasswordDatabase != "" && p.PasswordEntry != "" {
		return "profile:" + p.PasswordDatabase + ":" + p.PasswordEntry
	}
	return "prompt"
}

func fileExists(p string) bool {
	if p == "" {
		return false
	}
	info, err := os.Stat(runtimex.ExpandPath(p))
	return err == nil && !info.IsDir()
}
