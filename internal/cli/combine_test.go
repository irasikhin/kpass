package cli

import (
	"strings"
	"testing"
)

// addOtpEntry inserts an "OTP-only" entry under otp/email via the CLI so the
// scenario mirrors what import-pass / import-otp would produce.
func (f *fixture) addOtpEntry(t *testing.T, otpURI string) {
	t.Helper()
	_, stderr, code := f.runCLI("insert", "otp/email", "--password", "x", "--otp", otpURI, "-f")
	if code != 0 {
		t.Fatalf("insert otp/email: code=%d stderr=%s", code, stderr)
	}
}

// --- basic merge: OTP into existing login (the headline use case) ----------

func TestCombineAttachesOtpToExistingLogin(t *testing.T) {
	f := newFixture(t)
	// internet/email exists (seeded). Add a separate otp-only entry.
	f.addOtpEntry(t, "otpauth://totp/X?secret=ABC&issuer=I")

	_, stderr, code := f.runCLI("combine", "otp/email", "internet/email", "-f")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	db := openSeededDB(t, f.dbPath, "master-password")
	dst := findEntryByPath(db, "internet/email")
	if dst == nil {
		t.Fatal("dst missing")
	}
	if got := dst.GetContent("otp"); got != "otpauth://totp/X?secret=ABC&issuer=I" {
		t.Fatalf("dst.otp=%q", got)
	}
	// dst password should be unchanged (was pw-email).
	if got := dst.GetPassword(); got != "pw-email" {
		t.Fatalf("dst password mutated: %q", got)
	}
	// src must remain (no --delete-src).
	if findEntryByPath(db, "otp/email") == nil {
		t.Fatal("src removed without --delete-src")
	}
}

func TestCombineDeleteSrcRemovesSource(t *testing.T) {
	f := newFixture(t)
	f.addOtpEntry(t, "otpauth://totp/X?secret=ABC")
	_, _, code := f.runCLI("combine", "otp/email", "internet/email", "-f", "--delete-src")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	db := openSeededDB(t, f.dbPath, "master-password")
	if findEntryByPath(db, "otp/email") != nil {
		t.Fatal("src should be deleted")
	}
	if got := findEntryByPath(db, "internet/email").GetContent("otp"); got == "" {
		t.Fatal("dst should still have the otp field")
	}
}

// --- conflict policies ------------------------------------------------------

func TestCombineKeepPolicyKeepsDst(t *testing.T) {
	f := newFixture(t)
	// Both entries already exist; passwords differ.
	_, _, code := f.runCLI("combine", "work/email", "internet/email", "-f", "--on-conflict=keep")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	db := openSeededDB(t, f.dbPath, "master-password")
	dst := findEntryByPath(db, "internet/email")
	if dst.GetPassword() != "pw-email" {
		t.Fatalf("password=%q (keep should preserve dst)", dst.GetPassword())
	}
}

func TestCombineOverwritePolicyTakesSrc(t *testing.T) {
	f := newFixture(t)
	_, _, code := f.runCLI("combine", "work/email", "internet/email", "-f", "--on-conflict=overwrite")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	db := openSeededDB(t, f.dbPath, "master-password")
	dst := findEntryByPath(db, "internet/email")
	if dst.GetPassword() != "pw-work" {
		t.Fatalf("password=%q (overwrite should take src)", dst.GetPassword())
	}
	if dst.GetContent("UserName") != "worker" {
		t.Fatalf("username=%q", dst.GetContent("UserName"))
	}
}

func TestCombineBothPolicyStashesAlt(t *testing.T) {
	f := newFixture(t)
	_, _, code := f.runCLI("combine", "work/email", "internet/email", "-f", "--on-conflict=both")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	db := openSeededDB(t, f.dbPath, "master-password")
	dst := findEntryByPath(db, "internet/email")
	if dst.GetPassword() != "pw-email" {
		t.Fatalf("dst.password=%q (should remain)", dst.GetPassword())
	}
	if got := dst.GetContent("password.alt"); got != "pw-work" {
		t.Fatalf("password.alt=%q", got)
	}
	if got := dst.GetContent("username.alt"); got != "worker" {
		t.Fatalf("username.alt=%q", got)
	}
}

// --- ask policy via stdin ---------------------------------------------------

