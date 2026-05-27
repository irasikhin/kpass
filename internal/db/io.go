package db

import (
	"fmt"
	"os"
)

func openFile(path string) (*os.File, error) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return nil, fmt.Errorf("KeePass database not found: %s", path)
	}
	return os.Open(path)
}
