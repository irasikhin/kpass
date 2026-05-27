# Changelog

All notable changes to this project are documented here. Format based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versioning follows
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

`scripts/release.sh` appends new sections automatically from
[Conventional Commits](https://www.conventionalcommits.org/) — please write
commit messages with that in mind. See [CONTRIBUTING.md](./CONTRIBUTING.md).

## [Unreleased]

## [0.1.0-alpha.2] — 2026-05-27

### Added

- enforce 0o600 on created database, backup, and export files (dce576d)
- accept legacy default_database key with conflict detection (83e53c9)
- print migration hints for removed commands (385176a)

### Fixed

- init handles configs with nil databases map or missing default (64f0774)

### Refactored

- remove dead code (removed.go legacy hints, __clear-clipboard, suggestCommand/levenshtein) (73b1559)
- YAGNI/DRY quick wins — remove dead code, use stdlib (2f21459)
- collapse OTP field dispatch into resolveFieldValue (4309fcc)
- drop dead entryGet/entryTitle, inline single-use bool wrapper (2471ecf)
- collapse twin filter/unique/sortedNames pairs into generics (3604469)
- drop local readPasswordFile, use config.ReadPasswordFile (354c657)
- centralise protected-field schema in entrySet (fdb65bc)
- collapse pwopts adapter; drop search --verbose/--flat aliases (4917362)
- split AuditCmd.Run into per-check helpers and a renderer (2b8cd69)
- split PickCmd.Run into action methods (2831dcb)
- split EditCmd.Run into editor / flag-based paths (cf2bb9b)
- split DoctorCmd.Run; collapse JSON/text duplication (6714e09)
- split RemoveCmd.Run into focused phase methods (6060abd)
- replace functional options with PickOpts struct (facefdc)
- split GenerateCmd.Run into target/apply/print/report phases (d9f8823)
- split StatsCmd.Run; share dbStats value across renderers (503f8d1)
- split HistoryCmd.Run into list/diff/restore phases (292f4cb)

### Docs

- drop "entry-first" tagline in favour of "another CLI for KeePass" (671070e)
- update GPT-4o to GPT 5.5 in vibe-coded credits (376576d)
- trim per DRY/KISS/YAGNI principles (996f4a1)
- add SECURITY policy and link from README; correct CHANGELOG clipboard wording (14add17)

### Build

- drop /vendor in favour of GOPROXY + Nix vendorHash (9668945)

### CI

- add Nix flake check job, contents:read perms, and dependabot (7e9be98)
- smoke-test linux binary on tag and document tag checklist (34876b5)

### Chores

- drop orphan scripts/install-hooks.sh (297755d)


## [0.1.0-alpha.1] — 2026-05-27

Initial public release on GitHub. Pre-stable software: CLI flags, config
schema, on-disk layout and the Go module API can change without notice
until 1.0.

### Features at launch

- Core CLI: `ls`, `search`, `get`, `copy`, `pick`, `insert`, `edit`,
  `remove`, `move`, `duplicate`, `mkdir`, `generate`.
- Multi-database profiles via `~/.config/kpass/config.toml`; chained
  password unlocking (one DB can hold another's master password).
- Per-database master-password cache with TTL.
- Clipboard and interactive picker integration via common platform tools
  (`xclip`/`xsel`, `wl-clipboard`, macOS clipboard support, `fzf`).
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
[0.1.0-alpha.2]: https://github.com/irasikhin/kpass/compare/v0.1.0-alpha.1...v0.1.0-alpha.2
