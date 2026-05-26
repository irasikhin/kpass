package cli

import (
	"fmt"
	"os"

	"github.com/irasikhin/kpass/internal/color"
	"github.com/irasikhin/kpass/internal/db"
)

// UndoCmd restores the database from a backup, or lists/prunes backups.
type UndoCmd struct {
	List      bool   `short:"l" help:"List available backups."`
	Force     bool   `short:"f" help:"Skip confirmation prompts."`
	Index     int    `default:"-1" help:"Restore a specific backup by index (0 = most recent)."`
	Prune     int    `default:"-1" help:"Delete old backups. 0 = all, N = keep last N."`
	RestoreTo string `name:"restore-to" help:"Restore backup to a different path instead of overwriting the current database." placeholder:"PATH"`
}

// Help returns extended help for undo.
func (UndoCmd) Help() string {
	return `Restore the database from a timestamped backup, or manage backups.

kpass automatically creates a .bak snapshot before every destructive
save (remove, edit, generate, etc.).

  --list           Show all available backups with timestamps.
  --index N        Restore a specific backup (0 = most recent, default).
  --restore-to P   Restore to a different path instead of overwriting.
  --prune N        Delete old backups: 0 = all, N = keep last N.

Restoring overwrites the current database. The current database is
NOT backed up before the restore — make a manual copy first if needed.
Use --restore-to to write the backup to a separate file.`
}

func (cmd *UndoCmd) Run(c *ctx) error {
	if err := c.openDatabase(); err != nil {
		return err
	}

	backups, err := c.db.ListBackups()
	if err != nil {
		return &UserError{Msg: err.Error()}
	}

	// --- prune ---
	if cmd.Prune >= 0 {
		return runPrune(c, backups, cmd.Prune, cmd.Force)
	}

	// --- list ---
	if cmd.List {
		if len(backups) == 0 {
			fmt.Fprintln(c.out, color.Faint("No backups found."))
			return nil
		}
		fmt.Fprintf(c.out, "%s\n", color.Bold(fmt.Sprintf("Backups for %s:", c.cfg.Database)))
		for i, b := range backups {
			marker := " "
			if i == 0 {
				marker = color.Green("*")
			}
			info, _ := os.Stat(b)
			ts := ""
			sz := ""
			if info != nil {
				ts = info.ModTime().Format("2006-01-02 15:04")
				sz = formatSize(info.Size())
			}
			fmt.Fprintf(c.out, "%s %s %s %s\n", marker, color.Faint(fmt.Sprintf("#%d", i)), color.Faint(ts), color.Faint(sz))
		}
		fmt.Fprintf(c.out, "\n%s %s\n", color.Faint("Total:"), color.Bold(fmt.Sprintf("%d backups", len(backups))))
		return nil
	}

	// --- restore ---
	if len(backups) == 0 {
		return &UserError{Msg: "No backups available to restore."}
	}

	idx := cmd.Index
	if idx < 0 {
		idx = 0
	}
	if idx >= len(backups) {
		return &UserError{Msg: fmt.Sprintf("Backup index %d out of range (0-%d).", idx, len(backups)-1)}
	}

	backup := backups[idx]
	stat, err := os.Stat(backup)
	if err != nil {
		return &UserError{Msg: fmt.Sprintf("Backup not accessible: %s", backup)}
	}

	targetPath := c.cfg.Database
	if cmd.RestoreTo != "" {
		targetPath = cmd.RestoreTo
	}

	fmt.Fprintf(c.out, "%s %s\n", color.Yellow("Backup:"), color.Faint(backup))
	fmt.Fprintf(c.out, "%s %s\n", color.Faint("Modified:"), stat.ModTime().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(c.out, "%s %s\n", color.Faint("Size:"), formatSize(stat.Size()))
	if cmd.RestoreTo != "" {
		fmt.Fprintf(c.out, "%s %s\n", color.Faint("Restore to:"), targetPath)
	}

	if !cmd.Force {
		action := "Restore this backup"
		if cmd.RestoreTo != "" {
			action = "Restore to " + targetPath
		}
		ok, err := confirm(c, action,
			fmt.Sprintf("File: %s", backup),
			fmt.Sprintf("Modified: %s", stat.ModTime().Format("2006-01-02 15:04:05")),
			fmt.Sprintf("Size: %s", formatSize(stat.Size())),
		)
		if err != nil {
			return &UserError{Msg: err.Error()}
		}
		if !ok {
			return &UserError{Msg: "Aborted."}
		}
	}

	if err := db.RestoreBackup(backup, targetPath); err != nil {
		return &UserError{Msg: err.Error()}
	}
	fmt.Fprintln(c.out, color.Green("Database restored from backup."))
	return nil
}

// runPrune deletes old backups, keeping the most recent `keep` (0 = delete all).
func runPrune(c *ctx, backups []string, keep int, force bool) error {
	if len(backups) == 0 {
		fmt.Fprintln(c.out, color.Faint("No backups to prune."))
		return nil
	}

	var toDelete []string
	if keep == 0 {
		toDelete = backups
	} else if keep < len(backups) {
		toDelete = backups[keep:]
	}

	if len(toDelete) == 0 {
		fmt.Fprintf(c.out, "%s %d %s\n", color.Faint("Only"), len(backups), color.Faint("backups — nothing to prune."))
		return nil
	}

	if !force {
		var details []string
		for _, b := range toDelete {
			info, _ := os.Stat(b)
			ts := ""
			if info != nil {
				ts = info.ModTime().Format("2006-01-02 15:04")
			}
			details = append(details, fmt.Sprintf("%s  %s", color.Faint(ts), b))
		}
		action := fmt.Sprintf("Delete %d old backups", len(toDelete))
		if keep == 0 {
			action = fmt.Sprintf("Delete all %d backups", len(toDelete))
		} else {
			details = append([]string{color.Faint(fmt.Sprintf("Keeping last %d", keep))}, details...)
		}
		ok, err := confirm(c, action, details...)
		if err != nil {
			return &UserError{Msg: err.Error()}
		}
		if !ok {
			return &UserError{Msg: "Aborted."}
		}
	}

	deleted := 0
	for _, b := range toDelete {
		if err := os.Remove(b); err != nil {
			fmt.Fprintf(c.errw, "%s %s: %v\n", color.Red("Failed to delete"), b, err)
			continue
		}
		deleted++
	}

	if keep == 0 {
		fmt.Fprintf(c.out, "%s\n", color.Green(fmt.Sprintf("Deleted all %d backups.", deleted)))
	} else {
		fmt.Fprintf(c.out, "%s %d %s\n", color.Green(fmt.Sprintf("Deleted %d backups,", deleted)), keep, color.Faint("kept."))
	}
	return nil
}

func formatSize(bytes int64) string {
	switch {
	case bytes < 1024:
		return fmt.Sprintf("%d B", bytes)
	case bytes < 1024*1024:
		return fmt.Sprintf("%.1f KiB", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%.1f MiB", float64(bytes)/(1024*1024))
	}
}
