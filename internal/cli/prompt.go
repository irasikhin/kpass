package cli

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/irasikhin/kpass/internal/color"
)

// confirm asks the user a y/N question on stderr-style output (c.out) and
// reads a single line from c.in. Returns true when the reply starts with 'y'
// (case-insensitive), or when the global --yes flag is set. action describes
// what is about to happen (e.g. "Delete", "Proceed", "Apply"). details, if
// non-empty, are printed as dimmed context lines before the prompt.
func confirm(c *ctx, action string, details ...string) (bool, error) {
	if c.gf.yes {
		if len(details) > 0 {
			fmt.Fprintf(c.out, "\n%s: %s\n", color.Yellow(action), color.Faint("(auto-yes)"))
			for _, d := range details {
				fmt.Fprintf(c.out, "  %s\n", color.Faint(d))
			}
		} else {
			fmt.Fprintf(c.out, "\n%s %s\n", color.Yellow(action), color.Faint("(auto-yes)"))
		}
		return true, nil
	}

	if len(details) > 0 {
		fmt.Fprintf(c.out, "\n%s:\n", color.Yellow(action))
		for _, d := range details {
			fmt.Fprintf(c.out, "  %s\n", color.Faint(d))
		}
	} else {
		fmt.Fprintf(c.out, "\n%s?", color.Yellow(action))
	}
	fmt.Fprintf(c.out, "\n%s ", color.Faint("[y/N]:"))

	reader := bufio.NewReader(c.in)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	reply := strings.ToLower(strings.TrimSpace(line))
	return reply == "y" || reply == "yes", nil
}
