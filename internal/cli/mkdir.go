package cli

import (
	"encoding/json"
	"fmt"

	"github.com/irasikhin/kpass/internal/runtimex"
)

// MkdirCmd creates a group path (intermediate groups included).
type MkdirCmd struct {
	Group string `arg:"" help:"Group path to create (e.g. work/projects)."`
	JSON  bool   `help:"Output as JSON."`
}

func (cmd *MkdirCmd) Run(c *ctx) error {
	path := runtimex.NormalizePath(cmd.Group)
	if path == "" {
		return &UserError{Msg: "Group path cannot be empty."}
	}
	if err := c.openDatabase(); err != nil {
		return err
	}
	c.db.EnsureGroup(path)
	if err := c.db.Save(); err != nil {
		return &UserError{Msg: err.Error()}
	}

	if cmd.JSON {
		data, _ := json.Marshal(map[string]any{
			"status": "ok",
			"path":   path,
		})
		fmt.Fprintln(c.out, string(data))
		return nil
	}

	fmt.Fprintln(c.out, path)
	return nil
}
