// Package cli implements the kpass command-line interface. Argument parsing
// and help rendering are delegated to alecthomas/kong; this file orchestrates:
//
//  1. Loading the TOML file config so `--config` is honored before commands run.
//  2. Kong parse + dispatch, with `@profile` selectors captured via Passthrough
//     and errors reshaped into our UserError stderr format.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/alecthomas/kong"

	"github.com/irasikhin/kpass/internal/color"
	"github.com/irasikhin/kpass/internal/config"
	"github.com/irasikhin/kpass/internal/db"
)

// errHelpRequested is a sentinel returned by command Run methods to signal
// that the command's full --help should be displayed (e.g. when a command
// with optional positional arguments is invoked without any actionable
// parameters).
var errHelpRequested = errors.New("help requested")

// ctx is the runtime context bound into every command's Run method. Holds
// stdio, parsed config, and a lazily-opened *db.DB.
type ctx struct {
	in   io.Reader
	out  io.Writer
	errw io.Writer

	fileConfig config.FileConfig
	configPath string

	// gf carries the parsed global flags that influence database opening.
	gf globalFlags
	// selector is the value of @<profile> stripped from argv pre-parse.
	selector string

	cfg *config.Config // set after openDatabase
	db  *db.DB         // set after openDatabase
}

// globalFlags mirrors the values surfaced by the kong root struct that affect
// database opening — passed through to config.ResolveRuntime.
type globalFlags struct {
	database     string
	passwordFile string
	keyFile      string
	cacheTTL     *int
	noCache      *bool
	useKeyring   *bool
	yes          bool
}

func (g globalFlags) toRuntimeFlags() config.RuntimeFlags {
	return config.RuntimeFlags{
		Database:     g.database,
		PasswordFile: g.passwordFile,
		KeyFile:      g.keyFile,
		CacheTTL:     g.cacheTTL,
		NoCache:      g.noCache,
		UseKeyring:   g.useKeyring,
	}
}

// Run is the entry point invoked by cmd/kpass/main.go.
func Run(argv []string, in io.Reader, out, errw io.Writer) int {
	// Detect terminal and NO_COLOR before anything prints.
	color.Init()

	// Hidden child-process branch for clipboard auto-clear. Never goes
	// through kong because we don't want it to surface in --help.
	if len(argv) > 0 && argv[0] == "__clear-clipboard" {
		return runClearClipboard(argv)
	}

	c := &ctx{in: in, out: out, errw: errw}
	if err := runOnce(c, argv); err != nil {
		if errors.Is(err, context.Canceled) {
			fmt.Fprintln(errw, color.Yellow("Interrupted."))
			return 130
		}
		var ue *UserError
		var me *db.MatchError
		switch {
		case errors.As(err, &ue):
			if ue.Msg != "" {
				fmt.Fprintln(errw, color.Red(ue.Msg))
			}
			return 1
		case errors.As(err, &me):
			fmt.Fprintln(errw, color.Red(me.Msg))
			return 1
		default:
			fmt.Fprintln(errw, color.Red(err.Error()))
			return 1
		}
	}
	return 0
}

