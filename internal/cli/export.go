package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/irasikhin/kpass/internal/color"
	"github.com/irasikhin/kpass/internal/db"
	"github.com/irasikhin/kpass/internal/runtimex"
)

const exportHelp = `JSON output:

  [
    {
      "path": "internet/email",
      "title": "email",
      "username": "alice",
      "password": "pw-email",
      "url": "https://mail.example.com",
      "notes": "personal",
      "otp": "",
      "tags": [],
      "custom_fields": {}
    }
  ]

CSV header: path,title,username,password,url,notes,otp,…`

// ExportCmd exports entries to stdout in the given format.
type ExportCmd struct {
	Format string   `short:"o" default:"json" enum:"json,csv" help:"Output format."`
	Output string   `help:"Write to file instead of stdout." placeholder:"PATH"`
	Force  bool     `short:"f" help:"Overwrite an existing output file."`
	Tag    []string `help:"Filter by tag (AND). Repeatable."`
	TagAny []string `name:"tag-any" help:"Filter by tag (OR — at least one matches). Repeatable."`
}

// Help returns detailed help for the export command.
func (ExportCmd) Help() string { return exportHelp }

func (cmd *ExportCmd) Run(c *ctx) error {
	if err := c.openDatabase(); err != nil {
		return err
	}

	entries := c.db.SortedEntries()

	// Apply tag filter.
	if len(cmd.Tag) > 0 || len(cmd.TagAny) > 0 {
		filtered := entries[:0]
		for _, e := range entries {
			if matchTagFilter(e, cmd.Tag, cmd.TagAny) {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	if len(entries) == 0 {
		return &UserError{Msg: "No entries to export."}
	}

	var out = c.out
	var f *os.File
	if cmd.Output != "" {
		outputPath := runtimex.ExpandPath(cmd.Output)
		if _, err := os.Stat(outputPath); err == nil && !cmd.Force {
			return &UserError{Msg: fmt.Sprintf("Output file already exists: %s. Use --force to overwrite.", outputPath)}
		}
		var err error
		f, err = os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			return &UserError{Msg: err.Error()}
		}
		defer f.Close()
		if err := f.Chmod(0o600); err != nil {
			return &UserError{Msg: err.Error()}
		}
		out = f
	}

	switch cmd.Format {
	case "csv":
		return exportCSV(out, entries)
	default:
		return exportJSON(out, entries)
	}
}

func exportJSON(w io.Writer, entries []*db.Entry) error {
	items := make([]db.EntryJSON, len(entries))
	for i, e := range entries {
		items[i] = e.ToJSON()
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(items)
}

func exportCSV(w io.Writer, entries []*db.Entry) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"path", "title", "username", "password", "url", "notes", "otp"}); err != nil {
		return err
	}
	for _, e := range entries {
		otp := ""
		if e.OtpURI() != "" {
			otp = e.OtpURI()
		}
		row := []string{
			e.DisplayPath(),
			e.Raw().GetTitle(),
			e.Raw().GetContent("UserName"),
			e.Raw().GetPassword(),
			e.Raw().GetContent("URL"),
			e.Raw().GetContent("Notes"),
			otp,
		}
		for k, v := range e.CustomFields() {
			row = append(row, fmt.Sprintf("%s=%s", k, v))
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

const importHelp = `Expected JSON structure (same as export output):

  [
    {
      "path": "internet/email",
      "title": "email",
      "username": "alice",
      "password": "pw-email",
      "url": "https://mail.example.com",
      "notes": "personal",
      "otp": "otpauth://...",
      "custom_fields": {"env": "prod"}
    }
  ]

Expected CSV header: path,title,username,password,url,notes,otp`

// ImportCmd imports entries from a file into the database.
type ImportCmd struct {
	Source     string `arg:"" help:"Source file to import (CSV or JSON)."`
	Format     string `short:"o" default:"json" enum:"json,csv" help:"Input format."`
	OnConflict string `default:"skip" enum:"error,skip,overwrite,rename" help:"On path conflict: error, skip, overwrite, rename."`
	Force      bool   `short:"f" help:"Skip confirmation prompt."`
}

// Help returns detailed help for the import command.
func (ImportCmd) Help() string { return importHelp }

func (cmd *ImportCmd) Run(c *ctx) error {
	if err := c.openDatabase(); err != nil {
		return err
	}

	data, err := os.ReadFile(cmd.Source)
	if err != nil {
		return &UserError{Msg: err.Error()}
	}

	var imported []importEntry
	switch cmd.Format {
	case "csv":
		imported, err = parseCSVImport(data)
	default:
		imported, err = parseJSONImport(data)
	}
	if err != nil {
		return &UserError{Msg: err.Error()}
	}

	return applyImport(c, imported, cmd.OnConflict, cmd.Force)
}

// applyImport is the shared write path for import-style commands: prints the
// list to be imported, prompts for confirmation unless force, then creates or
// updates entries per the conflict policy and saves.
func applyImport(c *ctx, imported []importEntry, onConflict string, force bool) error {
	if len(imported) == 0 {
		return &UserError{Msg: "No entries found in import file."}
	}

	if !force {
		details := make([]string, len(imported))
		for i, ie := range imported {
			details[i] = ie.Path
		}
		ok, err := confirm(c, fmt.Sprintf("Import %d entries", len(imported)), details...)
		if err != nil {
			return &UserError{Msg: err.Error()}
		}
		if !ok {
			return &UserError{Msg: "Aborted."}
		}
	}

	stats := struct{ created, updated, skipped int }{}
	for _, ie := range imported {
		existing := c.db.FindEntryByExactPath(ie.Path)
		if existing != nil {
			switch onConflict {
			case "error":
				return &UserError{Msg: fmt.Sprintf("Entry already exists: %s", ie.Path)}
			case "skip":
				stats.skipped++
				continue
			case "overwrite":
				applyImportFields(existing, ie)
				stats.updated++
			case "rename":
				ie.Path = findAvailablePath(c.db, ie.Path)
				ie.Title = lastSegment(ie.Path)
				existing = nil
			}
		}
		if existing == nil {
			entry := createEntryFromImport(c.db, ie)
			applyImportFields(entry, ie)
			stats.created++
		}
	}
	if err := c.db.Save(); err != nil {
		return &UserError{Msg: err.Error()}
	}

	fmt.Fprintf(c.out, "%s %d, %s %d, %s %d\n",
		color.Green("Created:"), stats.created,
		color.Yellow("Updated:"), stats.updated,
		color.Faint("Skipped:"), stats.skipped)
	return nil
}

type importEntry struct {
	Path     string
	Title    string
	Username string
	Password string
	URL      string
	Notes    string
	OTP      string
	Custom   map[string]string
}

func parseJSONImport(data []byte) ([]importEntry, error) {
	var raw []map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON: %v", err)
	}
	var out []importEntry
	for _, m := range raw {
		ie := importEntry{
			Path:     strVal(m, "path"),
			Title:    strVal(m, "title"),
			Username: strVal(m, "username"),
			Password: strVal(m, "password"),
			URL:      strVal(m, "url"),
			Notes:    strVal(m, "notes"),
			OTP:      strVal(m, "otp"),
		}
		if cf, ok := m["custom_fields"].(map[string]any); ok {
			ie.Custom = make(map[string]string)
			for k, v := range cf {
				ie.Custom[k] = fmt.Sprint(v)
			}
		}
		if ie.Path == "" {
			return nil, fmt.Errorf("entry missing 'path' field")
		}
		out = append(out, ie)
	}
	return out, nil
}

func parseCSVImport(data []byte) ([]importEntry, error) {
	r := csv.NewReader(strings.NewReader(string(data)))
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("invalid CSV: %v", err)
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("CSV must have a header row plus data")
	}
	header := records[0]
	colIndex := map[string]int{}
	for i, h := range header {
		colIndex[strings.ToLower(strings.TrimSpace(h))] = i
	}
	pathIdx, ok := colIndex["path"]
	if !ok {
		return nil, fmt.Errorf("CSV must have a 'path' column")
	}
	get := func(row []string, name string) string {
		if idx, ok := colIndex[name]; ok && idx < len(row) {
			return row[idx]
		}
		return ""
	}
	var out []importEntry
	for _, row := range records[1:] {
		ie := importEntry{
			Path:     row[pathIdx],
			Title:    get(row, "title"),
			Username: get(row, "username"),
			Password: get(row, "password"),
			URL:      get(row, "url"),
			Notes:    get(row, "notes"),
			OTP:      get(row, "otp"),
			Custom:   map[string]string{},
		}
		for k, idx := range colIndex {
			if knownCSV[k] {
				continue
			}
			if idx < len(row) && row[idx] != "" {
				if strings.Contains(row[idx], "=") {
					parts := strings.SplitN(row[idx], "=", 2)
					ie.Custom[parts[0]] = parts[1]
				} else {
					ie.Custom[k] = row[idx]
				}
			}
		}
		if ie.Path == "" {
			continue
		}
		out = append(out, ie)
	}
	return out, nil
}

var knownCSV = map[string]bool{
	"path": true, "title": true, "username": true, "password": true,
	"url": true, "notes": true, "otp": true,
}

func strVal(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		return fmt.Sprint(v)
	}
	return ""
}

