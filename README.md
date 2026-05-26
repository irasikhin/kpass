# kpass

[![CI](https://github.com/irasikhin/kpass/actions/workflows/ci.yml/badge.svg)](https://github.com/irasikhin/kpass/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Entry-first CLI for [KeePass](https://keepass.info/) databases —
another CLI for KeePass, built vibe-coding style: human designs, AI drives.

> ⚠️ **Alpha — pre-stable software.** CLI flags, config schema, on-disk
> layout and the Go module API can change without notice until 1.0. Always
> back up your `.kdbx` file before destructive operations. Use at your own risk.

> Vibe-coded by Claude Opus 3.7, DeepSeek V4 Pro, and GPT-4o.

## Quick start

```bash
kpass init                   # create a database + config
kpass insert work/email      # add your first entry
kpass ls                     # tree view
kpass get work/email         # show entry details
kpass copy work/email --otp  # copy OTP to clipboard
kpass search github          # fuzzy search
```

## Usage examples

### Working with the default database

```bash
kpass ls                           # tree view of the default DB
kpass ls --long                    # table with username, URL, OTP
kpass insert work/email -u alice   # create entry with username
kpass get work/email               # show all fields
kpass get work/email -f password   # show only the password
kpass copy work/email              # copy password to clipboard
kpass copy work/email -f otp       # copy OTP code
kpass edit work/email -u bob       # change username
kpass generate shopping/amazon     # generate and store a password
kpass move work/email personal/email  # move to another group
kpass remove work/email            # delete entry (asks confirmation)
kpass open work/email              # open URL in browser
kpass tags                         # list all tags with counts
```

### Multiple databases

```bash
# List all configured profiles
kpass db ls

# Switch the default
kpass db default personal

# Use a specific profile for one command
kpass ls @work
kpass get @personal shopping/amazon

# Add another database
kpass db add vault ~/keepass/vault.kdbx
```

### Search & filtering

```bash
kpass search github                # case-insensitive by name/path
kpass search --field username bob  # search inside fields
kpass search --tag work            # entries tagged "work"
kpass ls --tag work --tag critical # entries with BOTH tags
kpass ls --tag-any work personal   # entries with ANY of the tags
```

### Security

```bash
kpass audit                # check for weak/reused passwords
kpass audit @work          # audit a specific database
kpass history work/email   # view entry change history
kpass undo                 # list automated backups
kpass undo --restore-to ~/keepass/passwords.kdbx.bak.001  # restore
```

## Install

```bash
nix profile install github:irasikhin/kpass       # Nix
go install github.com/irasikhin/kpass/cmd/kpass@latest  # Go
```

Or download a [pre-built binary](https://github.com/irasikhin/kpass/releases).

Optional: install `fzf` for interactive picker, `xclip`/`wl-clipboard` for clipboard
(bundled automatically with the Nix package).

## Commands

| Command     | What it does                          |
|-------------|---------------------------------------|
| `init`      | Create a new database and config      |
| `ls`        | List entries as a tree                |
| `search`    | Find entries by name, path, or fields |
| `get`       | Show one entry or one field           |
| `copy`      | Copy a field to the clipboard         |
| `insert`    | Create a new entry                    |
| `edit`      | Edit an existing entry                |
| `generate`  | Generate and store a password         |
| `remove`    | Delete an entry                       |
| `move`      | Move or rename an entry               |
| `duplicate` | Duplicate an entry                    |
| `mkdir`     | Create a group path                   |
| `attach`    | Manage attachments                    |
| `pick`      | Pick interactively                    |
| `open`      | Open URL in browser                   |
| `tags`      | Tag management (list/add/remove)      |
| `combine`   | Merge two entries                     |
| `merge`     | Import from another database          |
| `export`    | Export to JSON/CSV                    |
| `import`    | Import from JSON/CSV                  |
| `audit`     | Check for security issues             |
| `history`   | View/restore entry versions           |
| `undo`      | Restore from backup                   |
| `stats`     | Database statistics                   |
| `clean`     | Remove empty groups                   |
| `doctor`    | Validate config and profiles          |
| `db`        | Manage database profiles              |

Run `kpass --help` for global flags, `kpass <cmd> --help` for details.

## Config

`~/.config/kpass/config.toml` (created by `kpass init`):

```toml
# ~/.config/kpass/config.toml
default_database = "work"

[databases.work]
database        = "~/keepass/work.kdbx"          # path to .kdbx file (required)
key_file        = "~/keepass/work.key"           # KeePass key file for extra DB protection
password_file   = "~/.secrets/work"              # read master password from a file
password_database = "bootstrap"                  # name of another profile that holds the password
password_entry  = "vaults/work"                  # path to the entry inside that other profile's DB
cache_ttl       = 600                            # remember master password for N seconds (0=forever, -1=off)
no_cache        = false                          # never remember the master password
backup_keep     = 10                             # keep at most N automatic backups (0=keep all)
backup_max_age  = 30                             # auto-delete backups older than N days (0=forever)

[databases.personal]
database = "~/keepass/personal.kdbx"
```

## Contributing & development

```bash
go test ./...              # tests
golangci-lint run ./...    # lint (config: .golangci.yml)
pre-commit install         # git hooks (format, lint, test)
```

See [CONTRIBUTING.md](./CONTRIBUTING.md) for conventions and release process.

## License

MIT — [LICENSE](./LICENSE).
