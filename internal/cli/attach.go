package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/irasikhin/kpass/internal/color"
	"github.com/irasikhin/kpass/internal/config"
	"github.com/irasikhin/kpass/internal/runtimex"
)

// AttachCmd is a noun-of-actions for entry attachments.
type AttachCmd struct {
	Ls      AttachLsCmd      `cmd:"" help:"List attachments on an entry."`
	Add     AttachAddCmd     `cmd:"" help:"Add (or replace, with -f) an attachment."`
	Remove  AttachRemoveCmd  `cmd:"" help:"Remove an attachment by name."`
	Extract AttachExtractCmd `cmd:"" help:"Write an attachment to disk."`
}

type AttachLsCmd struct {
	Entry string `arg:"" help:"Entry to list attachments for."`
}

func (cmd *AttachLsCmd) Run(c *ctx) error {
	if err := c.openDatabase(); err != nil {
		return err
	}
	entry, err := c.db.ResolveEntry(cmd.Entry)
	if err != nil {
		return err
	}
	for _, name := range entry.AttachmentList() {
		fmt.Fprintln(c.out, name)
	}
	return nil
}

type AttachAddCmd struct {
	Entry string `arg:"" help:"Entry to attach the file to."`
	File  string `arg:"" help:"Path to the file to attach."`
	Name  string `help:"Stored attachment name (defaults to the source basename)."`
	Force bool   `short:"f" help:"Replace an existing attachment with the same name."`
}

func (cmd *AttachAddCmd) Run(c *ctx) error {
	sourcePath := runtimex.ExpandPath(cmd.File)
	info, err := os.Stat(sourcePath)
	if err != nil || info.IsDir() {
		return &UserError{Msg: fmt.Sprintf("Attachment file not found: %s", sourcePath)}
	}
	if info.Size() >= config.LargeAttachmentWarnBytes {
		fmt.Fprintf(c.errw, "%s attachment '%s' is %.1f MiB and will increase the KeePass database size.\n",
			color.Yellow("Warning:"), filepath.Base(sourcePath), float64(info.Size())/float64(1024*1024))
	}
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return &UserError{Msg: err.Error()}
	}

	if err := c.openDatabase(); err != nil {
		return err
	}
	entry, err := c.db.ResolveEntry(cmd.Entry)
	if err != nil {
		return err
	}
	attachmentName := cmd.Name
	if attachmentName == "" {
		attachmentName = filepath.Base(sourcePath)
	}
	if err := entry.AddAttachment(attachmentName, data, cmd.Force); err != nil {
		return &UserError{Msg: err.Error()}
	}
	if err := c.db.Save(); err != nil {
		return &UserError{Msg: err.Error()}
	}
	fmt.Fprintf(c.out, "%s %s %s %s\n", color.Green("Added attachment"), color.Bold(attachmentName), color.Faint("to"), color.Bold(entry.DisplayPath()))
	return nil
}

type AttachRemoveCmd struct {
	Entry    string `arg:"" help:"Entry holding the attachment."`
	Filename string `arg:"" help:"Stored attachment name to remove."`
}

func (cmd *AttachRemoveCmd) Run(c *ctx) error {
	if err := c.openDatabase(); err != nil {
		return err
	}
	entry, err := c.db.ResolveEntry(cmd.Entry)
	if err != nil {
		return err
	}
	if err := entry.RemoveAttachment(cmd.Filename); err != nil {
		return &UserError{Msg: err.Error()}
	}
	if err := c.db.Save(); err != nil {
		return &UserError{Msg: err.Error()}
	}
	fmt.Fprintf(c.out, "%s %s %s %s\n", color.Green("Removed attachment"), color.Bold(cmd.Filename), color.Faint("from"), color.Bold(entry.DisplayPath()))
	return nil
}

type AttachExtractCmd struct {
	Entry    string `arg:"" help:"Entry holding the attachment."`
	Filename string `arg:"" help:"Stored attachment name to extract."`
	Output   string `arg:"" optional:"" help:"Destination file (or directory). Defaults to the attachment name."`
	Force    bool   `short:"f" help:"Overwrite an existing output file."`
}

func (cmd *AttachExtractCmd) Run(c *ctx) error {
	if err := c.openDatabase(); err != nil {
		return err
	}
	entry, err := c.db.ResolveEntry(cmd.Entry)
	if err != nil {
		return err
	}
	data, err := entry.AttachmentContent(cmd.Filename)
	if err != nil {
		return &UserError{Msg: err.Error()}
	}
	outputPath := cmd.Filename
	if cmd.Output != "" {
		outputPath = runtimex.ExpandPath(cmd.Output)
	}
	if info, err := os.Stat(outputPath); err == nil && info.IsDir() {
		outputPath = filepath.Join(outputPath, cmd.Filename)
	}
	if _, err := os.Stat(outputPath); err == nil && !cmd.Force {
		return &UserError{Msg: fmt.Sprintf("Output file already exists: %s. Use --force to overwrite it.", outputPath)}
	}
	if dir := filepath.Dir(outputPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return &UserError{Msg: err.Error()}
		}
	}
	if err := os.WriteFile(outputPath, data, 0o600); err != nil {
		return &UserError{Msg: err.Error()}
	}
	fmt.Fprintln(c.out, outputPath)
	return nil
}
