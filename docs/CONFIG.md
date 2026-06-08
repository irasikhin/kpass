# Configuration reference

kpass reads `~/.config/kpass/config.toml` (override with `--config PATH` or
`KPASS_CONFIG=…`). Everything in the file is optional except the top-level
`default` and at least one `[databases.<name>]` block.

A minimal config:

```toml
default = "work"

[databases.work]
database = "~/keepass/work.kdbx"
```

`kpass init` writes a config of this shape on first run.

---

## Top-level keys

| Key                 | Type    | Required | Notes                                                                 |
|---------------------|---------|----------|-----------------------------------------------------------------------|
| `default`           | string  | yes      | Name of the profile to use when `@profile` is not given.              |
| `databases.<name>`  | table   | yes      | One block per database; at least one block is required.               |

Unknown top-level keys are rejected at load time.

---

## Per-profile keys (`[databases.<name>]`)

### Required

| Key        | Type   | Notes                                       |
|------------|--------|---------------------------------------------|
| `database` | string | Path to the `.kdbx` file. `~` is expanded.  |

### Master-password sources (pick at most one)

| Key                 | Type   | Notes                                                                                       |
|---------------------|--------|---------------------------------------------------------------------------------------------|
| `password_file`     | string | Read the master password from this file (first line, trailing newline stripped). `~` expanded. |
| `password_database` | string | Resolve the master password by reading an entry from another profile.                       |
| `password_entry`    | string | Path inside `password_database` where the password lives. Must be set together with `password_database`. |

