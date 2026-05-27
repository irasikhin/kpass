package db

import (
	"encoding/json"
	"strings"
	"testing"
)

func findEntry(t *testing.T, d *DB, path string) *Entry {
	t.Helper()
	for _, e := range d.SortedEntries() {
		if e.DisplayPath() == path {
			return e
		}
	}
	t.Fatalf("entry not found: %s", path)
	return nil
}

func TestEntry_ToJSON(t *testing.T) {
	d := seedDB(t)
	e := findEntry(t, d, "work/email")
	j := e.ToJSON()
	if j.Path != "work/email" || j.Title != "email" || j.Username != "alice@work" {
		t.Errorf("basic fields: %+v", j)
	}
	if j.Password != "work-pass" {
		t.Errorf("password = %q", j.Password)
	}
	if !j.HasOTP || !strings.Contains(j.OTP, "otpauth://") {
		t.Errorf("OTP fields: %+v", j)
	}
	if len(j.Attachments) != 1 || j.Attachments[0] != "doc.txt" {
		t.Errorf("attachments = %v", j.Attachments)
	}
	if j.CustomFields["Pin"] != "1234" {
		t.Errorf("custom field Pin = %v", j.CustomFields)
	}

	// JSON should serialize without error.
	if _, err := json.Marshal(j); err != nil {
		t.Errorf("Marshal: %v", err)
	}
}

func TestEntry_ToJSON_NoCustomFields(t *testing.T) {
	d := seedDB(t)
	e := findEntry(t, d, "work/chat")
	j := e.ToJSON()
	if j.CustomFields != nil {
		t.Errorf("CustomFields should be nil when empty, got %v", j.CustomFields)
	}
}

func TestEntry_ToInfoJSON(t *testing.T) {
	d := seedDB(t)
	e := findEntry(t, d, "work/email")
	info := e.ToInfoJSON()
	if !info.HasPassword || !info.HasURL || !info.HasOTP || !info.HasNotes || info.AttachCount != 1 {
		t.Errorf("ToInfoJSON = %+v", info)
	}
}

func TestEntry_GetAttribute_Standard(t *testing.T) {
	d := seedDB(t)
	e := findEntry(t, d, "work/email")
	cases := []struct {
		attr, want string
	}{
		{"title", "email"},
		{"name", "email"},
		{"username", "alice@work"},
		{"user", "alice@work"},
		{"login", "alice@work"},
		{"password", "work-pass"},
		{"pass", "work-pass"},
		{"url", "https://mail.example.com"},
		{"notes", "notes-here"},
		{"path", "work/email"},
	}
	for _, tc := range cases {
		got, err := e.GetAttribute(tc.attr)
		if err != nil {
			t.Errorf("GetAttribute(%q): %v", tc.attr, err)
			continue
		}
		if got != tc.want {
			t.Errorf("GetAttribute(%q) = %q, want %q", tc.attr, got, tc.want)
		}
	}
}

func TestEntry_GetAttribute_OTP(t *testing.T) {
	d := seedDB(t)
	e := findEntry(t, d, "work/email")
	for _, a := range []string{"otp", "totp", "code"} {
		got, err := e.GetAttribute(a)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(got, "otpauth://") {
			t.Errorf("GetAttribute(%q) = %q", a, got)
		}
	}
}

func TestEntry_GetAttribute_CustomField(t *testing.T) {
	d := seedDB(t)
	e := findEntry(t, d, "work/email")
	got, err := e.GetAttribute("Pin")
	if err != nil {
		t.Fatal(err)
	}
	if got != "1234" {
		t.Errorf("Pin = %q", got)
	}
}

func TestEntry_GetAttribute_NotFound(t *testing.T) {
	d := seedDB(t)
	e := findEntry(t, d, "work/email")
	if _, err := e.GetAttribute("nonexistent-field"); err == nil {
		t.Error("expected not-found error")
	}
}

func TestEntry_OtpURI(t *testing.T) {
	d := seedDB(t)
	emailEntry := findEntry(t, d, "work/email")
	if uri := emailEntry.OtpURI(); !strings.HasPrefix(uri, "otpauth://") {
		t.Errorf("otp uri = %q", uri)
	}
	chat := findEntry(t, d, "work/chat")
	if uri := chat.OtpURI(); uri != "" {
		t.Errorf("chat should have no otp, got %q", uri)
	}
}

func TestEntry_SetField_KnownAndCustom(t *testing.T) {
	d := seedDB(t)
	e := findEntry(t, d, "work/chat")

	e.SetField("title", "renamed")
	if e.Title() != "renamed" {
		t.Errorf("title not updated in path: %q", e.Title())
	}
	if e.Raw().GetTitle() != "renamed" {
		t.Errorf("title not updated in raw: %q", e.Raw().GetTitle())
	}

	e.SetField("username", "u")
	e.SetField("password", "p")
	e.SetField("url", "u-r-l")
	e.SetField("notes", "n")
	e.SetField("otp", "otpauth://totp/X?secret=ABC")
	if e.Raw().GetPassword() != "p" || e.Raw().GetContent("URL") != "u-r-l" {
		t.Errorf("SetField didn't update standard fields")
	}

	e.SetField("CustomKey", "custom-value")
	got, err := e.GetAttribute("CustomKey")
	if err != nil {
		t.Fatal(err)
	}
	if got != "custom-value" {
		t.Errorf("custom = %q", got)
	}
}