func applyImportFields(entry *db.Entry, ie importEntry) {
	if ie.Title != "" {
		entry.SetTitle(ie.Title)
	}
	if ie.Username != "" {
		entry.SetField("username", ie.Username)
	}
	if ie.Password != "" {
		entry.SetField("password", ie.Password)
	}
	if ie.URL != "" {
		entry.SetField("url", ie.URL)
	}
	if ie.Notes != "" {
		entry.SetField("notes", ie.Notes)
	}
	if ie.OTP != "" {
		entry.SetField("otp", ie.OTP)
	}
	for k, v := range ie.Custom {
		entry.SetField(k, v)
	}
}

func createEntryFromImport(d *db.DB, ie importEntry) *db.Entry {
	parts := runtimex.SplitPath(ie.Path)
	title := parts[len(parts)-1]
	if ie.Title != "" {
		title = ie.Title
	}
	parent := d.EnsureGroup(runtimex.JoinPath(parts[:len(parts)-1]))
	return d.CreateEntry(parent, title, ie.Username, ie.Password, ie.URL, ie.Notes, ie.OTP)
}

func findAvailablePath(d *db.DB, base string) string {
	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s (%d)", base, i)
		if d.FindEntryByExactPath(candidate) == nil {
			return candidate
		}
	}
}
