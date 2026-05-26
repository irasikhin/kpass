package pwgen

import (
	"fmt"
	"math"
	"strings"
	"unicode"
)

// Strength represents a password strength assessment.
type Strength struct {
	Bits  float64
	Label string
	Bar   string // 10-segment bar like "████████░░"
}

// Assess returns a Strength assessment for the given password.
func Assess(password string) Strength {
	bits := estimateBits(password)
	s := Strength{Bits: bits}

	switch {
	case bits < 28:
		s.Label = "Very Weak"
		s.Bar = bar(1)
	case bits < 36:
		s.Label = "Weak"
		s.Bar = bar(2)
	case bits < 60:
		s.Label = "Fair"
		s.Bar = bar(5)
	case bits < 80:
		s.Label = "Strong"
		s.Bar = bar(8)
	default:
		s.Label = "Very Strong"
		s.Bar = bar(10)
	}
	return s
}

// estimateBits computes a simple entropy estimate based on length and
// character class diversity.
func estimateBits(password string) float64 {
	if len(password) == 0 {
		return 0
	}

	var hasUpper, hasLower, hasDigit, hasSymbol bool
	for _, r := range password {
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

	pool := 0
	if hasLower {
		pool += 26
	}
	if hasUpper {
		pool += 26
	}
	if hasDigit {
		pool += 10
	}
	if hasSymbol {
		pool += 32
	}
	if pool == 0 {
		pool = 26 // assume lowercase only
	}

	return float64(len(password)) * math.Log2(float64(pool))
}

func bar(segments int) string {
	filled := strings.Repeat("█", segments)
	empty := strings.Repeat("░", 10-segments)
	return fmt.Sprintf("[%s%s]", filled, empty)
}
