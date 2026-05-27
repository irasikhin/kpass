package db

import (
	"fmt"
	"strconv"

	"github.com/tobischo/gokeepasslib/v3"

	"github.com/irasikhin/kpass/internal/config"
	"github.com/irasikhin/kpass/internal/runtimex"
)

// MergeStats reports the outcome of Merge.
type MergeStats struct {
	Imported    int
	Overwritten int
	Skipped     int
	Renamed     int
}

// ConflictStrategy is one of "error", "skip", "overwrite", "rename".
type ConflictStrategy string

const (
	ConflictError     ConflictStrategy = "error"
	ConflictSkip      ConflictStrategy = "skip"
	ConflictOverwrite ConflictStrategy = "overwrite"
	ConflictRename    ConflictStrategy = "rename"
)

// MergeOpts configures Merge.
type MergeOpts struct {
	OnConflict   ConflictStrategy
	RenameSuffix string
}

// Merge imports every entry in `source` into `d`. Conflicts (same display
// path) are handled per `opts.OnConflict`. Mirrors Python cmd_merge.
func (d *DB) Merge(source *DB, opts MergeOpts) (MergeStats, error) {
	var stats MergeStats
	for _, src := range source.SortedEntries() {
		entryPath := src.DisplayPath()
		var existing *Entry
		if exact := d.FindEntryByExactPath(entryPath); exact != nil {
			existing = exact
		}

		if existing != nil {
			switch opts.OnConflict {
			case ConflictError:
				return stats, fmt.Errorf("Merge conflict on entry: %s", entryPath)
			case ConflictSkip:
				stats.Skipped++
				continue
			case ConflictOverwrite:
				d.replaceEntryData(existing, src)
				stats.Overwritten++
				continue
			case ConflictRename:
				parts := runtimex.SplitPath(entryPath)
				groupPath := runtimex.JoinPath(parts[:len(parts)-1])
				title := parts[len(parts)-1]
				newPath := d.UniqueEntryPath(groupPath, title, opts.RenameSuffix)
				d.importEntry(src, newPath)
				stats.Renamed++
				continue
			}
		}
		d.importEntry(src, entryPath)
		stats.Imported++
	}
	return stats, nil
}

// UniqueEntryPath mirrors Python unique_entry_path.
func (d *DB) UniqueEntryPath(groupPath, title, suffix string) string {
	base := title
	if groupPath != "" {
		base = groupPath + "/" + title
	}
	if !d.entryPathExists(base) {
		return base
	}
	known := map[string]bool{}
	for _, e := range d.SortedEntries() {
		known[e.DisplayPath()] = true
	}
	var candidate string
	index := 2
	if suffix != "" {
		candidate = title + " (" + suffix + ")"
	} else {
		candidate = title + " (2)"
		index = 3
	}
	for {
		path := candidate
		if groupPath != "" {
			path = groupPath + "/" + candidate
		}
		if !known[path] {
			return path
		}
		if suffix != "" {
			candidate = title + " (" + suffix + " " + strconv.Itoa(index) + ")"
		} else {
			candidate = title + " (" + strconv.Itoa(index) + ")"
		}
		index++
	}
}

func (d *DB) entryPathExists(path string) bool {
	for _, e := range d.SortedEntries() {
		if e.DisplayPath() == path {
			return true
		}
	}
	return false
}

// importEntry mirrors Python import_entry: deep-clone the source into the
// target tree at destinationPath.
func (d *DB) importEntry(src *Entry, destinationPath string) *Entry {
	parts := runtimex.SplitPath(destinationPath)
	parent := d.EnsureGroup(runtimex.JoinPath(parts[:len(parts)-1]))
	title := parts[len(parts)-1]
	cloned := cloneRawEntry(src.e, title)
	parent.g.Entries = append(parent.g.Entries, cloned)
	ref := &parent.g.Entries[len(parent.g.Entries)-1]
	// Re-import binaries into the target DB (they live in a different db's
	// binary table). Walk the source entry's binaries and rebuild references.
	for i, br := range src.e.Binaries {
		bin := src.d.Raw.FindBinary(br.Value.ID)
		if bin == nil {
			continue
		}
		data, err := bin.GetContentBytes()
		if err != nil {
			continue
		}
		newBin := d.Raw.AddBinary(data)
		ref.Binaries[i] = newBin.CreateReference(br.Name)
	}
	return &Entry{
		d:      d,
		e:      ref,
		parent: parent.g,
		Path:   append(append([]string(nil), parent.Path...), title),
	}
}

