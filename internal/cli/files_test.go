package cli

import "os"

// writeFileErr returns an error rather than calling t.Fatal — used by hook
// callbacks where t isn't in scope.
func writeFileErr(path, contents string) error {
	return os.WriteFile(path, []byte(contents), 0o600)
}
