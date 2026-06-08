package config

const (
	DefaultCacheTTL          = 300
	DefaultClipboardTimeout  = 10
	LargeAttachmentWarnBytes = 5 * 1024 * 1024
)

// Config is the resolved runtime configuration for the selected database.
type Config struct {
	Database         string
	PasswordFile     string
	Password         string // pre-resolved (e.g. from another DB); empty means prompt/file
	KeyFile          string
	CacheTTL         int
	NoCache          bool
	UseKeyring       bool // store/read master password in the OS keyring
	BackupKeep       int  // max .bak files to keep (0 = unlimited)
	BackupMaxAgeDays int  // delete .bak older than N days (0 = forever)
}

// Profile is one parsed [databases.<name>] block in the TOML config.
type Profile struct {
	Database         string
	PasswordFile     string
	PasswordDatabase string
	PasswordEntry    string
	KeyFile          string
	CacheTTL         *int
	NoCache          *bool
	UseKeyring       bool // store/read master password in the OS keyring
	BackupKeep       int  // max backups to keep (0 = unlimited)
	BackupMaxAgeDays int  // delete backups older than N days (0 = forever)
}

// FileConfig is the whole parsed TOML file.
type FileConfig struct {
	DefaultDatabase string
	Databases       map[string]Profile
}