func TestCombineAskAcceptingSrc(t *testing.T) {
	f := newFixture(t)
	// Send "1" for each conflict (src wins), then "y" to confirm.
	// internet/email vs work/email conflict on: username, password, url,
	// notes (title is "email" for both). Reply "1" 4 times, then "y".
	stdin := strings.Repeat("1\n", 4) + "y\n"
	_, stderr, code := f.runCLIWith(runOpts{stdin: stdin}, "combine", "work/email", "internet/email")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	db := openSeededDB(t, f.dbPath, "master-password")
	dst := findEntryByPath(db, "internet/email")
	if dst.GetPassword() != "pw-work" {
		t.Fatalf("password=%q", dst.GetPassword())
	}
}

func TestCombineAskAbort(t *testing.T) {
	f := newFixture(t)
	stdin := "a\n"
	_, stderr, code := f.runCLIWith(runOpts{stdin: stdin}, "combine", "work/email", "internet/email")
	if code == 0 {
		t.Fatal("expected non-zero exit on abort")
	}
	if !strings.Contains(stderr, "Aborted") {
		t.Fatalf("stderr=%q", stderr)
	}
	// Confirm no changes leaked.
	db := openSeededDB(t, f.dbPath, "master-password")
	if got := findEntryByPath(db, "internet/email").GetPassword(); got != "pw-email" {
		t.Fatalf("dst.password=%q (should be untouched)", got)
	}
}

func TestCombineAskDefaultIsKeep(t *testing.T) {
	f := newFixture(t)
	// Empty answers (just newlines) for each conflict — should choose [2] keep
	// dst by default. Then "y" to apply.
	stdin := strings.Repeat("\n", 4) + "y\n"
	_, _, code := f.runCLIWith(runOpts{stdin: stdin}, "combine", "work/email", "internet/email")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	db := openSeededDB(t, f.dbPath, "master-password")
	if got := findEntryByPath(db, "internet/email").GetPassword(); got != "pw-email" {
		t.Fatalf("password=%q (default should keep dst)", got)
	}
}

// --- --only filter ----------------------------------------------------------

func TestCombineOnlyRestrictsFields(t *testing.T) {
	f := newFixture(t)
	f.addOtpEntry(t, "otpauth://totp/X?secret=ABC")
	// src.password=x but dst should NOT get it because --only=otp.
	_, _, code := f.runCLI("combine", "otp/email", "internet/email", "-f", "--only", "otp")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	db := openSeededDB(t, f.dbPath, "master-password")
	dst := findEntryByPath(db, "internet/email")
	if dst.GetPassword() != "pw-email" {
		t.Fatalf("password mutated: %q (only=otp should not touch it)", dst.GetPassword())
	}
	if dst.GetContent("otp") == "" {
		t.Fatal("otp should have been adopted")
	}
}

func TestCombineOnlyUnknownErrors(t *testing.T) {
	f := newFixture(t)
	_, stderr, code := f.runCLI("combine", "work/email", "internet/email", "-f", "--only", "bogus")
	if code == 0 {
		t.Fatal("expected error")
	}
	if !strings.Contains(stderr, "unknown field") {
		t.Fatalf("stderr=%q", stderr)
	}
}

// --- dry-run ----------------------------------------------------------------

func TestCombineDryRunDoesNotMutate(t *testing.T) {
	f := newFixture(t)
	f.addOtpEntry(t, "otpauth://totp/X?secret=ABC")
	stdout, _, code := f.runCLI("combine", "otp/email", "internet/email", "--dry-run", "-f")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(stdout, "Combine plan") {
		t.Fatalf("expected plan header, got: %s", stdout)
	}
	db := openSeededDB(t, f.dbPath, "master-password")
	if got := findEntryByPath(db, "internet/email").GetContent("otp"); got != "" {
		t.Fatalf("otp leaked into dst on --dry-run: %q", got)
	}
}

// --- guards ----------------------------------------------------------------

func TestCombineSelfErrors(t *testing.T) {
	f := newFixture(t)
	_, stderr, code := f.runCLI("combine", "internet/email", "internet/email", "-f")
	if code == 0 {
		t.Fatal("expected error")
	}
	if !strings.Contains(stderr, "same entry") {
		t.Fatalf("stderr=%q", stderr)
	}
}

func TestCombineMissingEntryErrors(t *testing.T) {
	f := newFixture(t)
	_, stderr, code := f.runCLI("combine", "no-such", "internet/email", "-f")
	if code == 0 {
		t.Fatal("expected error")
	}
	if stderr == "" {
		t.Fatal("expected error message")
	}
}

// --- tags + custom fields union --------------------------------------------

