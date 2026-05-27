package otp

import (
	"testing"
	"time"
)

func TestParse_Valid(t *testing.T) {
	uri := "otpauth://totp/Example:alice@example.com?secret=JBSWY3DPEHPK3PXP&issuer=Example"
	spec, err := Parse(uri)
	if err != nil {
		t.Fatalf("Parse(%q): unexpected error: %v", uri, err)
	}
	if spec.Secret != "JBSWY3DPEHPK3PXP" {
		t.Errorf("secret = %q, want JBSWY3DPEHPK3PXP", spec.Secret)
	}
	if spec.Digits != 6 {
		t.Errorf("digits = %d, want 6", spec.Digits)
	}
	if spec.Period != 30 {
		t.Errorf("period = %d, want 30", spec.Period)
	}
	if spec.Algorithm != "SHA1" {
		t.Errorf("algorithm = %q, want SHA1", spec.Algorithm)
	}
}

func TestParse_DigitsPeriod(t *testing.T) {
	uri := "otpauth://totp/foo?secret=AAA&digits=8&period=60"
	spec, err := Parse(uri)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Digits != 8 {
		t.Errorf("digits = %d, want 8", spec.Digits)
	}
	if spec.Period != 60 {
		t.Errorf("period = %d, want 60", spec.Period)
	}
}

func TestParse_InvalidURL(t *testing.T) {
	_, err := Parse("not-a-uri")
	if err == nil {
		t.Error("expected error for invalid URI")
	}
}

func TestParse_NonOtpAuth(t *testing.T) {
	_, err := Parse("https://example.com")
	if err == nil {
		t.Error("expected error for non-otpauth URL")
	}
}

func TestParse_NoSecret(t *testing.T) {
	_, err := Parse("otpauth://totp/foo?digits=6")
	if err == nil {
		t.Error("expected error for missing secret")
	}
}

func TestParse_InvalidDigits(t *testing.T) {
	_, err := Parse("otpauth://totp/foo?secret=AAA&digits=abc")
	if err == nil {
		t.Error("expected error for invalid digits")
	}
}

func TestParse_SHA256(t *testing.T) {
	uri := "otpauth://totp/foo?secret=AAA&algorithm=SHA256"
	spec, err := Parse(uri)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Algorithm != "SHA256" {
		t.Errorf("algorithm = %q, want SHA256", spec.Algorithm)
	}
}

func TestGenerate_Empty(t *testing.T) {
	_, err := Generate("", time.Time{})
	if err == nil {
		t.Error("expected error for empty URI")
	}
}

// Known test vector from RFC 6238 Appendix B.
// Secret: GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ (base32 "12345678901234567890")
// Time: 1970-01-01T00:00:59 UTC, SHA1, 8 digits → 94287082
func TestGenerate_RFC6238_SHA1(t *testing.T) {
	uri := "otpauth://totp/test?secret=GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ&digits=8&algorithm=SHA1"
	// Restore any test hook after this test.
	old := NowHook
	defer func() { NowHook = old }()
	fixedTime := time.Unix(59, 0).UTC()
	NowHook = func() time.Time { return fixedTime }

	code, err := Generate(uri, time.Time{})
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}
	if code != "94287082" {
		t.Errorf("code = %q, want 94287082", code)
	}
}

func TestGenerate_SHA256(t *testing.T) {
	// SHA256 requires a 32-byte key (52 base32 chars).
	// GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQGEZA = "12345678901234567890123456789012"
	uri := "otpauth://totp/test?secret=GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQGEZA&digits=8&algorithm=SHA256"
	old := NowHook
	defer func() { NowHook = old }()
	fixedTime := time.Unix(59, 0).UTC()
	NowHook = func() time.Time { return fixedTime }

	code, err := Generate(uri, time.Time{})
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}
	// RFC 6238 test vector for SHA256, time=59, 8 digits.
	if code != "46119246" {
		t.Errorf("code = %q, want 46119246", code)
	}
}

func TestGenerate_ExplicitTime(t *testing.T) {
	uri := "otpauth://totp/test?secret=GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ&digits=8&algorithm=SHA1"
	fixedTime := time.Unix(1111111109, 0).UTC()

	code, err := Generate(uri, fixedTime)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}
	if code != "07081804" {
		t.Errorf("code = %q, want 07081804", code)
	}
}

func TestGenerate_UnknownAlgorithm(t *testing.T) {
	uri := "otpauth://totp/test?secret=AAA&algorithm=MD5"
	_, err := Generate(uri, time.Now())
	if err == nil {
		t.Error("expected error for unknown algorithm")
	}
}

func TestGenerate_InvalidSecret(t *testing.T) {
	uri := "otpauth://totp/test?secret=!!!!!!"
	_, err := Generate(uri, time.Now())
	if err == nil {
		t.Error("expected error for invalid base32 secret")
	}
}

func TestParse_InvalidPeriod(t *testing.T) {
	_, err := Parse("otpauth://totp/foo?secret=AAA&period=zz")
	if err == nil {
		t.Error("expected error for invalid period")
	}
}

func TestGenerate_ParseError(t *testing.T) {
	if _, err := Generate("not-a-uri", time.Now()); err == nil {
		t.Error("expected Parse error to propagate from Generate")
	}
}

func TestGenerate_SHA512(t *testing.T) {
	// 64-byte secret for SHA512 (base32).
	uri := "otpauth://totp/t?secret=GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQGEZDGNA&digits=8&algorithm=SHA512"
	code, err := Generate(uri, time.Unix(59, 0).UTC())
	if err != nil {
		t.Fatalf("SHA512 Generate error: %v", err)
	}
	if len(code) != 8 {
		t.Errorf("SHA512 code length = %d", len(code))
	}
}

func TestGenerate_WallTimeFallback(t *testing.T) {
	// Exercise the time.Now() branch by clearing NowHook and passing zero time.
	old := NowHook
	NowHook = nil
	defer func() { NowHook = old }()
	uri := "otpauth://totp/t?secret=GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ&digits=6&algorithm=SHA1"
	code, err := Generate(uri, time.Time{})
	if err != nil {
		t.Fatalf("Generate wall-time fallback: %v", err)
	}
	if len(code) != 6 {
		t.Errorf("code length = %d", len(code))
	}
}

func TestGenerate_SixDigits(t *testing.T) {
	uri := "otpauth://totp/test?secret=GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ&digits=6&algorithm=SHA1"
	old := NowHook
	defer func() { NowHook = old }()
	NowHook = func() time.Time { return time.Unix(59, 0).UTC() }

	code, err := Generate(uri, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(code) != 6 {
		t.Errorf("6-digit code has length %d", len(code))
	}
}
