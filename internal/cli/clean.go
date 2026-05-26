package cli

import (
	"encoding/json"
	"fmt"

	"github.com/irasikhin/kpass/internal/color"
)

// CleanCmd removes empty groups from the database.
type CleanCmd struct {
	Force  bool `short:"f" help:"Skip confirmation prompts."`
	JSON   bool `help:"Output as JSON."`
	DryRun bool `name:"dry-run" help:"Show empty groups without removing them."`
}

func (cmd *CleanCmd) Run(c *ctx) error {
	if err := c.openDatabase(); err != nil {
		return err
	}

	empty := c.db.EmptyGroups()
	if len(empty) == 0 {
		if cmd.JSON {
			fmt.Fprintln(c.out, `{"removed":[],"count":0}`)
		} else {
			fmt.Fprintln(c.out, color.Green("No empty groups found."))
		}
		return nil
	}

	if cmd.DryRun {
		if cmd.JSON {
			data, _ := json.Marshal(map[string]any{
				"would_remove": empty,
				"count":        len(empty),
			})
			fmt.Fprintln(c.out, string(data))
		} else {
			fmt.Fprintf(c.out, "%s %s\n", color.Yellow("Would remove"), color.Bold(fmt.Sprintf("%d empty groups", len(empty))))
			for _, g := range empty {
				fmt.Fprintf(c.out, "  %s\n", color.Faint(g))
			}
		}
		return nil
	}

	if !cmd.Force {
		ok, err := confirm(c, fmt.Sprintf("Remove %d empty groups", len(empty)), empty...)
		if err != nil {
			return &UserError{Msg: err.Error()}
		}
		if !ok {
			return &UserError{Msg: "Aborted."}
		}
	}

	removed := 0
	for _, g := range empty {
		if err := c.db.RemoveGroup(g); err != nil {
			return &UserError{Msg: err.Error()}
		}
		removed++
	}

	if err := c.db.Save(); err != nil {
		return &UserError{Msg: err.Error()}
	}

	if cmd.JSON {
		fmt.Fprintf(c.out, "{\"removed\":%d,\"count\":%d}\n", removed, len(empty))
	} else {
		fmt.Fprintf(c.out, "%s\n", color.Green(fmt.Sprintf("Removed %d empty group(s).", removed)))
	}
	return nil
}
