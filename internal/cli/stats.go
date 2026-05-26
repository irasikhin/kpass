package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/irasikhin/kpass/internal/color"
)

// StatsCmd prints database statistics: entry count, group count, tag count,
// file size, and last modified time.
type StatsCmd struct {
	JSON bool `help:"Output as JSON."`
}

func (cmd *StatsCmd) Run(c *ctx) error {
	if err := c.openDatabase(); err != nil {
		return err
	}

	entries := c.db.SortedEntries()
	groups := c.db.SortedGroups()
	tags := c.db.AllTags()

	// Count entries with specific features.
	var withPassword, withURL, withOTP, withNotes, withAttach int
	for _, e := range entries {
		if e.Raw().GetPassword() != "" {
			withPassword++
		}
		if e.Raw().GetContent("URL") != "" {
			withURL++
		}
		if e.OtpURI() != "" {
			withOTP++
		}
		if e.Raw().GetContent("Notes") != "" {
			withNotes++
		}
		if len(e.Raw().Binaries) > 0 {
			withAttach++
		}
	}

	fileInfo, err := os.Stat(c.cfg.Database)
	var fileSize int64
	var fileModTime string
	if err == nil {
		fileSize = fileInfo.Size()
		fileModTime = fileInfo.ModTime().Format("2006-01-02 15:04:05")
	}

	// Count weak passwords.
	weakCount := 0
	for _, e := range entries {
		pw := e.Raw().GetPassword()
		if pw != "" && weakPasswordReason(pw) != "" {
			weakCount++
		}
	}

	// Top tags by count.
	type tagPair struct {
		Tag   string `json:"tag"`
		Count int    `json:"count"`
	}
	tagPairs := make([]tagPair, 0, len(tags))
	for t, n := range tags {
		tagPairs = append(tagPairs, tagPair{Tag: t, Count: n})
	}
	sort.Slice(tagPairs, func(i, j int) bool {
		if tagPairs[i].Count != tagPairs[j].Count {
			return tagPairs[i].Count > tagPairs[j].Count
		}
		return tagPairs[i].Tag < tagPairs[j].Tag
	})
	topTags := tagPairs
	if len(topTags) > 5 {
		topTags = topTags[:5]
	}

	if cmd.JSON {
		data, _ := json.Marshal(map[string]any{
			"database":       c.cfg.Database,
			"size_bytes":     fileSize,
			"modified":       fileModTime,
			"entries":        len(entries),
			"groups":         len(groups),
			"tags":           len(tags),
			"with_password":  withPassword,
			"with_url":       withURL,
			"with_otp":       withOTP,
			"with_notes":     withNotes,
			"with_attach":    withAttach,
			"weak_passwords": weakCount,
			"top_tags":       topTags,
		})
		fmt.Fprintln(c.out, string(data))
		return nil
	}

	fmt.Fprintf(c.out, "%s %s\n", color.Bold("Database:"), c.cfg.Database)
	if fileSize > 0 {
		fmt.Fprintf(c.out, "  %s %s (%s)\n", color.Faint("File:"), formatSize(fileSize), fileModTime)
	}
	fmt.Fprintf(c.out, "\n")
	fmt.Fprintf(c.out, "  %s %d\n", color.Cyan("Entries:"), len(entries))
	fmt.Fprintf(c.out, "  %s %d\n", color.Cyan("Groups:"), len(groups))
	fmt.Fprintf(c.out, "  %s %d\n", color.Cyan("Unique tags:"), len(tags))
	fmt.Fprintf(c.out, "\n")
	fmt.Fprintf(c.out, "  %s %d\n", color.Faint("With password:"), withPassword)
	fmt.Fprintf(c.out, "  %s %d\n", color.Faint("With URL:"), withURL)
	fmt.Fprintf(c.out, "  %s %d\n", color.Faint("With OTP:"), withOTP)
	fmt.Fprintf(c.out, "  %s %d\n", color.Faint("With notes:"), withNotes)
	fmt.Fprintf(c.out, "  %s %d\n", color.Faint("With attachments:"), withAttach)
	if weakCount > 0 {
		fmt.Fprintf(c.out, "  %s %d\n", color.Red("Weak passwords:"), weakCount)
	}

	if len(topTags) > 0 {
		fmt.Fprintf(c.out, "\n%s\n", color.Faint("Top tags:"))
		for _, p := range topTags {
			fmt.Fprintf(c.out, "  %s %s\n", color.Bold(p.Tag), color.Faint(fmt.Sprintf("(%d)", p.Count)))
		}
	}

	return nil
}
