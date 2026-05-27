package db

import (
	"io"
	"os"

	"github.com/tobischo/gokeepasslib/v3"
)

// Filesystem/library seams used by unit tests to drive error paths that the
// real OS or gokeepasslib will never produce on valid inputs.
var (
	osCreateTempFn = os.CreateTemp
	osRenameFn     = os.Rename
	osOpenFileFn   = os.OpenFile
	osOpenFn       = os.Open
	osStatFn       = os.Stat
	osChmodFileFn  = func(f *os.File, mode os.FileMode) error { return f.Chmod(mode) }
	ioCopyFn       = io.Copy
	encodeFn       = func(w io.Writer, raw *gokeepasslib.Database) error {
		return gokeepasslib.NewEncoder(w).Encode(raw)
	}
	lockProtectedFn   = func(raw *gokeepasslib.Database) error { return raw.LockProtectedEntries() }
	unlockProtectedFn = func(raw *gokeepasslib.Database) error { return raw.UnlockProtectedEntries() }
)