Combining `password_file` with the `password_database`/`password_entry` pair
is rejected. If neither is set, kpass prompts for the master password
interactively (cached per [session](#session-cache)).

### Key file

| Key        | Type   | Notes                                                                 |
|------------|--------|-----------------------------------------------------------------------|
| `key_file` | string | Composite key file path. `~` expanded. Combines with the master password (whichever source above). |

### Session cache

The decrypted master password is cached per profile for `session_ttl`
seconds so subsequent commands don't re-prompt.

| Key            | Type | Default                                                | Notes                                              |
|----------------|------|--------------------------------------------------------|----------------------------------------------------|
| `session_ttl`  | int  | 300 (`config.DefaultCacheTTL`)                         | Seconds to cache the password. Cache lives in `$XDG_RUNTIME_DIR/kpass/`. |
| `cache_ttl`    | int  | —                                                      | Legacy alias for `session_ttl`.                    |
| `no_session`   | bool | `false`                                                | Disable the cache for this profile.                |
| `no_cache`     | bool | —                                                      | Legacy alias for `no_session`.                     |

> The legacy top-level `default_database` key was removed in `0.3.0`.
> Rename it to `default`.

### System keyring

Instead of the plaintext session cache, the master password can be persisted
in the operating system's secret store — gnome-keyring / any Secret Service
backend (KWallet, KeePassXC) on Linux, the Keychain on macOS, the Credential
Manager on Windows.

| Key           | Type | Default | Notes                                                                 |
|---------------|------|---------|-----------------------------------------------------------------------|
| `use_keyring` | bool | `false` | Read the master password from the OS keyring before prompting, and store it there after a successful unlock. |

When `use_keyring` is on, the plaintext `$XDG_RUNTIME_DIR/kpass/*.json`
[session cache](#session-cache) is **not written** for that profile — the
keyring is the only at-rest copy and it is OS-encrypted. `use_keyring` cannot
be combined with `password_file` (rejected at load time); a
`password_database` chain takes precedence and bypasses the keyring.

Manage stored secrets with the `keyring` subcommands (these also flip the
config key for you):

```bash
kpass keyring set [@profile]     # prompt, verify, store, enable use_keyring
kpass keyring rm  [@profile]     # delete the secret, disable use_keyring
kpass keyring status [@profile]  # backend availability + whether stored
```

A Secret Service provider must be running for storage to succeed; if the
backend is unavailable kpass falls back to prompting (nothing is cached).
`kpass doctor` flags a profile whose `use_keyring` is set when no backend is
reachable.

### Backups

Every `kpass` mutation writes a timestamped `*.bak` next to the database
before saving. These keys cap how many backups stay on disk.

| Key                   | Type | Default      | Notes                                                          |
|-----------------------|------|--------------|----------------------------------------------------------------|
| `backup_keep`         | int  | `0` = keep all | Maximum number of backups to retain.                          |
| `backup_max_age_days` | int  | `0` = forever  | Delete backups older than N days.                             |

Both must be non-negative integers; combine freely (whichever criterion
triggers first removes a backup).

---

## Environment variables

| Variable             | Effect                                                                          |
|----------------------|---------------------------------------------------------------------------------|
| `KPASS_CONFIG`       | Path to the config file. Equivalent to `--config`.                              |
| `KPASS_SESSION_TTL`  | Override `session_ttl` (seconds) for the chosen profile.                        |
| `KPASS_CACHE_TTL`    | Legacy alias for `KPASS_SESSION_TTL`.                                           |
| `KPASS_PASSWORD_FILE`| Fallback `password_file` when no config / no profile match.                     |
| `KPASS_KEY_FILE`     | Fallback `key_file` when no config / no profile match.                          |
| `KPASS_USE_KEYRING`  | Override `use_keyring` (`1`/`true`/`yes`/`on` enable; anything else disables).   |
| `KEEPASS_DB_PATH`    | Fallback `database` path when no profile is selected.                           |
| `XDG_RUNTIME_DIR`    | Where the session-cache file lives. If unset, the cache is disabled.            |
| `XDG_CACHE_HOME`     | Where shell-completion entry caches live (`$XDG_CACHE_HOME/kpass/`).            |
| `NO_COLOR`           | Disable ANSI colors. Equivalent to `--no-color`.                                |
| `VISUAL` / `EDITOR`  | Editor used by `kpass edit`. `VISUAL` wins if both are set.                     |
| `WAYLAND_DISPLAY`    | If set, kpass picks the Wayland clipboard backend (`wl-copy`/`wl-paste`).       |
| `PASSWORD_STORE_DIR` | Default source directory for `kpass import-pass`.                               |

---

## Global CLI flags that override config

These globals apply to every command (`kpass --help` lists them). When set,
they shadow the per-profile config value for the current invocation.

| Flag                       | Profile field    |
|----------------------------|------------------|
| `-c, --config PATH`        | (selects file)   |
| `-d, --database PATH`      | `database`       |
| `-p, --password-file PATH` | `password_file`  |
| `-k, --key-file PATH`      | `key_file`       |
| `--session-ttl N` (alias: `--cache-ttl`) | `session_ttl` |
| `--no-session` (alias: `--no-cache`)     | `no_session`  |
| `--use-keyring` / `--no-keyring`         | `use_keyring` |
| `-C, --no-color`           | (cosmetic)       |
| `-y, --yes`                | (cosmetic)       |

---

## Examples

### 1. Multiple databases, one default

```toml
default = "personal"

[databases.personal]
database = "~/keepass/personal.kdbx"

[databases.work]
database = "~/keepass/work.kdbx"
```

Run `kpass ls` → reads `personal`. Run `kpass ls @work` → reads `work`.

### 2. Chained password (work password stored inside personal)

```toml
default = "personal"

[databases.personal]
database = "~/keepass/personal.kdbx"

[databases.work]
database          = "~/keepass/work.kdbx"
password_database = "personal"
password_entry    = "db-passwords/work"
```

Opening `work` unlocks `personal` first, reads the password from
`db-passwords/work`, then unlocks `work`. The chain is detected for cycles
and rejected with an error.

### 3. Read master password from a file

```toml
default = "main"

[databases.main]
database      = "~/keepass/main.kdbx"
password_file = "~/.config/kpass/main.password"
key_file      = "~/.config/kpass/main.key"
```

The password file is read whole; trailing newlines are stripped. Treat it
like an SSH private key (`chmod 600`, don't commit).

### 4. Tight backup retention

```toml
default = "main"

[databases.main]
database            = "~/keepass/main.kdbx"
backup_keep         = 20
backup_max_age_days = 30
```

Keeps at most 20 backups and never anything older than 30 days. With both
set, the union policy applies — a backup is removed when either limit is
exceeded.

### 5. Disable session cache (paranoid mode)

```toml
default = "vault"

[databases.vault]
database   = "~/keepass/vault.kdbx"
no_session = true
```

Every command re-prompts for the master password. Equivalent to running
each command with `--no-session`.

### 6. Per-profile session TTL

```toml
default = "personal"

[databases.personal]
database    = "~/keepass/personal.kdbx"
session_ttl = 1800        # 30 minutes

[databases.work]
database    = "~/keepass/work.kdbx"
session_ttl = 300         # 5 minutes
no_session  = false
```

### 7. Store the master password in the OS keyring

```toml
default = "personal"

[databases.personal]
database    = "~/keepass/personal.kdbx"
use_keyring = true
```

```bash
kpass keyring set        # prompt once; the password is verified then stored
kpass get personal/x     # no prompt — read from the keyring, no plaintext cache
```

The first unlock prompts and stores; later commands read from the keyring
until you run `kpass keyring rm`. Easiest to enable via `kpass keyring set`,
which writes `use_keyring = true` for you after verifying the password.

---

## Resolution order

For each setting kpass takes the **first** value found, in this order:

1. CLI flag (e.g. `--database`, `--session-ttl`).
2. Environment variable where one exists (`KPASS_*`).
3. Per-profile value in `config.toml`.
4. Built-in default (`session_ttl = 300`, `backup_keep = 0`, etc.).

The selected profile is whatever follows `@` in the first positional arg
(`kpass ls @work`), or the top-level `default` otherwise.

---

## Related

- `kpass init` — writes a starter config + creates the first database.
- `kpass doctor` — validates the current config and reports unreachable
  files, broken password chains, and unknown keys.
- `kpass db add NAME PATH` / `kpass db remove NAME` / `kpass db default NAME` —
  edit profiles from the command line.
- `kpass keyring set` / `kpass keyring rm` / `kpass keyring status` — manage the
  master password in the OS keyring (`use_keyring`).
- [`internal/config/load.go`](../internal/config/load.go) is the authoritative
  parser; if this doc disagrees with that file, the file is right.
