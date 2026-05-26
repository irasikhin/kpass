package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/tobischo/gokeepasslib/v3"
	w "github.com/tobischo/gokeepasslib/v3/wrappers"

	"github.com/irasikhin/kpass/internal/color"
)

// HistoryCmd shows or restores entry history (KeePass built-in versioning).
type HistoryCmd struct {
	Entry   string `arg:"" help:"Entry path or partial path."`
	Diff    bool   `help:"Show fields that differ from the current version."`
	Restore int    `default:"-1" help:"Restore a history version by index (0 = most recent)."`
}

// Help returns extended help for history.
func (HistoryCmd) Help() string {
	return `View or restore previous versions of an entry using KeePass built-in
history.

Without flags, lists all history versions with timestamps (most recent
first). The current version is marked with *.

  --diff     Show a per-field diff between the current version and the
             most recent history entry.
  --restore N  Replace the current entry with history version #N.`
}

func (cmd *HistoryCmd) Run(c *ctx) error {
	if err := c.openDatabase(); err != nil {
		return err
	}
	entry, err := c.db.ResolveEntry(cmd.Entry)
	if err != nil {
		return err
	}

	raw := entry.Raw()
	histories := raw.Histories

	if len(histories) == 0 {
		fmt.Fprintln(c.out, color.Faint("No history entries."))
		return nil
	}

	// Flatten: each History may contain multiple Entries.
	type historyEntry struct {
		idx   int
		entry *gokeepasslib.Entry
		time  time.Time
	}
	var items []historyEntry
	idx := 0
	for _, h := range histories {
		for i := range h.Entries {
			e := &h.Entries[i]
			t := time.Now()
			if e.Times.LastModificationTime != nil {
				t = e.Times.LastModificationTime.Time
			}
			items = append(items, historyEntry{idx: idx, entry: e, time: t})
			idx++
		}
	}

	if cmd.Restore >= 0 {
		if cmd.Restore >= len(items) {
			return &UserError{Msg: fmt.Sprintf("History index %d out of range (0-%d).", cmd.Restore, len(items)-1)}
		}
		hist := items[cmd.Restore]
		restoreEntry(raw, hist.entry)
		if err := c.db.Save(); err != nil {
			return &UserError{Msg: err.Error()}
		}
		fmt.Fprintf(c.out, "%s %s\n", color.Green("Restored version"), color.Bold(fmt.Sprintf("#%d", cmd.Restore)))
		fmt.Fprintf(c.out, "  %s %s\n", color.Faint("from"), hist.time.Format("2006-01-02 15:04:05"))
		return nil
	}

	if cmd.Diff {
		if len(items) == 0 {
			return nil
		}
		latest := items[len(items)-1]
		changes := diffEntries(raw, latest.entry)
		if len(changes) == 0 {
			fmt.Fprintln(c.out, color.Faint("No differences from current version."))
		} else {
			fmt.Fprintf(c.out, "%s %s\n", color.Bold("Diff vs"), color.Faint(fmt.Sprintf("version #%d (%s)", latest.idx, latest.time.Format("2006-01-02 15:04"))))
			for _, ch := range changes {
				fmt.Fprintf(c.out, "  %s %s %s %s\n",
					color.Cyan(ch.field+":"),
					color.Red(ch.old),
					color.Faint("→"),
					color.Green(ch.new))
			}
		}
		return nil
	}

	// Default: list history.
	fmt.Fprintln(c.out, color.Bold(entry.DisplayPath()))
	for i := len(items) - 1; i >= 0; i-- {
		e := items[i]
		marker := " "
		if i == len(items)-1 {
			marker = color.Green("*")
		}
		title := e.entry.GetTitle()
		fmt.Fprintf(c.out, "%s %s %s %s\n",
			marker,
			color.Faint(fmt.Sprintf("#%d", e.idx)),
			e.time.Format("2006-01-02 15:04:05"),
			color.Faint(title))
	}
	fmt.Fprintf(c.out, "\n%s %s\n",
		color.Faint("Total:"),
		color.Bold(fmt.Sprintf("%d version(s)", len(items))))
	return nil
}

// restoreEntry copies fields from src into dst (current entry).
func restoreEntry(dst, src *gokeepasslib.Entry) {
	// Copy standard values.
	copyValue(dst, src, "Title")
	copyValue(dst, src, "UserName")
	copyValue(dst, src, "Password")
	copyValue(dst, src, "URL")
	copyValue(dst, src, "Notes")
	copyValue(dst, src, "otp")
	// Copy all other values.
	for _, sv := range src.Values {
		known := map[string]bool{
			"Title": true, "UserName": true, "Password": true,
			"URL": true, "Notes": true, "otp": true,
		}
		if !known[sv.Key] {
			found := false
			for j, dv := range dst.Values {
				if dv.Key == sv.Key {
					dst.Values[j].Value.Content = sv.Value.Content
					found = true
					break
				}
			}
			if !found {
				dst.Values = append(dst.Values, sv)
			}
		}
	}
}

func copyValue(dst, src *gokeepasslib.Entry, key string) {
	sv := src.GetContent(key)
	for i, dv := range dst.Values {
		if dv.Key == key {
			dst.Values[i].Value.Content = sv
			return
		}
	}
	// Key not in dst — add it.
	v := gokeepasslib.ValueData{Key: key, Value: gokeepasslib.V{Content: sv}}
	if key == "Password" || key == "otp" {
		v.Value.Protected = w.NewBoolWrapper(true)
	}
	dst.Values = append(dst.Values, v)
}

type fieldChange struct {
	field string
	old   string
	new   string
}

func diffEntries(current, old *gokeepasslib.Entry) []fieldChange {
	keys := []string{"Title", "UserName", "Password", "URL", "Notes", "otp"}
	var changes []fieldChange
	for _, k := range keys {
		cur := current.GetContent(k)
		prev := old.GetContent(k)
		if cur != prev {
			label := strings.ToLower(k)
			if label == "username" {
				label = "username"
			}
			changes = append(changes, fieldChange{
				field: label,
				old:   truncateDiff(prev),
				new:   truncateDiff(cur),
			})
		}
	}
	return changes
}

func truncateDiff(s string) string {
	if len(s) <= 40 {
		if s == "" {
			return "(empty)"
		}
		return s
	}
	return s[:37] + "..."
}
