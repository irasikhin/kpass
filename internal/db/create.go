package db

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tobischo/gokeepasslib/v3"

	"github.com/irasikhin/kpass/internal/runtimex"
)

// Create initializes a new empty KeePass KDBX v4 database at `path`
// protected by `password`. If the file already exists, Create returns an error
// (use --force to overwrite).
func Create(path, password, keyFile string) error {
	expanded := runtimex.ExpandPath(path)
	if expanded == "" {
		return fmt.Errorf("database path cannot be empty")
	}

	if _, err := os.Stat(expanded); err == nil {
		return fmt.Errorf("database already exists: %s (use --force to overwrite)", expanded)
	}

	dir := filepath.Dir(expanded)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("cannot create directory %s: %w", dir, err)
	}

	creds, err := buildCreds(password, keyFile)
	if err != nil {
		return err
	}

	db := gokeepasslib.NewDatabase(
		gokeepasslib.WithDatabaseKDBXVersion4(),
	)
	db.Credentials = creds

	// Name the root group after the database file (without extension).
	rootName := "Root"
	if base := filepath.Base(expanded); len(base) > 0 {
		if ext := filepath.Ext(base); len(ext) > 0 {
			rootName = base[:len(base)-len(ext)]
		} else {
			rootName = base
		}
	}
	root := db.Content.Root.Groups[0]
	root.Name = rootName

	f, err := os.Create(expanded)
	if err != nil {
		return fmt.Errorf("cannot create database file: %w", err)
	}
	defer f.Close()

	if err := gokeepasslib.NewEncoder(f).Encode(db); err != nil {
		return fmt.Errorf("cannot write database: %w", err)
	}

	return nil
}
