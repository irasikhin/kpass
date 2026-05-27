package db

import (
	"fmt"
	"strings"

	"github.com/irasikhin/kpass/internal/color"
	"github.com/irasikhin/kpass/internal/pwgen"
)

// EntryJSON is the JSON-serialisable representation of an entry.
type EntryJSON struct {
	Path         string            `json:"path"`
	Title        string            `json:"title"`
	Username     string            `json:"username"`
	Password     string            `json:"password"`
	URL          string            `json:"url"`
	Notes        string            `json:"notes"`
	OTP          string            `json:"otp"`
	HasOTP       bool              `json:"has_otp"`
	Attachments  []string          `json:"attachments"`
	CustomFields map[string]string `json:"custom_fields,omitempty"`
}

// ToJSON returns a JSON-ready struct for this entry.
func (e *Entry) ToJSON() EntryJSON {
	otp := e.OtpURI()
	j := EntryJSON{
		Path:        e.DisplayPath(),
		Title:       e.Raw().GetTitle(),
		Username:    e.Raw().GetContent("UserName"),
		Password:    e.Raw().GetPassword(),
		URL:         e.Raw().GetContent("URL"),
		Notes:       e.Raw().GetContent("Notes"),
		OTP:         otp,
		HasOTP:      otp != "",
		Attachments: e.AttachmentList(),
	}
	if cf := e.CustomFields(); len(cf) > 0 {
		j.CustomFields = cf
	}
	return j
}

// EntryInfoJSON is a lightweight version for ls/search output.
type EntryInfoJSON struct {
	Path        string `json:"path"`
	HasPassword bool   `json:"has_password"`
	HasURL      bool   `json:"has_url"`
	HasOTP      bool   `json:"has_otp"`
	HasNotes    bool   `json:"has_notes"`
	AttachCount int    `json:"attach_count"`
}

// ToInfoJSON returns a lightweight JSON struct for ls/search.
func (e *Entry) ToInfoJSON() EntryInfoJSON {
	return EntryInfoJSON{
		Path:        e.DisplayPath(),
		HasPassword: e.Raw().GetPassword() != "",
		HasURL:      e.Raw().GetContent("URL") != "",
		HasOTP:      e.OtpURI() != "",
		HasNotes:    e.Raw().GetContent("Notes") != "",
		AttachCount: len(e.Raw().Binaries),
	}
}

// GetAttribute mirrors Python get_entry_attribute. otp_uri callers will
// receive the raw URI; cli converts it to a code via the otp package.
// Custom fields are looked up by their KeePass key name.
func (e *Entry) GetAttribute(attr string) (string, error) {
	a := strings.ToLower(attr)
	switch a {
	case "title", "name":
		return e.e.GetTitle(), nil
	case "username", "user", "login":
		return e.e.GetContent("UserName"), nil
	case "password", "pass":
		return e.e.GetPassword(), nil
	case "url":
		return e.e.GetContent("URL"), nil
	case "notes":
		return e.e.GetContent("Notes"), nil
	case "path":
		return e.DisplayPath(), nil
	case "otp", "totp", "code":
		return e.OtpURI(), nil
	}
	// Custom field — look up by the original key name or case-insensitive.
	val := e.e.GetContent(attr)
	if val != "" {
		return val, nil
	}
	// Try exact match on the Values slice for case-sensitive custom keys.
	for _, v := range e.e.Values {
		if v.Key == attr {
			if v.Value.Protected.Bool {
				return "", fmt.Errorf("cannot read protected custom field: %s", attr)
			}
			return v.Value.Content, nil
		}
	}
	return "", fmt.Errorf("field not found: %s", attr)
}

// OtpURI returns the stored otpauth:// URI (the "otp" custom field).
func (e *Entry) OtpURI() string {
	return e.e.GetContent("otp")
}

// SetField sets a known field; mirrors Python attribute assignments. The
// `protected` argument forces the protected flag for new password values.
// Custom fields (keys not among the predefined set) are stored as-is.
func (e *Entry) SetField(name, value string) {
	switch strings.ToLower(name) {
	case "title":
		entrySet(e.e, "Title", value)
		if len(e.Path) > 0 {
			e.Path[len(e.Path)-1] = value
		} else {
			e.Path = []string{value}
		}
		return
	case "username":
		entrySet(e.e, "UserName", value)
		return
	case "password":
		entrySet(e.e, "Password", value)
		return
	case "url":
		entrySet(e.e, "URL", value)
		return
	case "notes":
		entrySet(e.e, "Notes", value)
		return
	case "otp":
		entrySet(e.e, "otp", value)
		return
	}
	// Custom field — store with original key name.
	entrySet(e.e, name, value)
}

