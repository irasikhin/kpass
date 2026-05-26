package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/irasikhin/kpass/internal/color"
	"github.com/irasikhin/kpass/internal/db"
	"github.com/irasikhin/kpass/internal/runtimex"
)

// MergeCmd imports entries from another KDBX file into the current database.
type MergeCmd struct {
	Source             string `arg:"" help:"Source database file to merge from."`
	SourcePasswordFile string `help:"Password file for the source database." placeholder:"PATH"`
	SourceKeyFile      string `help:"Key file for the source database." placeholder:"PATH"`
	OnConflict         string `default:"rename" enum:"error,skip,overwrite,rename" help:"On path conflict: error, skip, overwrite, or rename."`
	RenameSuffix       string `help:"Suffix appended to renamed entries (default: \" (imported)\")."`
	JSON               bool   `help:"Output as JSON."`
}

// Help returns extended help for merge.
func (MergeCmd) Help() string {
	return `Import every entry from another KeePass database (KDBX) into the
current database.

Entries are matched by their full display path. When a path already
exists, --on-conflict controls what happens:

      error     – abort with an error
      skip      – silently skip the conflicting entry
      overwrite – replace the existing entry with the source data
      rename    – add a suffix to create a unique path (default)

The source database is opened independently with --source-password-file
and --source-key-file.`
}

func (cmd *MergeCmd) Run(c *ctx) error {
	if err := c.openDatabase(); err != nil {
		return err
	}

	targetAbs, _ := filepath.Abs(runtimex.ExpandPath(c.cfg.Database))
	sourceAbs, _ := filepath.Abs(runtimex.ExpandPath(cmd.Source))
	if targetAbs == sourceAbs {
		return &UserError{Msg: "Source and target database must be different."}
	}

	src, err := db.OpenSimple(cmd.Source, cmd.SourcePasswordFile, cmd.SourceKeyFile, "")
	if err != nil {
		return &UserError{Msg: err.Error()}
	}

	stats, err := c.db.Merge(src, db.MergeOpts{
		OnConflict:   db.ConflictStrategy(cmd.OnConflict),
		RenameSuffix: cmd.RenameSuffix,
	})
	if err != nil {
		return &UserError{Msg: err.Error()}
	}
	if err := c.db.Save(); err != nil {
		return &UserError{Msg: err.Error()}
	}

	if cmd.JSON {
		data, _ := json.Marshal(map[string]any{
			"status":      "ok",
			"imported":    stats.Imported,
			"overwritten": stats.Overwritten,
			"skipped":     stats.Skipped,
			"renamed":     stats.Renamed,
		})
		fmt.Fprintln(c.out, string(data))
		return nil
	}

	fmt.Fprintf(c.out, "%s %d\n", color.Cyan("Imported:"), stats.Imported)
	fmt.Fprintf(c.out, "%s %d\n", color.Yellow("Overwritten:"), stats.Overwritten)
	fmt.Fprintf(c.out, "%s %d\n", color.Faint("Skipped:"), stats.Skipped)
	fmt.Fprintf(c.out, "%s %d\n", color.Faint("Renamed:"), stats.Renamed)
	return nil
}
