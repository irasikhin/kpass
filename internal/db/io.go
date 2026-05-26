package db

import (
	"fmt"
	"os"
	"strings"
)

func openFile(path string) (*os.File, error) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return nil, fmt.Errorf("KeePass database not found: %s", path)
	}
	return os.Open(path)
}

func readPasswordFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("password file not found: %s", path)
	}
	s := string(data)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i], nil
	}
	return s, nil
}
