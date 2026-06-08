package cli

import (
	"strings"

	"github.com/alecthomas/kong"
)

// kpassCLI is the root kong struct. Global flags live at this level (kong
// treats them as persistent); subcommands are nested via `cmd:""`.
type kpassCLI struct {
	// --- global flags ---
	Config       string           `help:"Path to config file (default ~/.config/kpass/config.toml)." placeholder:"PATH" short:"c"`
	Database     string           `help:"Override default database path." placeholder:"PATH" short:"d"`
	PasswordFile string           `help:"Read master password from file." placeholder:"PATH" short:"p"`
	KeyFile      string           `help:"Composite key file." placeholder:"PATH" short:"k"`
	CacheTTL     int              `help:"Master-password cache TTL in seconds." placeholder:"N" aliases:"session-ttl" default:"-1"`
	NoCache      bool             `help:"Disable master-password cache." aliases:"no-session"`
	UseKeyring   bool             `help:"Read/store the master password in the OS keyring."`
	NoKeyring    bool             `help:"Disable the OS keyring for this invocation."`
	NoColor      bool             `help:"Disable colored output." short:"C"`
	Yes          bool             `help:"Auto-answer yes to all confirmation prompts." short:"y"`
	Version      kong.VersionFlag `short:"V" help:"Print version and exit."`

	// --- subcommands ---
	Ls         LsCmd         `cmd:"" help:"List entries as a tree."`
	Search     SearchCmd     `cmd:"" help:"Find entries by name, path, or fields."`
	Get        GetCmd        `cmd:"" help:"Show one entry or one field."`
	Copy       CopyCmd       `cmd:"" help:"Copy a field to the clipboard."`
	Attach     AttachCmd     `cmd:"" help:"Manage entry attachments."`
	Pick       PickCmd       `cmd:"" help:"Pick an entry interactively (fzf)."`
	Insert     InsertCmd     `cmd:"" help:"Create a new entry."`
	Edit       EditCmd       `cmd:"" help:"Edit an existing entry."`
	Generate   GenerateCmd   `cmd:"" help:"Generate and store a new password."`
	Remove     RemoveCmd     `cmd:"" help:"Delete an entry."`
	Move       MoveCmd       `cmd:"" help:"Move or rename an entry."`
	Duplicate  DuplicateCmd  `cmd:"" help:"Duplicate an entry to a new path."`
	Mkdir      MkdirCmd      `cmd:"" help:"Create a group path."`
	Merge      MergeCmd      `cmd:"" help:"Import entries from another database."`
	Doctor     DoctorCmd     `cmd:"" help:"Validate config and profiles."`
	Audit      AuditCmd      `cmd:"" help:"Check database for security issues."`
	Open       OpenCmd       `cmd:"" help:"Open an entry's URL in the browser."`
	Clean      CleanCmd      `cmd:"" help:"Remove empty groups from the database."`
	Export     ExportCmd     `cmd:"" help:"Export entries to JSON or CSV."`
	Import     ImportCmd     `cmd:"" help:"Import entries from JSON or CSV."`
	ImportPass ImportPassCmd `cmd:"" name:"import-pass" help:"Import entries from a pass(1) password store."`
	Tags       TagsCmd       `cmd:"" help:"List all unique tags with entry counts."`
	Tag        TagCmd        `cmd:"" help:"Bulk tag operations on entries (add, remove, rename)."`
	Combine    CombineCmd    `cmd:"" help:"Merge two entries (e.g., attach OTP-only entry into existing login)."`
	History    HistoryCmd    `cmd:"" help:"View or restore entry history versions."`
	Undo       UndoCmd       `cmd:"" help:"Restore database from a backup or list backups."`
	Init       InitCmd       `cmd:"" help:"Initialize a new KeePass database and config."`
	Stats      StatsCmd      `cmd:"" help:"Show database statistics."`
	Db         DbCmd         `cmd:"" help:"Manage database profiles in config."`
	Keyring    KeyringCmd    `cmd:"" help:"Manage OS keyring storage of the master password."`
	Completion CompletionCmd `cmd:"" help:"Generate shell completion script."`
	Complete   CompleteCmd   `cmd:"" name:"__complete" hidden:"" help:"Internal helper for shell completion."`
}

// globalFlags extracts the database-affecting flags into the runtime form.
func (cli *kpassCLI) globalFlags() globalFlags {
	g := globalFlags{
		database:     cli.Database,
		passwordFile: cli.PasswordFile,
		keyFile:      cli.KeyFile,
		yes:          cli.Yes,
	}
	// CacheTTL default of -1 means "not set" (kong has no zero/unset
	// distinction for ints without a *int kludge).
	if cli.CacheTTL >= 0 {
		v := cli.CacheTTL
		g.cacheTTL = &v
	}
	if cli.NoCache {
		t := true
		g.noCache = &t
	}
	switch {
	case cli.UseKeyring:
		t := true
		g.useKeyring = &t
	case cli.NoKeyring:
		f := false
		g.useKeyring = &f
	}
	return g
}

// commandsWithSelector lists kong commands that accept the `@profile`
// selector. Maintenance commands (doctor, db) don't.
var commandsWithSelector = map[string]bool{
	"ls":             true,
	"search":         true,
	"get":            true,
	"copy":           true,
	"pick":           true,
	"attach":         true,
	"attach ls":      true,
	"attach add":     true,
	"attach remove":  true,
	"attach extract": true,
	"insert":         true,
	"edit":           true,
	"generate":       true,
	"remove":         true,
	"move":           true,
	"duplicate":      true,
	"mkdir":          true,
	"merge":          true,
	"audit":          true,
	"open":           true,
	"clean":          true,
	"export":         true,
	"import":         true,
	"import-pass":    true,
	"tags":           true,
	"tag":            true,
	"tag add":        true,
	"tag remove":     true,
	"tag rename":     true,
	"combine":        true,
	"history":        true,
	"undo":           true,
	"stats":          true,
	"keyring":        true,
}

// normalizeFields strips empty entries from a field slice; the enum tag on
// kong fields already rejects invalid choices.
func normalizeFields(fields []string) []string {
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		out = append(out, f)
	}
	return out
}
