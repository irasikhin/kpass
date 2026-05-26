# kpass

[![CI](https://github.com/irasikhin/kpass/actions/workflows/ci.yml/badge.svg)](https://github.com/irasikhin/kpass/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Another CLI for [KeePass](https://keepass.info/), built vibe-coding style:
human designs, AI drives.

> ⚠️ **Alpha — pre-stable software.** CLI flags, config schema, on-disk
> layout and the Go module API can change without notice until 1.0. Always
> back up your `.kdbx` file before destructive operations.

> Vibe-coded by Claude Opus 3.7, DeepSeek V4 Pro, and GPT 5.5.

## Install

```bash
nix profile install github:irasikhin/kpass             # Nix (bundles fzf, xclip, wl-clipboard)
go install github.com/irasikhin/kpass/cmd/kpass@latest # Go
```

Or grab a [pre-built binary](https://github.com/irasikhin/kpass/releases).
Outside Nix, install `fzf` (picker) and `xclip`/`wl-clipboard` (clipboard)
yourself if you want those features.

## Quick start

```bash
kpass init                       # one-time: create a database + config
kpass insert work/github         # add an entry
kpass ls                         # tree view
kpass get work/github            # show fields
kpass copy work/github -f otp    # copy OTP to clipboard
kpass search gh                  # fuzzy search by name, path, or field
```

Use `@profile` to address a non-default database: `kpass ls @personal`.

## Config

`~/.config/kpass/config.toml`:

```toml
default_database = "work"

[databases.work]
database = "~/keepass/work.kdbx"
```

`kpass init` writes this for you. Every field beyond `database` is optional
— key files, password sources, caching, backup retention. See
`kpass db add --help` for the full schema.

## Docs

- `kpass --help` — full command list and global flags
- `kpass <cmd> --help` — per-command options and examples
- [CHANGELOG.md](./CHANGELOG.md) — what's in this release
- [CONTRIBUTING.md](./CONTRIBUTING.md) — development setup, conventions, releases

## License

MIT — see [LICENSE](./LICENSE).