func TestEntry_SetField_TitleEmptyPath(t *testing.T) {
	e := &Entry{e: emptyRawEntry()}
	e.SetField("title", "first")
	if len(e.Path) != 1 || e.Path[0] != "first" {
		t.Errorf("Path after SetField title from empty = %v", e.Path)
	}
}

func TestEntry_CustomFields_FiltersStandardKeys(t *testing.T) {
	d := seedDB(t)
	e := findEntry(t, d, "work/email")
	cf := e.CustomFields()
	for _, k := range []string{"Title", "UserName", "Password", "URL", "Notes", "otp"} {
		if _, ok := cf[k]; ok {
			t.Errorf("standard key %q should not appear in CustomFields", k)
		}
	}
	if cf["Pin"] != "1234" {
		t.Errorf("Pin missing from CustomFields: %v", cf)
	}
}

func TestEntry_CustomFields_ProtectedRedacted(t *testing.T) {
	d := seedDB(t)
	e := findEntry(t, d, "work/chat")
	e.SetField("PrivateKey", "secret")
	for i, v := range e.Raw().Values {
		if v.Key == "PrivateKey" {
			vv := e.Raw().Values[i].Value
			vv.Protected = boolWrap(true)
			e.Raw().Values[i].Value = vv
		}
	}
	cf := e.CustomFields()
	if cf["PrivateKey"] != "<protected>" {
		t.Errorf("protected custom should be redacted, got %q", cf["PrivateKey"])
	}
}

func TestEntry_FormatFull(t *testing.T) {
	d := seedDB(t)
	e := findEntry(t, d, "work/email")
	out := e.FormatFull()
	for _, want := range []string{"Path:", "Title:", "UserName:", "URL:", "Notes:", "TOTP:", "yes", "Tags:", "personal", "hot", "Pin:", "1234"} {
		if !strings.Contains(out, want) {
			t.Errorf("FormatFull missing %q in:\n%s", want, out)
		}
	}
}

func TestEntry_FormatFull_NoTotpNoTags(t *testing.T) {
	d := seedDB(t)
	e := findEntry(t, d, "work/chat")
	out := e.FormatFull()
	if !strings.Contains(out, "TOTP:") || !strings.Contains(out, "no") {
		t.Errorf("expected 'TOTP: no', got:\n%s", out)
	}
}

func TestEntry_FormatFullWithPassword_Masked(t *testing.T) {
	d := seedDB(t)
	e := findEntry(t, d, "work/email")
	out := e.FormatFullWithPassword(true)
	if strings.Contains(out, "work-pass") {
		t.Errorf("masked output leaks password:\n%s", out)
	}
	if !strings.Contains(out, "wo") || !strings.Contains(out, "ss") {
		t.Errorf("mask should preserve first/last 2 chars:\n%s", out)
	}
}

func TestEntry_FormatFullWithPassword_MaskShort(t *testing.T) {
	d := seedDB(t)
	e := findEntry(t, d, "work/chat")
	e.SetField("password", "abc") // ≤4 chars triggers the "****" mask branch.
	out := e.FormatFullWithPassword(true)
	if !strings.Contains(out, "****") || strings.Contains(out, "abc") {
		t.Errorf("short-password mask: %s", out)
	}
}

func TestEntry_FormatFullWithPassword_Unmasked(t *testing.T) {
	d := seedDB(t)
	e := findEntry(t, d, "work/email")
	out := e.FormatFullWithPassword(false)
	if !strings.Contains(out, "work-pass") {
		t.Errorf("unmasked should show password:\n%s", out)
	}
	if !strings.Contains(out, "Strength:") {
		t.Errorf("expected strength line:\n%s", out)
	}
}

func TestEntry_SearchableField(t *testing.T) {
	d := seedDB(t)
	e := findEntry(t, d, "work/email")
	cases := []struct{ field, want string }{
		{"path", "work/email"},
		{"title", "email"},
		{"username", "alice@work"},
		{"password", "work-pass"},
		{"url", "https://mail.example.com"},
		{"notes", "notes-here"},
		{"Pin", "1234"},
	}
	for _, tc := range cases {
		if got := e.SearchableField(tc.field); got != tc.want {
			t.Errorf("SearchableField(%q) = %q, want %q", tc.field, got, tc.want)
		}
	}
	if got := e.SearchableField("otp"); got == "" {
		t.Errorf("otp searchable should be the URI")
	}
}
