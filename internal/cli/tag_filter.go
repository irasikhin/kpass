package cli

import (
	"strings"

	"github.com/irasikhin/kpass/internal/db"
)

// matchTagFilter reports whether e satisfies the AND/OR tag filter combo.
//   - allOf: entry must carry every listed tag (case-insensitive).
//   - anyOf: entry must carry at least one of the listed tags.
//
// Empty slices skip that check. An empty entry tag set fails any non-empty
// filter. Comparison is case-insensitive.
func matchTagFilter(e *db.Entry, allOf, anyOf []string) bool {
	if len(allOf) == 0 && len(anyOf) == 0 {
		return true
	}
	entryTags := e.Tags()
	if len(entryTags) == 0 {
		return false
	}
	set := make(map[string]bool, len(entryTags))
	for _, t := range entryTags {
		set[strings.ToLower(t)] = true
	}
	for _, want := range allOf {
		if !set[strings.ToLower(want)] {
			return false
		}
	}
	if len(anyOf) > 0 {
		ok := false
		for _, want := range anyOf {
			if set[strings.ToLower(want)] {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}
