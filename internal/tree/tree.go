package tree

import (
	"fmt"
	"sort"
	"strings"

	"github.com/irasikhin/kpass/internal/color"
	"github.com/irasikhin/kpass/internal/runtimex"
)

// EntryInfo carries the metadata needed for rich tree rendering.
type EntryInfo struct {
	Path        string
	Username    string
	URL         string
	Tags        string
	HasPassword bool
	HasURL      bool
	HasNotes    bool
	HasOTP      bool
	AttachCount int
	Suffix      string // arbitrary dimmed suffix appended after indicators
}

// Indicators returns the icon string for this entry.
func (e *EntryInfo) Indicators() string {
	var parts []string
	if e.HasPassword {
		parts = append(parts, "🔑")
	}
	if e.HasURL {
		parts = append(parts, "🔗")
	}
	if e.HasOTP {
		parts = append(parts, "⏱")
	}
	if e.HasNotes {
		parts = append(parts, "📝")
	}
	if e.AttachCount > 0 {
		parts = append(parts, fmt.Sprintf("📎%d", e.AttachCount))
	}
	if e.Suffix != "" {
		parts = append(parts, e.Suffix)
	}
	if len(parts) == 0 {
		return ""
	}
	return " " + strings.Join(parts, "")
}

type richNode struct {
	children   map[string]*richNode
	entryCount int    // count of leaf entries under this node
	indicator  string // indicator string if this is a leaf entry
}

func newRichNode() *richNode { return &richNode{children: map[string]*richNode{}} }

func buildRich(entries []*EntryInfo) *richNode {
	root := newRichNode()
	for _, e := range entries {
		cur := root
		parts := runtimex.SplitPath(e.Path)
		for i, part := range parts {
			child, ok := cur.children[part]
			if !ok {
				child = newRichNode()
				cur.children[part] = child
			}
			cur = child
			if i == len(parts)-1 {
				cur.indicator = e.Indicators()
			}
		}
	}
	// Compute entry counts bottom-up.
	computeCounts(root)
	return root
}

func computeCounts(n *richNode) int {
	count := 0
	for _, child := range n.children {
		if len(child.children) == 0 {
			count++ // leaf = one entry
		} else {
			count += computeCounts(child)
		}
	}
	n.entryCount = count
	return count
}

// RenderRich renders a tree with entry indicators and group counts.
// depth=0 means unlimited.
func RenderRich(entries []*EntryInfo, rootLabel string, depth int) string {
	var b strings.Builder
	b.WriteString(color.Bold(rootLabel))
	root := buildRich(entries)
	walkRich(&b, root, "", 0, depth)
	return b.String()
}

// RenderLong renders entries as a table: PATH, USER, URL, TOTP, ATTACH.
func RenderLong(entries []*EntryInfo) string {
	// Compute column widths.
	maxPath, maxUser, maxURL := 0, 0, 0
	for _, e := range entries {
		if len(e.Path) > maxPath {
			maxPath = len(e.Path)
		}
		if len(e.Username) > maxUser {
			maxUser = len(e.Username)
		}
		if len(e.URL) > maxURL {
			maxURL = len(e.URL)
		}
	}
	if maxPath < 4 {
		maxPath = 4
	}
	if maxUser < 4 {
		maxUser = 4
	}
	if maxURL < 3 {
		maxURL = 3
	}

	header := fmt.Sprintf("%-*s  %-*s  %-*s  %-4s  %-6s  %s",
		maxPath, "PATH", maxUser, "USER", maxURL, "URL", "TOTP", "ATTACH", "TAGS")
	sep := strings.Repeat("─", len(header))

	var b strings.Builder
	b.WriteString(color.Faint(header))
	b.WriteByte('\n')
	b.WriteString(color.Faint(sep))

	for _, e := range entries {
		totp := "  —"
		if e.HasOTP {
			totp = "  ✓"
		}
		att := "    —"
		if e.AttachCount > 0 {
			att = fmt.Sprintf("  %2d", e.AttachCount)
		}
		b.WriteByte('\n')
		fmt.Fprintf(&b, "%-*s  %-*s  %-*s  %s  %s  %s",
			maxPath, e.Path,
			maxUser, e.Username,
			maxURL, truncate(e.URL, maxURL),
			totp, att, e.Tags)
	}
	return b.String()
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func walkRich(b *strings.Builder, n *richNode, prefix string, currentDepth int, maxDepth int) {
	names := sortedChildNames(n.children)
	for i, name := range names {
		isLast := i == len(names)-1
		connector := "├── "
		nextPrefix := prefix + "│   "
		if isLast {
			connector = "└── "
			nextPrefix = prefix + "    "
		}
		b.WriteByte('\n')
		b.WriteString(color.Faint(prefix))
		b.WriteString(color.Faint(connector))
		child := n.children[name]
		if len(child.children) == 0 {
			// Leaf entry.
			b.WriteString(color.Bold(name))
			b.WriteString(color.Faint(child.indicator))
		} else {
			// Group node.
			b.WriteString(name)
			if child.entryCount > 0 {
				b.WriteString(color.Faint(fmt.Sprintf(" (%d)", child.entryCount)))
			}
			if maxDepth == 0 || currentDepth+1 < maxDepth {
				walkRich(b, child, nextPrefix, currentDepth+1, maxDepth)
			}
		}
	}
}

func sortedChildNames[V any](m map[string]V) []string {
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	sort.Slice(names, func(i, j int) bool {
		return strings.ToLower(names[i]) < strings.ToLower(names[j])
	})
	return names
}