func runOnce(c *ctx, argv []string) error {
	// 1. Pre-scan for --config so file config loads before kong runs.
	explicitConfig := preScanConfig(argv)
	fc, configPath, err := config.Load(explicitConfig)
	if err != nil {
		return &UserError{Msg: err.Error()}
	}
	c.fileConfig = fc
	c.configPath = configPath

	// 2. Extract @profile selector (any argv token starting with @).
	rest, selector, err := extractSelector(argv)
	if err != nil {
		return err
	}
	c.selector = selector

	// 3. Build the kong application.
	cli := &kpassCLI{}
	app, err := newKongApp(cli, c)
	if err != nil {
		return &UserError{Msg: err.Error()}
	}

	// 3a. When called with no arguments at all, print help.
	if len(rest) == 0 {
		rest = []string{"--help"}
	}

	// Recover from kong's exits (help, version, error printing).
	var kctx *kong.Context
	var exit *kongExit
	func() {
		defer func() {
			if r := recover(); r != nil {
				if ke, ok := r.(kongExit); ok {
					exit = &ke
					return
				}
				panic(r)
			}
		}()
		kctx, err = app.Parse(rest)
	}()
	if exit != nil {
		if exit.code != 0 {
			return &UserError{Msg: ""}
		}
		return nil
	}
	if err != nil {
		if isUsageError(err) {
			restHelp := append([]string{}, rest...)
			restHelp = append(restHelp, "--help")
			cli2 := &kpassCLI{}
			app2, err2 := newKongApp(cli2, c)
			if err2 == nil {
				func() {
					defer func() {
						if r := recover(); r != nil {
							if _, ok := r.(kongExit); ok {
								return
							}
							panic(r)
						}
					}()
					_, _ = app2.Parse(restHelp)
				}()
				return nil
			}
		}
		return &UserError{Msg: reshapeKongError(err)}
	}

	// 4. Pull global flag values into ctx.
	c.gf = cli.globalFlags()
	if cli.NoColor {
		color.Disable()
	}

	// 5. @profile selector compatibility check.
	if c.selector != "" {
		top := topCommand(kctx.Command())
		if !commandsWithSelector[top] {
			return &UserError{Msg: fmt.Sprintf("Command '%s' does not accept @db.", top)}
		}
	}

	// 6. Run the selected command.
	if err := kctx.Run(c); err != nil {
		if errors.Is(err, errHelpRequested) {
			restHelp := append([]string{}, rest...)
			restHelp = append(restHelp, "--help")
			cli2 := &kpassCLI{}
			app2, err2 := newKongApp(cli2, c)
			if err2 == nil {
				func() {
					defer func() {
						if r := recover(); r != nil {
							if _, ok := r.(kongExit); ok {
								return
							}
							panic(r)
						}
					}()
					_, _ = app2.Parse(restHelp)
				}()
				return nil
			}
		}
		return err
	}
	return nil
}

// kongExit is the panic value used to intercept kong.Exit invocations so that
// help / version output can short-circuit cleanly without os.Exit.
type kongExit struct{ code int }

// newKongApp constructs a kong parser with the standard options shared across
// the initial parse and any retry-for-help parse.
func newKongApp(cli *kpassCLI, c *ctx) (*kong.Kong, error) {
	return kong.New(
		cli,
		kong.Name("kpass"),
		kong.Description("Another CLI for KeePass."),
		kong.UsageOnError(),
		kong.Writers(c.out, c.errw),
		kong.Bind(c),
		kong.Vars{"version": versionLine()},
		kong.ConfigureHelp(kong.HelpOptions{
			Compact:             true,
			Summary:             false,
			FlagsLast:           true,
			NoExpandSubcommands: true,
		}),
		kong.Exit(func(code int) {
			panic(kongExit{code})
		}),
	)
}

// isUsageError reports whether err is a kong ParseError whose underlying
// message starts with "expected " (missing positional arg or missing
// subcommand). Such errors should trigger the command's --help instead.
func isUsageError(err error) bool {
	var pe *kong.ParseError
	if !errors.As(err, &pe) {
		return false
	}
	return strings.HasPrefix(pe.Error(), "expected ")
}

// preScanConfig finds --config (or --config=foo) anywhere in argv. Returns ""
// if not present. Mirrors what kong will later parse, but lets us load the file
// before any command runs.
func preScanConfig(argv []string) string {
	for i := 0; i < len(argv); i++ {
		t := argv[i]
		if t == "--config" && i+1 < len(argv) {
			return argv[i+1]
		}
		if strings.HasPrefix(t, "--config=") {
			return t[len("--config="):]
		}
	}
	return ""
}

