# Changelog

All notable changes to this project are documented here. Format based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versioning follows
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

`scripts/release.sh` appends new sections automatically from
[Conventional Commits](https://www.conventionalcommits.org/) — please write
commit messages with that in mind. See [CONTRIBUTING.md](./CONTRIBUTING.md).

## [Unreleased]

## [0.1.0-alpha.1] — 2026-05-27

Initial public release on GitHub. Pre-stable software: CLI flags, config
schema, on-disk layout and the Go module API can change without notice
until 1.0.

### Features at launch

- Entry-first CLI: `ls`, `search`, `get`, `copy`, `pick`, `insert`, `edit`,
  `remove`, `move`, `duplicate`, `mkdir`, `generate`.
- Multi-database profiles via `~/.config/kpass/config.toml`; chained
  password unlocking (one DB can hold another's master password).
- Per-database master-password cache with TTL.
- Pure-Go clipboard (X11 via dlopen, Wayland via `wl-clipboard` fallback)
  and interactive picker — no `fzf`/`xclip` runtime dependency.
- Attachments (`attach`), OTP (RFC 6238) export/copy, custom fields, tags.
- Cross-database tooling: `merge`, `import`/`export` (JSON/CSV/pass(1)),
  `combine` (merge two entries within one DB).
- Safety: automatic timestamped backups, `undo` with `--restore-to`,
  `history` per entry, `audit` (weak/reused passwords, missing fields),
  `doctor` (config + profile validation).
- Output: tree/flat/JSON formats, color via ANSI with `NO_COLOR` support,
  shell completions (bash, zsh, fish).
- Non-interactive mode via global `--yes/-y` and `--json` on mutating
  commands.

[Unreleased]: https://github.com/irasikhin/kpass/compare/v0.1.0-alpha.1...HEAD
[0.1.0-alpha.1]: https://github.com/irasikhin/kpass/releases/tag/v0.1.0-alpha.1
