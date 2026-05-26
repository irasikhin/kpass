package cli

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/irasikhin/kpass/internal/runtimex"
)

const importPassHelp = `Import all entries from a pass(1) password store.

Walks the store directory, decrypts each *.gpg file via GPG (so any
required passphrase is prompted by your existing gpg-agent / pinentry),
parses each entry, and imports the result into the target KeePass
database.

Pass entry convention recognized:
  - line 1                 → password
  - "login:" / "user:" / "username:" / "email:" → username
  - "url:" / "website:"    → url
  - "otpauth://..."         → otp
  - other "key: value"     → custom field
  - remaining lines        → notes

Store directory defaults to $PASSWORD_STORE_DIR, then ~/.password-store.

Examples:
  kpass import-pass
  kpass import-pass ~/.password-store --on-conflict=rename
  kpass import-pass --gpg-binary=gpg2 -f`

// PassDecryptor decrypts a single .gpg file at the given absolute path.
// Tests inject a fake to avoid needing real GPG keys.
type PassDecryptor func(path string) ([]byte, error)

// PassDecryptorHook, when non-nil, replaces the default GPG-based decryptor.
// Used by tests via the same pattern as ClipboardWriter / OtpCoder.
var PassDecryptorHook PassDecryptor

// ImportPassCmd imports entries from a pass(1) password store.
type ImportPassCmd struct {
	Store      string `arg:"" optional:"" help:"Path to pass store (default $PASSWORD_STORE_DIR or ~/.password-store)."`
	GPGBinary  string `default:"gpg" help:"GPG binary to use for decryption." placeholder:"BIN"`
	OnConflict string `default:"skip" enum:"error,skip,overwrite,rename" help:"On path conflict: error, skip, overwrite, rename."`
	Force      bool   `short:"f" help:"Skip confirmation prompt."`
}

// Help returns detailed help for import-pass.
func (ImportPassCmd) Help() string { return importPassHelp }

func (cmd *ImportPassCmd) Run(c *ctx) error {
	store, err := resolvePassStore(cmd.Store)
	if err != nil {
		return &UserError{Msg: err.Error()}
	}
	if err := c.openDatabase(); err != nil {
		return err
	}

	dec := PassDecryptorHook
	if dec == nil {
		bin := cmd.GPGBinary
		if bin == "" {
			bin = "gpg"
		}
		dec = func(p string) ([]byte, error) { return decryptWithGPG(bin, p) }
	}

	imported, err := walkPassStore(store, dec)
	if err != nil {
		return &UserError{Msg: err.Error()}
	}
	return applyImport(c, imported, cmd.OnConflict, cmd.Force)
}

// resolvePassStore picks the store directory: explicit arg, then
// $PASSWORD_STORE_DIR, then ~/.password-store. Verifies it exists.
func resolvePassStore(explicit string) (string, error) {
	candidate := explicit
	if candidate == "" {
		candidate = os.Getenv("PASSWORD_STORE_DIR")
	}
	if candidate == "" {
		candidate = "~/.password-store"
	}
	candidate = runtimex.ExpandPath(candidate)
	info, err := os.Stat(candidate)
	if err != nil {
		return "", fmt.Errorf("pass store not found: %s", candidate)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("pass store is not a directory: %s", candidate)
	}
	return candidate, nil
}

// decryptWithGPG shells out to `gpg --decrypt --quiet path`, inheriting
// stderr so pinentry / gpg-agent prompts reach the user.
func decryptWithGPG(bin, path string) ([]byte, error) {
	cmd := exec.Command(bin, "--decrypt", "--quiet", path)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gpg decrypt failed for %s: %v", path, err)
	}
	return out, nil
}

// walkPassStore recursively scans root for *.gpg entries, decrypts them, and
// returns parsed importEntry values sorted by path. Hidden dirs (".git",
// ".extensions"), the ".gpg-id" file, and non-gpg files are ignored.
func walkPassStore(root string, dec PassDecryptor) ([]importEntry, error) {
	var entries []importEntry
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if p == root {
				return nil
			}
			if strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".gpg") {
			return nil
		}
		if strings.HasPrefix(name, ".") {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		relPath := filepath.ToSlash(strings.TrimSuffix(rel, ".gpg"))
		plaintext, err := dec(p)
		if err != nil {
			return fmt.Errorf("%s: %v", relPath, err)
		}
		entries = append(entries, parsePassEntry(relPath, plaintext))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, nil
}

// parsePassEntry turns the plaintext of one pass file into an importEntry.
// Convention: first line is the password; subsequent lines may carry
// well-known "key: value" pairs, an otpauth:// URI, or freeform notes.
func parsePassEntry(relPath string, content []byte) importEntry {
	text := strings.ReplaceAll(string(content), "\r\n", "\n")
	text = strings.TrimRight(text, "\n")
	lines := strings.Split(text, "\n")
	ie := importEntry{
		Path:   relPath,
		Title:  lastSegment(relPath),
		Custom: map[string]string{},
	}
	if len(lines) > 0 {
		ie.Password = lines[0]
	}
	var notes []string
	for _, line := range lines[1:] {
		if strings.HasPrefix(line, "otpauth://") {
			if ie.OTP == "" {
				ie.OTP = strings.TrimSpace(line)
			}
			continue
		}
		key, val, ok := splitKV(line)
		if ok {
			switch strings.ToLower(key) {
			case "login", "user", "username":
				if ie.Username == "" {
					ie.Username = val
					continue
				}
			case "url", "website":
				if ie.URL == "" {
					ie.URL = val
					continue
				}
			case "email":
				if ie.Username == "" {
					ie.Username = val
					continue
				}
				ie.Custom["email"] = val
				continue
			case "otp", "otpauth":
				if ie.OTP == "" {
					ie.OTP = val
					continue
				}
			case "password", "pass":
				// Some users put the password on a key line; ignore — line 1 wins.
				continue
			default:
				ie.Custom[strings.ToLower(key)] = val
				continue
			}
		}
		notes = append(notes, line)
	}
	// Trim blank padding from notes.
	for len(notes) > 0 && strings.TrimSpace(notes[0]) == "" {
		notes = notes[1:]
	}
	for len(notes) > 0 && strings.TrimSpace(notes[len(notes)-1]) == "" {
		notes = notes[:len(notes)-1]
	}
	ie.Notes = strings.Join(notes, "\n")
	if len(ie.Custom) == 0 {
		ie.Custom = nil
	}
	return ie
}

// splitKV recognizes "key: value" where key is a non-empty token that contains
// no whitespace. Returns key, value, true on success.
func splitKV(line string) (string, string, bool) {
	idx := strings.Index(line, ":")
	if idx <= 0 {
		return "", "", false
	}
	key := line[:idx]
	if strings.ContainsAny(key, " \t") {
		return "", "", false
	}
	val := strings.TrimSpace(line[idx+1:])
	return key, val, true
}

func lastSegment(path string) string {
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
}
