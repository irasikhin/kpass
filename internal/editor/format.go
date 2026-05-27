package editor

import (
	"fmt"
	"sort"
	"strings"
)

// EditableFields lists the keys the editor format accepts. Order matches the
// Python EDITABLE_ENTRY_FIELDS tuple.
var EditableFields = []string{"title", "username", "password", "url", "otp"}

// Fields holds the parsed editor output.
type Fields struct {
	Title    string
	Username string
	Password string
	URL      string
	OTP      string
	Notes    string
}

// Serialize mirrors Python serialize_entry_for_edit.
func Serialize(displayPath, title, username, password, url, otp, notes string) string {
	var b strings.Builder
	b.WriteString("# Edit the fields below. Use 'mv' to change the full path.\n")
	b.WriteString("# Path: " + displayPath + "\n")
	b.WriteString("title: " + title + "\n")
	b.WriteString("username: " + username + "\n")
	b.WriteString("password: " + password + "\n")
	b.WriteString("url: " + url + "\n")
	b.WriteString("otp: " + otp + "\n")
	b.WriteString("---\n")
	if notes != "" {
		b.WriteString(notes)
		if !strings.HasSuffix(notes, "\n") {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// Parse mirrors Python parse_edited_entry.
func Parse(text string) (Fields, error) {
	values := map[string]string{}
	for _, k := range EditableFields {
		values[k] = ""
	}
	var headerLines, notesLines []string
	inNotes := false
	for _, line := range strings.Split(text, "\n") {
		if inNotes {
			notesLines = append(notesLines, line)
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		if line == "---" {
			inNotes = true
			continue
		}
		if strings.TrimSpace(line) != "" {
			headerLines = append(headerLines, line)
		}
	}
	// strip the trailing empty line that splits produces from a trailing \n
	if len(notesLines) > 0 && notesLines[len(notesLines)-1] == "" {
		notesLines = notesLines[:len(notesLines)-1]
	}

	if !inNotes {
		return Fields{}, fmt.Errorf("edited entry is missing the '---' notes separator")
	}
	seen := map[string]bool{}
	for _, line := range headerLines {
		i := strings.IndexByte(line, ':')
		if i < 0 {
			return Fields{}, fmt.Errorf("invalid edit line: %s", line)
		}
		key := strings.ToLower(strings.TrimSpace(line[:i]))
		val := strings.TrimLeft(line[i+1:], " ")
		if _, ok := values[key]; !ok {
			return Fields{}, fmt.Errorf("unsupported edit field: %s", key)
		}
		if seen[key] {
			return Fields{}, fmt.Errorf("duplicate edit field: %s", key)
		}
		values[key] = val
		seen[key] = true
	}
	var missing []string
	for _, k := range EditableFields {
		if !seen[k] {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return Fields{}, fmt.Errorf("edited entry is missing field(s): %s", strings.Join(missing, ", "))
	}
	return Fields{
		Title:    values["title"],
		Username: values["username"],
		Password: values["password"],
		URL:      values["url"],
		OTP:      values["otp"],
		Notes:    strings.Join(notesLines, "\n"),
	}, nil
}