// replaceEntryData mirrors Python replace_entry_data: copy core fields and
// custom values, replace tags, custom properties, and attachments.
func (d *DB) replaceEntryData(target, source *Entry) {
	entrySet(target.e, "UserName", source.e.GetContent("UserName"), false)
	entrySet(target.e, "Password", source.e.GetPassword(), true)
	entrySet(target.e, "URL", source.e.GetContent("URL"), false)
	entrySet(target.e, "Notes", source.e.GetContent("Notes"), false)
	entrySet(target.e, "otp", source.e.GetContent("otp"), true)
	target.e.Tags = source.e.Tags

	// Replace custom properties (any key not in the standard set).
	standard := map[string]bool{
		"Title": true, "UserName": true, "Password": true,
		"URL": true, "Notes": true, "otp": true,
	}
	cleaned := target.e.Values[:0]
	for _, v := range target.e.Values {
		if standard[v.Key] {
			cleaned = append(cleaned, v)
		}
	}
	target.e.Values = cleaned
	for _, v := range source.e.Values {
		if standard[v.Key] {
			continue
		}
		target.e.Values = append(target.e.Values, v)
	}

	// Replace attachments.
	target.e.Binaries = nil
	for _, br := range source.e.Binaries {
		bin := source.d.Raw.FindBinary(br.Value.ID)
		if bin == nil {
			continue
		}
		data, err := bin.GetContentBytes()
		if err != nil {
			continue
		}
		newBin := d.Raw.AddBinary(data)
		target.e.Binaries = append(target.e.Binaries, newBin.CreateReference(br.Name))
	}
}

// cloneRawEntry returns a fresh Entry copying all values + non-binary fields
// from src; the title is replaced with `title`. Binaries are reset on the
// clone — caller is expected to repopulate them via AddBinary on the target.
func cloneRawEntry(src *gokeepasslib.Entry, title string) gokeepasslib.Entry {
	out := src.Clone()
	// Reset Histories on import — pykeepass creates new entries without
	// inherited history.
	out.Histories = nil
	entrySet(&out, "Title", title, false)
	// Preserve Binaries slice length so caller can index into it.
	out.Binaries = append(out.Binaries[:0:0], src.Binaries...)
	return out
}

// OpenSimple opens a kdbx file with the given credentials, no caching, no
// password chain. Used for the merge source database.
func OpenSimple(dbPath, passwordFile, keyFile, inlinePassword string) (*DB, error) {
	expanded := runtimex.ExpandPath(dbPath)
	password := inlinePassword
	if password == "" && passwordFile != "" {
		data, err := config.ReadPasswordFile(passwordFile)
		if err != nil {
			return nil, err
		}
		password = data
	}
	if password == "" && passwordFile == "" && keyFile == "" {
		return nil, fmt.Errorf("OpenSimple requires at least a password or key file.")
	}
	creds, err := buildCreds(password, keyFile)
	if err != nil {
		return nil, err
	}
	f, err := openFile(expanded)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	raw := gokeepasslib.NewDatabase()
	raw.Credentials = creds
	if err := gokeepasslib.NewDecoder(f).Decode(raw); err != nil {
		return nil, fmt.Errorf("failed to open KeePass database: %v", err)
	}
	if err := raw.UnlockProtectedEntries(); err != nil {
		return nil, err
	}
	return &DB{Path: expanded, KeyFile: keyFile, Raw: raw}, nil
}
