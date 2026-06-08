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

type profileStatus struct {
	Name   string   `json:"name"`
	Status string   `json:"status"`
	Issues []string `json:"issues,omitempty"`
}

func (cmd *DoctorCmd) Run(c *ctx) error {
	fc := c.fileConfig
	if len(fc.Databases) == 0 {
		return cmd.renderEmpty(c)
	}
	statuses := collectDoctorStatuses(c, fc)
	if cmd.JSON {
		return cmd.renderJSON(c, statuses)
	}
	return cmd.renderText(c, statuses)
}

func collectDoctorStatuses(c *ctx, fc config.FileConfig) []profileStatus {
	names := make([]string, 0, len(fc.Databases))
	for n := range fc.Databases {
		names = append(names, n)
	}
	sort.Strings(names)

	out := make([]profileStatus, 0, len(names))
	for _, name := range names {
		ps := profileStatus{Name: name, Status: "OK", Issues: profileIssues(c, fc, name)}
		if len(ps.Issues) > 0 {
			ps.Status = "ERROR"
		}
		out = append(out, ps)
	}
	return out
}

// profileIssues runs every validation check for a single profile and returns
// the human-readable issues found. Empty slice means the profile is healthy.
func profileIssues(c *ctx, fc config.FileConfig, name string) []string {
	p := fc.Databases[name]
	var issues []string
	if !fileExists(p.Database) {
		issues = append(issues, "database not found: "+p.Database)
	}
	if p.PasswordFile != "" && !fileExists(p.PasswordFile) {
		issues = append(issues, "password file not found: "+p.PasswordFile)
	}
	if p.KeyFile != "" && !fileExists(p.KeyFile) {
		issues = append(issues, "key file not found: "+p.KeyFile)
	}
	if p.PasswordDatabase != "" {
		if _, ok := fc.Databases[p.PasswordDatabase]; !ok {
			issues = append(issues, "password database profile not found: "+p.PasswordDatabase)
		}
	}
	if p.UseKeyring {
		if err := keyringAvailableFn(); err != nil {
			issues = append(issues, "system keyring not available: "+err.Error())
		}
	}
	if len(issues) == 0 {
		if _, err := config.ResolveProfile(fc, name, passwordFetcher, c.errw); err != nil {
			issues = append(issues, err.Error())
		}
	}
	return issues
}

func (cmd *DoctorCmd) renderEmpty(c *ctx) error {
	msg := fmt.Sprintf("No database profiles configured in %s", c.configPath)
	if cmd.JSON {
		data, _ := json.MarshalIndent(map[string]any{
			"config":   c.configPath,
			"default":  "",
			"profiles": []any{},
			"error":    msg,
		}, "", "  ")
		fmt.Fprintln(c.out, string(data))
		return nil
	}
	return &UserError{Msg: msg}
}

func (cmd *DoctorCmd) renderJSON(c *ctx, statuses []profileStatus) error {
	type doctorOutput struct {
		Config      string          `json:"config"`
		Default     string          `json:"default"`
		Profiles    []profileStatus `json:"profiles"`
		IssuesCount int             `json:"issues_count"`
	}
	out := doctorOutput{
		Config:   c.configPath,
		Default:  c.fileConfig.DefaultDatabase,
		Profiles: statuses,
	}
	for _, s := range statuses {
		out.IssuesCount += len(s.Issues)
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return &UserError{Msg: err.Error()}
	}
	fmt.Fprintln(c.out, string(data))
	return nil
}

func (cmd *DoctorCmd) renderText(c *ctx, statuses []profileStatus) error {
	fc := c.fileConfig
	fmt.Fprintf(c.out, "%s %s\n", color.Cyan("Config:"), c.configPath)
	fmt.Fprintf(c.out, "%s %s\n", color.Cyan("Default:"), fc.DefaultDatabase)

	total := 0
	for _, s := range statuses {
		p := fc.Databases[s.Name]
		status := color.Green("OK")
		if len(s.Issues) > 0 {
			status = color.Red("ERROR")
		}
		fmt.Fprintf(c.out, "%s %s: database=%s password=%s\n",
			status, color.Bold(s.Name), p.Database, passwordSourceDescription(p))
		for _, issue := range s.Issues {
			fmt.Fprintf(c.out, "  %s %s\n", color.Red("-"), issue)
		}
		total += len(s.Issues)
	}

	if total > 0 {
		fmt.Fprintf(c.out, "%s %s\n", color.Red("Doctor"), color.Red(fmt.Sprintf("found %d issue(s)", total)))
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
