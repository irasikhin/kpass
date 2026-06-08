# Changelog

All notable changes to this project are documented here. Format based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versioning follows
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

`scripts/release.sh` appends new sections automatically from
[Conventional Commits](https://www.conventionalcommits.org/) — please write
commit messages with that in mind. See [CONTRIBUTING.md](./CONTRIBUTING.md).

## [0.4.0] — 2026-06-08

### Added

- store master password in the OS keyring (1a0ebe0)

### Docs

- refresh coverage badge and table to 88.2% (5297f7d)
- pin CI badge to main branch to bust camo cache (c9d85d5)

### Chores

- fix lint and gofmt failures (06c31b5)


## [0.3.2] — 2026-05-27

### Docs

- drop model version numbers from vibe-coded line (d074e03)

### Tests

- cover Init detect() via test seam (aed51a1)
- cover ExpandPath no-HOME and default stdinFd (977e5f8)
- cover Suffix indicator, min-width defaults, and truncate (afa653b)
- drop dead Charset branches, cover rand errors and labels (21890cb)
- cover SHA512, invalid period, wall-time fallback, Parse propagation (d4a751f)
- seam tempfile FS calls, drop Close-error branch (e254f2c)
- cover FS error branches via filepathAbs/marshal/remove seams (2c41d2e)
- seam Backend(), cover default exec lambdas and Process.Release (9e5a8dc)
- cover Stat permission error, ReadFile error, MkdirAll error (ae1ce12)
- introduce FS/library seams, drive package to 100% (8666fcb)
- cover default runCmdFn lambda body (8aa44c3)
- add OpenStartHook seam and ~30 command tests (e5dca9e)


## [0.3.1] — 2026-05-27

### Tests

- cover Load/Store/Clear/Path edge cases (bf758bf)
- cover write.go and resolution edge cases (bf2707d)
- cover PromptSecret and stdin reader (6f983f5)
- cover Edit and EditorArgv (f0e96ae)
- add seams and cover all backends (4f464af)
- add seams and cover Pick fully (60519a1)
- build seed helper and cover all 11 files (09c009b)
- cover main via exit seam (9eec7c2)
- cover ~30 untested commands (b8aae51)

### CI

- surface coverage on every CI run, add badge (014121f)


## [0.3.0] — 2026-05-27

### Breaking

- The legacy top-level `default_database` config key is removed; use
  `default` instead. Configs that still use the old key now fail at
  load with an "unsupported KPass config key" error. (8f8eba3)

### Added

- `scripts/release.sh` now applies 0.x semver semantics: breaking
  changes (`type!:`, `BREAKING CHANGE:`) bump minor instead of major
  while the project stays in 0.x. (cd40a1e)

### Refactored

- drop legacy default_database top-level key (8f8eba3)

### Docs

- drop [Unreleased] section from CHANGELOG (a2084c9)
- add docs/CONFIG.md - full config reference and worked examples (53862dc)


## [0.2.1] — 2026-05-27

### Fixed

- restore mid-sentence CSV/OTP acronyms in error strings (784e739)
- init --force overwrites existing database (c13f377)
- drop open --field otp; harden Windows opener (077c3f0)
- address six "important" findings from the v0.2.0 audit (a15bea6)

### Refactored

- %w error wraps, drop dead Render/ClearAll, warn on backup fail (788b9c6)

### Docs

- drop "alpha" language and rewrite v0.2.0 changelog section (132e424)
- README copy flag is -F (not -f), and release.sh header is honest (76e746b)

### Chores

- ignore /.claude/ tooling state (72c0fa7)


## [0.2.0] — 2026-05-27

First stable release. Promotes the `0.1.0-alpha` series; users coming
from `v0.1.0-alpha.1` get every change recorded in the alpha sections
below plus the items listed here.

### Added

- Auto-detect bump from conventional commits in `scripts/release.sh`
  (defaults to `auto`; `BREAKING CHANGE:`/`type!:` → major, `feat:` →
  minor, otherwise patch). Refuses to bump from a prerelease tag —
  pass an explicit `X.Y.Z` for transitions out of an alpha/beta/rc.
  (a6e38a5)

### Fixed

- Release script's `BREAKING CHANGE:` detection is now anchored to the
  start of a line, so prose mentions in commit bodies no longer
  escalate the bump. (745e650)


## [0.1.0-alpha.3] — 2026-05-27

### Fixed

- mark intentional return-nil-on-error cases for nilerr (49a05d9)
- gate release smoke test by GOARCH=amd64 (60de487)

### Refactored

- drop dead trailing newlines, switch-with-one-case, and inefficient builders (659c448)
- drop unused printVersion and test helpers (5cc111a)
- drop unused parameters and results flagged by unparam (fadf157)

### Build

- bump golangci/golangci-lint-action from 6 to 9 (2a8ea7f)
- bump actions/setup-go from 5 to 6 (331b57b)
- bump actions/upload-artifact from 4 to 7 (4888d12)
- bump actions/download-artifact from 4 to 8 (8fac367)
- bump softprops/action-gh-release from 2 to 3 (635e0c0)

### CI

- migrate golangci-lint config and CI to v2.12.2 (cb23ed4)

### Style

- lowercase error strings and drop trailing punctuation (4f78453)


## [0.1.0-alpha.2] — 2026-05-27 (tag only, no binary release)

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

[0.2.0]: https://github.com/irasikhin/kpass/compare/v0.1.0-alpha.3...v0.2.0
[0.1.0-alpha.3]: https://github.com/irasikhin/kpass/compare/v0.1.0-alpha.1...v0.1.0-alpha.3
[0.1.0-alpha.2]: https://github.com/irasikhin/kpass/compare/v0.1.0-alpha.1...v0.1.0-alpha.2
[0.1.0-alpha.1]: https://github.com/irasikhin/kpass/releases/tag/v0.1.0-alpha.1
[0.2.1]: https://github.com/irasikhin/kpass/compare/v0.2.0...v0.2.1
[0.3.0]: https://github.com/irasikhin/kpass/compare/v0.2.1...v0.3.0
[0.3.1]: https://github.com/irasikhin/kpass/compare/v0.3.0...v0.3.1
[0.3.2]: https://github.com/irasikhin/kpass/compare/v0.3.1...v0.3.2
[0.4.0]: https://github.com/irasikhin/kpass/compare/v0.3.2...v0.4.0
