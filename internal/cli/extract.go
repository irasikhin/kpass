package cli

import "strings"

// extractSelector locates the first `@profile` token in argv, extracts it, and
// returns the remaining argv plus the profile name (without @).
//
// The selector must not immediately follow a flag-like token (starting with
// "-"), as that would indicate a flag value rather than a profile selector.
func extractSelector(argv []string) ([]string, string, error) {
	for i, token := range argv {
		if !strings.HasPrefix(token, "@") {
			continue
		}
		if token == "@" {
			return nil, "", &UserError{Msg: "Database selector cannot be empty."}
		}
		// Skip if the previous token looks like a flag that takes a value
		// (e.g. --database @work or -d @work).
		if i > 0 && strings.HasPrefix(argv[i-1], "-") {
			continue
		}
		out := append([]string{}, argv[:i]...)
		out = append(out, argv[i+1:]...)
		return out, token[1:], nil
	}
	return argv, "", nil
}
