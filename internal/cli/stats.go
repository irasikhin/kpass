package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/irasikhin/kpass/internal/color"
	"github.com/irasikhin/kpass/internal/db"
)

// StatsCmd prints database statistics: entry count, group count, tag count,
// file size, and last modified time.
type StatsCmd struct {
	JSON bool `help:"Output as JSON."`
}

type tagPair struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// dbStats is the data model shared by the JSON and text renderers.
type dbStats struct {
	Database     string
	FileSize     int64
	FileModTime  string
	EntryCount   int
	GroupCount   int
	TagCount     int
	WithPassword int
	WithURL      int
	WithOTP      int
	WithNotes    int
	WithAttach   int
	WeakCount    int
	TopTags      []tagPair
}

func (cmd *StatsCmd) Run(c *ctx) error {
	if err := c.openDatabase(); err != nil {
		return err
	}
	stats := collectDbStats(c)
	if cmd.JSON {
		return stats.renderJSON(c)
	}
	return stats.renderText(c)
}

func collectDbStats(c *ctx) dbStats {
	entries := c.db.SortedEntries()
	groups := c.db.SortedGroups()
	tags := c.db.AllTags()

	stats := dbStats{
		Database:   c.cfg.Database,
		EntryCount: len(entries),
		GroupCount: len(groups),
		TagCount:   len(tags),
		TopTags:    topTagsByCount(tags, 5),
	}
	stats.countFeatures(entries)
	if info, err := os.Stat(c.cfg.Database); err == nil {
		stats.FileSize = info.Size()
		stats.FileModTime = info.ModTime().Format("2006-01-02 15:04:05")
	}
	return stats
}

func (s *dbStats) countFeatures(entries []*db.Entry) {
	for _, e := range entries {
		raw := e.Raw()
		pw := raw.GetPassword()
		if pw != "" {
			s.WithPassword++
			if weakPasswordReason(pw) != "" {
				s.WeakCount++
			}
		}
		if raw.GetContent("URL") != "" {
			s.WithURL++
		}
		if e.OtpURI() != "" {
			s.WithOTP++
		}
		if raw.GetContent("Notes") != "" {
			s.WithNotes++
		}
		if len(raw.Binaries) > 0 {
			s.WithAttach++
		}
	}
}

func topTagsByCount(tags map[string]int, limit int) []tagPair {
	pairs := make([]tagPair, 0, len(tags))
	for t, n := range tags {
		pairs = append(pairs, tagPair{Tag: t, Count: n})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].Count != pairs[j].Count {
			return pairs[i].Count > pairs[j].Count
		}
		return pairs[i].Tag < pairs[j].Tag
	})
	if len(pairs) > limit {
		pairs = pairs[:limit]
	}
	return pairs
}

func (s dbStats) renderJSON(c *ctx) error {
	data, _ := json.Marshal(map[string]any{
		"database":       s.Database,
		"size_bytes":     s.FileSize,
		"modified":       s.FileModTime,
		"entries":        s.EntryCount,
		"groups":         s.GroupCount,
		"tags":           s.TagCount,
		"with_password":  s.WithPassword,
		"with_url":       s.WithURL,
		"with_otp":       s.WithOTP,
		"with_notes":     s.WithNotes,
		"with_attach":    s.WithAttach,
		"weak_passwords": s.WeakCount,
		"top_tags":       s.TopTags,
	})
	fmt.Fprintln(c.out, string(data))
	return nil
}

func (s dbStats) renderText(c *ctx) error {
	fmt.Fprintf(c.out, "%s %s\n", color.Bold("Database:"), s.Database)
	if s.FileSize > 0 {
		fmt.Fprintf(c.out, "  %s %s (%s)\n", color.Faint("File:"), formatSize(s.FileSize), s.FileModTime)
	}
	fmt.Fprintln(c.out)
	fmt.Fprintf(c.out, "  %s %d\n", color.Cyan("Entries:"), s.EntryCount)
	fmt.Fprintf(c.out, "  %s %d\n", color.Cyan("Groups:"), s.GroupCount)
	fmt.Fprintf(c.out, "  %s %d\n", color.Cyan("Unique tags:"), s.TagCount)
	fmt.Fprintln(c.out)
	fmt.Fprintf(c.out, "  %s %d\n", color.Faint("With password:"), s.WithPassword)
	fmt.Fprintf(c.out, "  %s %d\n", color.Faint("With URL:"), s.WithURL)
	fmt.Fprintf(c.out, "  %s %d\n", color.Faint("With OTP:"), s.WithOTP)
	fmt.Fprintf(c.out, "  %s %d\n", color.Faint("With notes:"), s.WithNotes)
	fmt.Fprintf(c.out, "  %s %d\n", color.Faint("With attachments:"), s.WithAttach)
	if s.WeakCount > 0 {
		fmt.Fprintf(c.out, "  %s %d\n", color.Red("Weak passwords:"), s.WeakCount)
	}

	if len(s.TopTags) > 0 {
		fmt.Fprintf(c.out, "\n%s\n", color.Faint("Top tags:"))
		for _, p := range s.TopTags {
			fmt.Fprintf(c.out, "  %s %s\n", color.Bold(p.Tag), color.Faint(fmt.Sprintf("(%d)", p.Count)))
		}
	}
	return nil
}
