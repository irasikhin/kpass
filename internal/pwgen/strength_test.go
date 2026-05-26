package pwgen

import (
	"testing"
)

func TestAssess_Empty(t *testing.T) {
	s := Assess("")
	if s.Bits != 0 {
		t.Errorf("empty password: bits = %f, want 0", s.Bits)
	}
	if s.Label != "Very Weak" {
		t.Errorf("empty password: label = %q, want Very Weak", s.Label)
	}
}

func TestAssess_Weak(t *testing.T) {
	// "abc" = 3 * log2(26) ≈ 3*4.7 ≈ 14.1 bits
	s := Assess("abc")
	if s.Label != "Very Weak" {
		t.Errorf("short password: label = %q, want Very Weak", s.Label)
	}
}

func TestAssess_Strong(t *testing.T) {
	// 24 chars with mixed groups should be > 80 bits
	s := Assess("aB3$xY9!kL2@mN5#pQ8&rT4^")
	if s.Bits < 80 {
		t.Errorf("complex password: bits = %f, want >= 80", s.Bits)
	}
	if s.Label != "Very Strong" {
		t.Errorf("complex password: label = %q, want Very Strong", s.Label)
	}
}

func TestAssess_BarLength(t *testing.T) {
	for _, pw := range []string{"", "short", "loooooooooooooooong!!!A1"} {
		s := Assess(pw)
		runes := len([]rune(s.Bar))
		if runes != 12 { // [ + 10 segments + ]
			t.Errorf("bar rune count = %d for %q, want 12", runes, pw)
		}
	}
}

func TestAssess_Monotonic(t *testing.T) {
	// Longer password with same chars → higher bits
	short := Assess("aB3")
	long := Assess("aB3aB3aB3aB3")
	if long.Bits <= short.Bits {
		t.Errorf("long password bits (%f) <= short bits (%f)", long.Bits, short.Bits)
	}
}

func TestAssess_Diversity(t *testing.T) {
	// Same length, more diversity → higher bits
	same := Assess("abcdefghijklmnop")
	mixed := Assess("aBcDeFgHiJkLmNoP")
	if mixed.Bits <= same.Bits {
		t.Errorf("mixed case bits (%f) <= lowercase bits (%f)", mixed.Bits, same.Bits)
	}
}
