# kpass

[![CI](https://github.com/irasikhin/kpass/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/irasikhin/kpass/actions/workflows/ci.yml?query=branch%3Amain)
[![Coverage](https://img.shields.io/badge/coverage-88.2%25-brightgreen.svg)](#testing)
[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Another CLI for [KeePass](https://keepass.info/), built vibe-coding style:
human designs, AI drives.

> ⚠️ **Pre-1.0.** CLI flags, config schema, on-disk layout and the Go
> module API may change between minor versions until 1.0. Always back up
> your `.kdbx` file before destructive operations.

> Vibe-coded by Claude Opus, DeepSeek, and GPT.

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
kpass copy work/github -F otp    # copy OTP to clipboard
kpass search gh                  # fuzzy search by name, path, or field
```

Use `@profile` to address a non-default database: `kpass ls @personal`.

## Config

`~/.config/kpass/config.toml`:

```toml
default = "work"

[databases.work]
database = "~/keepass/work.kdbx"
```

`kpass init` writes this for you. The full schema — every per-profile
key, environment variable, and global flag that can override config,
plus worked examples (chained passwords, key files, backup retention) —
is in [docs/CONFIG.md](./docs/CONFIG.md). `kpass db add NAME PATH` /
`kpass db remove NAME` / `kpass db default NAME` edit profiles from the
command line.

## Docs

- `kpass --help` — full command list and global flags
- `kpass <cmd> --help` — per-command options and examples
- [docs/CONFIG.md](./docs/CONFIG.md) — full config reference (TOML keys, env vars, examples)
- [SECURITY.md](./SECURITY.md) — vulnerability reporting and secret-handling notes
- [CHANGELOG.md](./CHANGELOG.md) — what's in this release
- [CONTRIBUTING.md](./CONTRIBUTING.md) — development setup, conventions, releases

## Testing

```bash
go test ./...                                 # run all tests
go test ./... -cover                          # with line coverage per package
go test ./... -coverprofile=cov.out           # write a profile
go tool cover -func=cov.out | tail -1         # total coverage
go tool cover -html=cov.out -o cov.html       # browseable report
```

Current coverage: **88.2%** total. Per package:

| Package              | Coverage |
| -------------------- | -------- |
| `cmd/kpass`          | 100.0%   |
| `internal/cache`     | 100.0%   |
| `internal/clip`      | 100.0%   |
| `internal/color`     | 100.0%   |
| `internal/config`    | 100.0%   |
| `internal/db`        | 100.0%   |
| `internal/editor`    | 100.0%   |
| `internal/otp`       | 100.0%   |
| `internal/picker`    | 100.0%   |
| `internal/pwgen`     | 100.0%   |
| `internal/runtimex`  | 100.0%   |
| `internal/tree`      | 100.0%   |
| `internal/cli`       | 80.8%    |

## License

MIT — see [LICENSE](./LICENSE).