// CustomFields returns all non-standard key-value pairs on the entry.
func (e *Entry) CustomFields() map[string]string {
	known := map[string]bool{
		"Title": true, "UserName": true, "Password": true,
		"URL": true, "Notes": true, "otp": true,
	}
	out := make(map[string]string)
	for _, v := range e.e.Values {
		if !known[v.Key] && v.Key != "" {
			if v.Value.Protected.Bool {
				out[v.Key] = "<protected>"
			} else {
				out[v.Key] = v.Value.Content
			}
		}
	}
	return out
}

// FormatFull mirrors Python format_entry output (used by cmd_get without --field).
func (e *Entry) FormatFull() string {
	lines := []string{
		color.Cyan("Path:") + " " + e.DisplayPath(),
		color.Cyan("Title:") + " " + e.e.GetTitle(),
		color.Cyan("UserName:") + " " + e.e.GetContent("UserName"),
		color.Cyan("URL:") + " " + e.e.GetContent("URL"),
		color.Cyan("Notes:") + " " + e.e.GetContent("Notes"),
	}
	otpState := "no"
	if e.OtpURI() != "" {
		otpState = "yes"
	}
	lines = append(lines, color.Cyan("TOTP:")+" "+otpState)
	if tags := e.Tags(); len(tags) > 0 {
		lines = append(lines, color.Cyan("Tags:")+" "+strings.Join(tags, ", "))
	}
	for k, v := range e.CustomFields() {
		lines = append(lines, color.Cyan(k+":")+" "+v)
	}
	return strings.Join(lines, "\n")
}

// FormatFullWithPassword is like FormatFull but includes the password line.
// If mask is true, the password is partially hidden.
func (e *Entry) FormatFullWithPassword(mask bool) string {
	pw := e.e.GetPassword()
	var strengthLine string
	if !mask && pw != "" {
		s := pwgen.Assess(pw)
		strengthLine = fmt.Sprintf("%s %s %s",
			color.Faint("Strength:"),
			color.Faint(s.Bar),
			color.Faint(fmt.Sprintf("%s (%.0f bits)", s.Label, s.Bits)))
	}
	if mask && len(pw) > 4 {
		pw = pw[:2] + strings.Repeat("•", len(pw)-4) + pw[len(pw)-2:]
	} else if mask {
		pw = "****"
	}
	lines := []string{
		color.Cyan("Path:") + " " + e.DisplayPath(),
		color.Cyan("Title:") + " " + e.e.GetTitle(),
		color.Cyan("UserName:") + " " + e.e.GetContent("UserName"),
		color.Cyan("Password:") + " " + pw,
	}
	if strengthLine != "" {
		lines = append(lines, strengthLine)
	}
	lines = append(lines,
		color.Cyan("URL:")+" "+e.e.GetContent("URL"),
		color.Cyan("Notes:")+" "+e.e.GetContent("Notes"),
		color.Cyan("TOTP:")+" "+func() string {
			if e.OtpURI() != "" {
				return "yes"
			}
			return "no"
		}(),
	)
	if tags := e.Tags(); len(tags) > 0 {
		lines = append(lines, color.Cyan("Tags:")+" "+strings.Join(tags, ", "))
	}
	for k, v := range e.CustomFields() {
		lines = append(lines, color.Cyan(k+":")+" "+v)
	}
	return strings.Join(lines, "\n")
}

// SearchableField returns the haystack used by `search --field`.
func (e *Entry) SearchableField(field string) string {
	switch field {
	case "path":
		return e.DisplayPath()
	case "title":
		return e.e.GetTitle()
	case "username":
		return e.e.GetContent("UserName")
	case "password":
		return e.e.GetPassword()
	case "url":
		return e.e.GetContent("URL")
	case "notes":
		return e.e.GetContent("Notes")
	case "otp":
		return e.OtpURI()
	}
	// Custom field lookup.
	return e.e.GetContent(field)
}