// reshapeKongError converts kong's default error message to something closer
// to our historical format ("kpass <cmd>: <thing>") and adds the help footer.
// Kong's errors are typically prefixed with "kpass: ", which already matches.
func reshapeKongError(err error) string {
	msg := err.Error()
	msg = strings.TrimPrefix(msg, "kpass: error: ")
	msg = strings.TrimPrefix(msg, "kpass: ")

	// Common kong phrasings → canonical form.
	switch {
	case strings.HasPrefix(msg, "expected one of "):
		// fall through; the message itself is descriptive enough
	case strings.HasPrefix(msg, "unexpected argument "):
		rest := strings.TrimPrefix(msg, "unexpected argument ")
		arg := rest
		if i := strings.Index(rest, ", did you mean "); i >= 0 {
			arg = rest[:i]
		}
		arg = strings.Trim(arg, "\"' ")
		if hint := removedCommandHint(arg); hint != "" {
			return hint + "\nUse 'kpass --help' for usage."
		}
		return fmt.Sprintf("kpass: argument command: invalid choice: '%s'\nUse 'kpass --help' for usage.", arg)
	}
	return "kpass: " + msg + "\nUse 'kpass --help' for usage."
}

func removedCommandHint(command string) string {
	hints := map[string]string{
		"show":  "show was removed; use: kpass get [@db] <entry> [--field ...]",
		"pass":  "pass was removed; use: kpass get [@db] <entry> --field password",
		"clip":  "clip was removed; use: kpass copy [@db] <entry> [--field ...]",
		"otp":   "otp was removed; use: kpass get [@db] <entry> --field otp or kpass copy [@db] <entry> --field otp",
		"grep":  "grep was removed; use: kpass search [@db] <term> [--field ...]",
		"close": "close was removed; session handling is automatic.",
		"cp":    "cp was removed; use: kpass duplicate [@db] <source> <destination> or kpass copy [@db] <entry>",
		"clone": "clone was removed; use: kpass duplicate [@db] <source> <destination>",
	}
	return hints[command]
}

// topCommand collapses a kong command path like "attach ls" into the top-level
// "attach" so the selector-not-supported error matches user expectations.
func topCommand(path string) string {
	if i := strings.IndexByte(path, ' '); i >= 0 {
		return path[:i]
	}
	return path
}

// resolveRuntime merges flags, env, and the selected profile into the final
// runtime Config without opening the database. Used by openDatabase and by
// the keyring subcommands.
func (c *ctx) resolveRuntime() (config.Config, error) {
	cfg, err := config.ResolveRuntime(c.fileConfig, c.selector, c.gf.toRuntimeFlags(), passwordFetcher, c.errw)
	if err != nil {
		return config.Config{}, &UserError{Msg: err.Error()}
	}
	return cfg, nil
}

// openDatabase resolves the runtime config and opens the DB. Called from
// command Run methods that need DB access.
func (c *ctx) openDatabase() error {
	cfg, err := c.resolveRuntime()
	if err != nil {
		return err
	}
	c.cfg = &cfg
	opened, err := db.Open(cfg)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return err
		}
		var ue *UserError
		if errors.As(err, &ue) {
			return ue
		}
		return &UserError{Msg: err.Error()}
	}
	c.db = opened

	// Refresh completion cache so the shell completion functions can
	// offer entry paths without re-opening the database.
	refreshCompletionCache(c.selector, opened)

	return nil
}

// refreshCompletionCache writes entry paths to the completion cache file so
// that shell tab-completion can offer paths without opening the database.
func refreshCompletionCache(profile string, opened *db.DB) {
	if profile == "" {
		profile = "default"
	}
	paths := make([]string, 0)
	for _, e := range opened.SortedEntries() {
		paths = append(paths, e.DisplayPath())
	}
	_ = WriteEntryCache(profile, paths)
}

// passwordFetcher opens a source DB and returns the password stored in the
// named entry. Wires config's recursive resolver into db.Open.
func passwordFetcher(src config.Config, entryPath string) (string, error) {
	d, err := db.Open(src)
	if err != nil {
		return "", err
	}
	entry, err := d.ResolveEntry(entryPath)
	if err != nil {
		return "", err
	}
	return entry.Raw().GetPassword(), nil
}
