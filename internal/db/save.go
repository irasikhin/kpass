package db

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Save re-locks protected entries and writes the database back to its source
// path. Creates a timestamped backup before overwriting. After saving,
// protected entries are re-unlocked so the caller can continue using the DB.
func (d *DB) Save() error {
	// Auto-backup before destructive write. Non-fatal: a save without a
	// backup is still better than no save at all, but surface a warning
	// to stderr so the user knows the safety net is missing on this write.
	if _, err := d.Backup(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: backup failed before save (%v); proceeding anyway\n", err)
	}

	// Auto-prune old backups (non-fatal).
	if d.BackupKeep > 0 || d.BackupMaxAgeDays > 0 {
		_ = d.pruneOldBackups()
	}

	if err := lockProtectedFn(d.Raw); err != nil {
		return fmt.Errorf("failed to lock protected entries: %w", err)
	}
	tmp, err := osCreateTempFn(filepathDir(d.Path), ".kpass-save-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if err := osChmodFileFn(tmp, 0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	defer func() {
		if tmpPath != "" {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := encodeFn(tmp, d.Raw); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := osRenameFn(tmpPath, d.Path); err != nil {
		return err
	}
	tmpPath = ""
	// Re-unlock so callers can keep operating on the in-memory DB.
	return unlockProtectedFn(d.Raw)
}

// Backup creates a timestamped backup of the database file before a
// destructive operation. Returns the backup path.
func (d *DB) Backup() (string, error) {
	ts := time.Now().Format("20060102-150405")
	ext := filepath.Ext(d.Path)
	base := strings.TrimSuffix(d.Path, ext)
	backupPath := fmt.Sprintf("%s.%s.bak", base, ts)

	src, err := osOpenFn(d.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // nothing to back up yet
		}
		return "", err
	}
	defer src.Close()

	dst, err := osOpenFileFn(backupPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	if _, err := ioCopyFn(dst, src); err != nil {
		return "", err
	}
	return backupPath, nil
}

// ListBackups returns paths of all .bak files for this database, sorted
// newest first.
func (d *DB) ListBackups() ([]string, error) {
	ext := filepath.Ext(d.Path)
	base := strings.TrimSuffix(d.Path, ext)
	pattern := base + ".*.bak"
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	sort.Sort(sort.Reverse(sort.StringSlice(matches)))
	return matches, nil
}

// RestoreBackup copies the backup file over the current database.
func RestoreBackup(backupPath, dbPath string) error {
	src, err := osOpenFn(backupPath)
	if err != nil {
		return fmt.Errorf("cannot open backup: %w", err)
	}
	defer src.Close()

	dst, err := osOpenFileFn(dbPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("cannot write database: %w", err)
	}
	defer dst.Close()
	if err := osChmodFileFn(dst, 0o600); err != nil {
		return fmt.Errorf("cannot set database permissions: %w", err)
	}

	if _, err := ioCopyFn(dst, src); err != nil {
		return err
	}
	return nil
}

// pruneOldBackups deletes .bak files exceeding BackupKeep count or
// BackupMaxAgeDays age. Called automatically from Save() when configured.
func (d *DB) pruneOldBackups() error {
	backups, err := d.ListBackups()
	if err != nil || len(backups) == 0 {
		return err
	}

	// Determine which backups to delete.
	toDelete := map[string]bool{}
	now := time.Now()

	for i, b := range backups {
		// Count-based: keep only first N.
		if d.BackupKeep > 0 && i >= d.BackupKeep {
			toDelete[b] = true
		}
		// Age-based: delete older than N days.
		if d.BackupMaxAgeDays > 0 {
			info, err := os.Stat(b)
			if err == nil {
				age := now.Sub(info.ModTime())
				if age.Hours() > float64(d.BackupMaxAgeDays*24) {
					toDelete[b] = true
				}
			}
		}
	}

	for b := range toDelete {
		_ = os.Remove(b)
	}
	return nil
}