func TestCombineTagsAreUnioned(t *testing.T) {
	f := newFixture(t)
	f.seedTags(t, map[string]string{
		"internet/email": "personal",
		"work/email":     "work,critical",
	})
	_, _, code := f.runCLI("combine", "work/email", "internet/email", "-f", "--on-conflict=keep")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	db := openSeededDB(t, f.dbPath, "master-password")
	tags := findEntryByPath(db, "internet/email").Tags
	if !strings.Contains(tags, "personal") || !strings.Contains(tags, "work") || !strings.Contains(tags, "critical") {
		t.Fatalf("tags=%q (should union all three)", tags)
	}
}

// --- delete-src safety ------------------------------------------------------

func TestCombineDeleteSrcRefusesOnLossyKeep(t *testing.T) {
	f := newFixture(t)
	// work/email vs internet/email conflict on password+username+url+notes.
	// --on-conflict=keep → silent loss → safety must block --delete-src.
	_, stderr, code := f.runCLIWith(runOpts{stdin: "y\n"},
		"combine", "work/email", "internet/email",
		"--on-conflict=keep", "--delete-src")
	if code == 0 {
		t.Fatal("expected non-zero exit (lossy delete-src)")
	}
	if !strings.Contains(stderr, "discard src values") {
		t.Fatalf("stderr=%q", stderr)
	}
	db := openSeededDB(t, f.dbPath, "master-password")
	if findEntryByPath(db, "work/email") == nil {
		t.Fatal("src should not have been deleted")
	}
}

func TestCombineDeleteSrcLossyAcceptsForce(t *testing.T) {
	f := newFixture(t)
	_, _, code := f.runCLI("combine", "work/email", "internet/email",
		"--on-conflict=keep", "--delete-src", "-f")
	if code != 0 {
		t.Fatalf("code=%d (force should override lossy guard)", code)
	}
	db := openSeededDB(t, f.dbPath, "master-password")
	if findEntryByPath(db, "work/email") != nil {
		t.Fatal("src should be gone with --force")
	}
}

func TestCombineDeleteSrcWithBothPolicyNotLossy(t *testing.T) {
	f := newFixture(t)
	// both policy preserves conflicting src values as <field>.alt → no loss.
	_, _, code := f.runCLI("combine", "work/email", "internet/email",
		"--on-conflict=both", "--delete-src", "-f")
	if code != 0 {
		t.Fatalf("code=%d (both should not trigger lossy guard)", code)
	}
	db := openSeededDB(t, f.dbPath, "master-password")
	if findEntryByPath(db, "work/email") != nil {
		t.Fatal("src should be deleted")
	}
	dst := findEntryByPath(db, "internet/email")
	if got := dst.GetContent("password.alt"); got != "pw-work" {
		t.Fatalf("password.alt=%q", got)
	}
}

func TestCombineDeleteSrcOnlyFilterIntentional(t *testing.T) {
	f := newFixture(t)
	// --only=otp explicitly scopes the merge. Src has plenty of other
	// fields but the user said "only otp"; --delete-src is intentional.
	// Add the otp first.
	if _, _, code := f.runCLI("edit", "work/email", "--otp", "otpauth://totp/X?secret=ABC"); code != 0 {
		t.Fatalf("edit failed: %d", code)
	}
	_, stderr, code := f.runCLIWith(runOpts{stdin: "y\n"},
		"combine", "work/email", "internet/email",
		"--only=otp", "--delete-src")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s (--only should not trigger lossy guard)", code, stderr)
	}
	db := openSeededDB(t, f.dbPath, "master-password")
	if findEntryByPath(db, "work/email") != nil {
		t.Fatal("src should be deleted")
	}
	if got := findEntryByPath(db, "internet/email").GetContent("otp"); got == "" {
		t.Fatal("otp should have been adopted")
	}
}

func TestCombineCustomFieldsAdopted(t *testing.T) {
	f := newFixture(t)
	// Put a custom field on src via edit -F.
	if _, _, code := f.runCLI("edit", "work/email", "-F", "env=prod"); code != 0 {
		t.Fatalf("edit -F failed: %d", code)
	}
	_, _, code := f.runCLI("combine", "work/email", "internet/email", "-f", "--on-conflict=keep")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	db := openSeededDB(t, f.dbPath, "master-password")
	if got := findEntryByPath(db, "internet/email").GetContent("env"); got != "prod" {
		t.Fatalf("env=%q (custom field should be adopted)", got)
	}
}
