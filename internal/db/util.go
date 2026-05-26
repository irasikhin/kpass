package db

import "path/filepath"

func filepathDir(p string) string {
	if p == "" {
		return "."
	}
	return filepath.Dir(p)
}
