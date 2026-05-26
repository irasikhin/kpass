package cli

import (
	"encoding/json"
	"fmt"

	"github.com/irasikhin/kpass/internal/runtimex"
)

// DuplicateCmd makes an independent copy of an entry at a new path. The
// underlying db helper is db.CloneEntry — only the user-facing name changed.
type DuplicateCmd struct {
	Source      string `arg:"" help:"Source entry path or partial path."`
	Destination string `arg:"" help:"Destination entry path for the duplicate."`
	Force       bool   `short:"f" help:"Overwrite an existing destination."`
	JSON        bool   `help:"Output as JSON."`
}

func (cmd *DuplicateCmd) Run(c *ctx) error {
	if err := c.openDatabase(); err != nil {
		return err
	}
	srcEntry, err := c.db.ResolveEntry(cmd.Source)
	if err != nil {
		return err
	}
	sourcePath := srcEntry.DisplayPath()

	destination := runtimex.NormalizePath(cmd.Destination)
	if destination == "" {
		return &UserError{Msg: "Destination path cannot be empty."}
	}
	for _, candidate := range c.db.SortedEntries() {
		if candidate.DisplayPath() == destination && !cmd.Force {
			return &UserError{Msg: fmt.Sprintf("Destination already exists: %s", destination)}
		}
	}

	parts := runtimex.SplitPath(destination)
	c.db.EnsureGroup(runtimex.JoinPath(parts[:len(parts)-1]))
	entry, err := c.db.ResolveEntry(sourcePath)
	if err != nil {
		return err
	}
	dstGroup, err := c.db.ResolveGroup(runtimex.JoinPath(parts[:len(parts)-1]))
	if err != nil {
		return &UserError{Msg: err.Error()}
	}
	c.db.CloneEntry(entry, dstGroup, parts[len(parts)-1])

	if err := c.db.Save(); err != nil {
		return &UserError{Msg: err.Error()}
	}

	if cmd.JSON {
		data, _ := json.Marshal(map[string]any{
			"status": "ok",
			"source": sourcePath,
			"target": destination,
		})
		fmt.Fprintln(c.out, string(data))
		return nil
	}

	fmt.Fprintln(c.out, destination)
	return nil
}
