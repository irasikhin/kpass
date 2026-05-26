package editor

import (
	"strings"
	"testing"
)

func TestSerialize_Basic(t *testing.T) {
	out := Serialize("work/email", "email", "alice", "secret", "https://example.com", "", "personal notes")
	if !strings.Contains(out, "# Path: work/email") {
		t.Error("missing path")
	}
	if !strings.Contains(out, "title: email") {
		t.Error("missing title")
	}
	if !strings.Contains(out, "username: alice") {
		t.Error("missing username")
	}
	if !strings.Contains(out, "password: secret") {
		t.Error("missing password")
	}
	if !strings.Contains(out, "url: https://example.com") {
		t.Error("missing url")
	}
	if !strings.Contains(out, "otp: ") {
		t.Error("missing otp")
	}
	if !strings.Contains(out, "---\npersonal notes") {
		t.Error("missing notes separator")
	}
}

func TestSerialize_NoNotes(t *testing.T) {
	out := Serialize("x", "t", "u", "p", "", "", "")
	if !strings.Contains(out, "---\n") {
		t.Error("missing separator")
	}
	parts := strings.Split(out, "---\n")
	if len(parts) > 1 && parts[1] != "" {
		t.Errorf("unexpected content after separator: %q", parts[1])
	}
}

func TestParse_Roundtrip(t *testing.T) {
	original := Serialize("a/b", "b", "u", "p", "https://x.com", "otpauth://x", "notes here\nline2")
	parsed, err := Parse(original)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if parsed.Title != "b" {
		t.Errorf("title = %q", parsed.Title)
	}
	if parsed.Username != "u" {
		t.Errorf("username = %q", parsed.Username)
	}
	if parsed.Password != "p" {
		t.Errorf("password = %q", parsed.Password)
	}
	if parsed.URL != "https://x.com" {
		t.Errorf("url = %q", parsed.URL)
	}
	if parsed.OTP != "otpauth://x" {
		t.Errorf("otp = %q", parsed.OTP)
	}
	if parsed.Notes != "notes here\nline2" {
		t.Errorf("notes = %q", parsed.Notes)
	}
}

func TestParse_MissingSeparator(t *testing.T) {
	_, err := Parse("title: foo\nusername: bar\npassword: baz\nurl: \notp: ")
	if err == nil {
		t.Error("expected error for missing --- separator")
	}
}

func TestParse_MissingField(t *testing.T) {
	input := "title: foo\nusername: bar\npassword: baz\nurl: \n---\n"
	_, err := Parse(input)
	if err == nil {
		t.Error("expected error for missing otp field")
	}
	if !strings.Contains(err.Error(), "missing field(s)") {
		t.Errorf("error = %v, want missing field", err)
	}
}

func TestParse_InvalidLine(t *testing.T) {
	input := "title: foo\nusername: bar\npassword: baz\nurl: \notp: \nbad line\n---\n"
	_, err := Parse(input)
	if err == nil {
		t.Error("expected error for invalid line")
	}
}

func TestParse_UnsupportedField(t *testing.T) {
	input := "title: foo\nusername: bar\npassword: baz\nurl: \notp: \nbad: value\n---\n"
	_, err := Parse(input)
	if err == nil {
		t.Error("expected error for unsupported field")
	}
}

func TestParse_DuplicateField(t *testing.T) {
	input := "title: foo\nusername: bar\npassword: baz\nurl: \notp: \ntitle: dup\n---\n"
	_, err := Parse(input)
	if err == nil {
		t.Error("expected error for duplicate field")
	}
}

func TestParse_SkipsComments(t *testing.T) {
	input := "# header comment\n# another\ntitle: t\nusername: u\npassword: p\nurl: \notp: \n# inline comment\n---\nnotes"
	parsed, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if parsed.Title != "t" {
		t.Errorf("title = %q", parsed.Title)
	}
	if parsed.Notes != "notes" {
		t.Errorf("notes = %q", parsed.Notes)
	}
}

func TestParse_EmptyFields(t *testing.T) {
	input := "title: \nusername: \npassword: \nurl: \notp: \n---\n"
	parsed, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if parsed.Title != "" {
		t.Errorf("title = %q, want empty", parsed.Title)
	}
}

func TestParse_CaseInsensitiveKeys(t *testing.T) {
	input := "TITLE: t\nUserName: u\nPassword: p\nURL: \nOTP: \n---\n"
	parsed, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if parsed.Title != "t" || parsed.Username != "u" || parsed.Password != "p" {
		t.Errorf("case insensitive parse failed")
	}
}
