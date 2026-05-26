package cli

import (
	"encoding/json"
	"fmt"

	"github.com/irasikhin/kpass/internal/runtimex"
)

// MoveCmd moves or renames an entry to a new path.
type MoveCmd struct {
	Source      string `arg:"" help:"Source entry path or partial path."`
	Destination string `arg:"" help:"Destination entry path."`
	Force       bool   `short:"f" help:"Overwrite an existing destination."`
	JSON        bool   `help:"Output as JSON."`
}

func (cmd *MoveCmd) Run(c *ctx) error {
	if err := c.openDatabase(); err != nil {
		return err
	}
	// Validate source up front, but only capture its path — pointers obtained
	// before EnsureGroup mutations can go stale across slice reallocations.
	sourceEntry, err := c.db.ResolveEntry(cmd.Source)
	if err != nil {
		return err
	}
	sourcePath := sourceEntry.DisplayPath()

	destination := runtimex.NormalizePath(cmd.Destination)
	if destination == "" {
		return &UserError{Msg: "Destination path cannot be empty."}
	}
	for _, candidate := range c.db.SortedEntries() {
		if candidate.DisplayPath() == sourcePath {
			continue
		}
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

	moved := entry
	if entry.Parent() != dstGroup.Raw() {
		moved, err = c.db.MoveEntry(entry, dstGroup)
		if err != nil {
			return &UserError{Msg: err.Error()}
		}
	}
	moved.SetTitle(parts[len(parts)-1])

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
