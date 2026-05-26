package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/irasikhin/kpass/internal/color"
)

// AuditCmd checks the database for security issues.
type AuditCmd struct {
	JSON bool `help:"Output as JSON."`
}

// auditIssue represents a single finding.
type auditIssue struct {
	Entry  string `json:"entry"`
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
	Field  string `json:"field,omitempty"`
}

func (cmd *AuditCmd) Run(c *ctx) error {
	if err := c.openDatabase(); err != nil {
		return err
	}

	entries := c.db.SortedEntries()
	var issues []auditIssue

	// --- Weak passwords ---
	for _, e := range entries {
		pw := e.Raw().GetPassword()
		if pw == "" {
			continue
		}
		reason := weakPasswordReason(pw)
		if reason != "" {
			issues = append(issues, auditIssue{
				Entry:  e.DisplayPath(),
				Kind:   "weak_password",
				Detail: reason,
				Field:  "password",
			})
		}
	}

	// --- Reused passwords ---
	// Group by SHA-256 hash to avoid holding plaintext passwords in a map.
	type pwGroup struct {
		hash    string
		plain   string // kept only for collision verification
		entries []string
	}
	groupsByHash := map[string]*pwGroup{}
	for _, e := range entries {
		pw := e.Raw().GetPassword()
		if pw == "" {
			continue
		}
		h := sha256.Sum256([]byte(pw))
		key := hex.EncodeToString(h[:])
		if g, ok := groupsByHash[key]; ok {
			// Verify collision is real (same password), not a hash collision.
			if g.plain == pw {
				g.entries = append(g.entries, e.DisplayPath())
			}
		} else {
			groupsByHash[key] = &pwGroup{hash: key, plain: pw, entries: []string{e.DisplayPath()}}
		}
	}
	for _, g := range groupsByHash {
		if len(g.entries) > 1 {
			issues = append(issues, auditIssue{
				Entry:  strings.Join(g.entries, ", "),
				Kind:   "reused_password",
				Detail: fmt.Sprintf("Same password used by %d entries", len(g.entries)),
				Field:  "password",
			})
		}
	}

	// --- Missing fields ---
	for _, e := range entries {
		if e.Raw().GetContent("UserName") == "" {
			issues = append(issues, auditIssue{
				Entry:  e.DisplayPath(),
				Kind:   "missing_field",
				Detail: "Username is empty",
				Field:  "username",
			})
		}
		if e.Raw().GetContent("URL") == "" {
			issues = append(issues, auditIssue{
				Entry:  e.DisplayPath(),
				Kind:   "missing_field",
				Detail: "URL is empty",
				Field:  "url",
			})
		}
	}

	// --- Duplicate OTP seeds ---
	otps := map[string][]string{}
	for _, e := range entries {
		otp := e.OtpURI()
		if otp == "" {
			continue
		}
		otps[otp] = append(otps[otp], e.DisplayPath())
	}
	for _, paths := range otps {
		if len(paths) > 1 {
			issues = append(issues, auditIssue{
				Entry:  strings.Join(paths, ", "),
				Kind:   "duplicate_otp",
				Detail: fmt.Sprintf("Same TOTP seed used by %d entries", len(paths)),
				Field:  "otp",
			})
		}
	}

	// --- Output ---
	if cmd.JSON {
		fmt.Fprintln(c.out, auditJSON(issues))
		return nil
	}

	if len(issues) == 0 {
		fmt.Fprintln(c.out, color.Green("Audit: no issues found"))
		return nil
	}

	// Group by kind.
	kinds := []string{"weak_password", "reused_password", "missing_field", "duplicate_otp"}
	kindLabels := map[string]string{
		"weak_password":   "Weak passwords",
		"reused_password": "Reused passwords",
		"missing_field":   "Missing fields",
		"duplicate_otp":   "Duplicate OTP seeds",
	}

	counts := map[string]int{}
	for _, iss := range issues {
		counts[iss.Kind]++
	}

	for _, kind := range kinds {
		cnt := counts[kind]
		if cnt == 0 {
			continue
		}
		label := kindLabels[kind]
		if cnt > 0 {
			label = color.Yellow(label)
		}
		fmt.Fprintf(c.out, "\n%s (%d):\n", label, cnt)
		for _, iss := range issues {
			if iss.Kind != kind {
				continue
			}
			entryPart := color.Bold(iss.Entry)
			if kind == "reused_password" || kind == "duplicate_otp" {
				// Multiple entries — join them colored.
				parts := strings.Split(iss.Entry, ", ")
				colored := make([]string, len(parts))
				for i, p := range parts {
					colored[i] = color.Bold(p)
				}
				entryPart = strings.Join(colored, ", ")
			}
			fmt.Fprintf(c.out, "  %s: %s\n", entryPart, iss.Detail)
		}
	}

	fmt.Fprintf(c.out, "\n%s\n", color.Red(fmt.Sprintf("Audit: %d issue(s) found", len(issues))))
	return nil
}

// weakPasswordReason returns a description if the password is weak, or empty string.
func weakPasswordReason(pw string) string {
	if len(pw) < 8 {
		return fmt.Sprintf("Too short (%d characters, minimum 8)", len(pw))
	}
	var hasUpper, hasLower, hasDigit, hasSymbol bool
	for _, r := range pw {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSymbol = true
		}
	}
	classes := 0
	for _, b := range []bool{hasUpper, hasLower, hasDigit, hasSymbol} {
		if b {
			classes++
		}
	}
	if classes < 2 {
		return "Only one character class (need at least 2: upper, lower, digit, symbol)"
	}
	if len(pw) < 12 && classes < 3 {
		return fmt.Sprintf("Short password with only %d character classes", classes)
	}
	if isCommonPassword(pw) {
		return "Common/easily guessable password"
	}
	return ""
}

// isCommonPassword checks against a small list of the most common passwords.
func isCommonPassword(pw string) bool {
	lower := strings.ToLower(pw)
	_, ok := commonPasswords[lower]
	return ok
}

// commonPasswords is a minimal set of the most commonly used passwords.
var commonPasswords = map[string]bool{
	"password": true, "123456": true, "12345678": true, "123456789": true,
	"1234567890": true, "qwerty": true, "qwerty123": true, "abc123": true,
	"111111": true, "123123": true, "admin": true, "letmein": true,
	"welcome": true, "monkey": true, "dragon": true, "master": true,
	"football": true, "baseball": true, "iloveyou": true, "trustno1": true,
	"sunshine": true, "princess": true, "1234": true, "12345": true,
	"1234567": true, "passw0rd": true, "password1": true, "p@ssword": true,
}

func auditJSON(issues []auditIssue) string {
	type out struct {
		Issues []auditIssue `json:"issues"`
		Count  int          `json:"count"`
	}
	o := out{Issues: issues, Count: len(issues)}
	if o.Issues == nil {
		o.Issues = []auditIssue{}
	}
	b, _ := json.MarshalIndent(o, "", "  ")
	return string(b)
}
