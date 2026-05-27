package pwgen

import (
	"errors"
	"io"
	"strings"
	"testing"
)

type failReader struct{}

func (failReader) Read([]byte) (int, error) { return 0, errors.New("rand failure") }

// flakyReader returns zero bytes for the first `ok` reads then errors.
type flakyReader struct {
	ok    int
	calls int
}

func (f *flakyReader) Read(p []byte) (int, error) {
	f.calls++
	if f.calls > f.ok {
		return 0, errors.New("rand exhausted")
	}
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

func withReader(t *testing.T, r io.Reader) {
	t.Helper()
	orig := randReader
	randReader = r
	t.Cleanup(func() { randReader = orig })
}

func containsAny(s, chars string) bool {
	for _, r := range s {
		if strings.ContainsRune(chars, r) {
			return true
		}
	}
	return false
}

func containsAllGroups(s string, lower, upper, digits, symbols bool) bool {
	if lower && !containsAny(s, lowerSet) {
		return false
	}
	if upper && !containsAny(s, upperSet) {
		return false
	}
	if digits && !containsAny(s, digitSet) {
		return false
	}
	if symbols && !containsAny(s, symbolSet) {
		return false
	}
	return true
}

func TestGenerate_Length(t *testing.T) {
	for _, length := range []int{8, 16, 32, 64} {
		pw, err := Generate(length, false, false, false, false, "")
		if err != nil {
			t.Fatalf("Generate(%d) error: %v", length, err)
		}
		if len(pw) != length {
			t.Errorf("Generate(%d) = len %d, want %d", length, len(pw), length)
		}
	}
}

func TestGenerate_LengthZero(t *testing.T) {
	_, err := Generate(0, false, false, false, false, "")
	if err == nil {
		t.Error("expected error for length 0")
	}
}

func TestGenerate_TooShortForGroups(t *testing.T) {
	// 4 groups but length 3
	_, err := Generate(3, true, true, true, true, "")
	if err == nil {
		t.Error("expected error when length < number of groups")
	}
}

func TestGenerate_AllCharGroups(t *testing.T) {
	pw, err := Generate(32, true, true, true, true, "")
	if err != nil {
		t.Fatal(err)
	}
	if !containsAllGroups(pw, true, true, true, true) {
		t.Errorf("password missing required groups: %s", pw)
	}
}

func TestGenerate_LowerOnly(t *testing.T) {
	pw, err := Generate(16, true, false, false, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if containsAny(pw, upperSet) || containsAny(pw, digitSet) || containsAny(pw, symbolSet) {
		t.Errorf("lower-only password contains other chars: %s", pw)
	}
}

func TestGenerate_DigitsSymbolsOnly(t *testing.T) {
	pw, err := Generate(12, false, false, true, true, "")
	if err != nil {
		t.Fatal(err)
	}
	if containsAny(pw, lowerSet) || containsAny(pw, upperSet) {
		t.Errorf("digits+symbols password contains letters: %s", pw)
	}
	if !containsAny(pw, digitSet) {
		t.Error("digits+symbols password missing digits")
	}
	if !containsAny(pw, symbolSet) {
		t.Error("digits+symbols password missing symbols")
	}
}

func TestGenerate_CustomSymbols(t *testing.T) {
	custom := "@#$%"
	pw, err := Generate(20, false, false, false, true, custom)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range pw {
		if !strings.ContainsRune(custom, r) {
			t.Errorf("custom symbols password contains unexpected char: %c", r)
		}
	}
}

func TestGenerate_Randomness(t *testing.T) {
	// Generate two passwords — they should differ.
	a, _ := Generate(24, true, true, true, true, "")
	b, _ := Generate(24, true, true, true, true, "")
	if a == b {
		t.Error("two generated passwords are identical — improbable")
	}
}

func TestCharset_All(t *testing.T) {
	s := Charset(true, true, true, true, "")
	if !strings.Contains(s, lowerSet) || !strings.Contains(s, upperSet) || !strings.Contains(s, digitSet) || !strings.Contains(s, symbolSet) {
		t.Error("charset missing expected groups")
	}
}

func TestCharset_None(t *testing.T) {
	s := Charset(false, false, false, false, "")
	if s != defaultSet {
		t.Errorf("empty charset should return defaultSet, got %q", s)
	}
}

func TestGenerate_RandError_RequiredGroup(t *testing.T) {
	withReader(t, failReader{})
	if _, err := Generate(8, true, true, true, true, ""); err == nil {
		t.Error("expected rand error from required-group loop")
	}
}

func TestGenerate_RandError_Fill(t *testing.T) {
	// No required groups → goes straight to the fill loop's randomChar.
	withReader(t, failReader{})
	if _, err := Generate(8, false, false, false, false, ""); err == nil {
		t.Error("expected rand error from fill loop")
	}
}

func TestGenerate_RandError_Shuffle(t *testing.T) {
	// Use a reader that succeeds for randomChar calls then fails for shuffle.
	// randomChar uses rand.Int → reads bytes from reader; shuffle uses rand.Int too.
	// Easiest: drive a reader that returns zeros for first N reads, then errors.
	// 1 required + 3 fill = 4 randomChar calls; shuffle fires next → fail on 5th.
	withReader(t, &flakyReader{ok: 4})
	if _, err := Generate(4, true, false, false, false, ""); err == nil {
		t.Error("expected rand error from shuffle loop")
	}
}

func TestRandomChar_Error(t *testing.T) {
	withReader(t, failReader{})
	if _, err := randomChar("abc"); err == nil {
		t.Error("expected randomChar error")
	}
}

func TestCharset_CustomSymbols(t *testing.T) {
	custom := "#$@"
	s := Charset(false, false, false, true, custom)
	if s != custom {
		t.Errorf("custom symbols: want %q, got %q", custom, s)
	}
}
