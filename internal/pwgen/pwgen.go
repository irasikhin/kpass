package pwgen

import (
	"crypto/rand"
	"fmt"
	"io"
	"math/big"
	"strings"
)

// randReader is the source of randomness; tests replace it to drive error paths.
var randReader io.Reader = rand.Reader

const (
	lowerSet   = "abcdefghijklmnopqrstuvwxyz"
	upperSet   = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digitSet   = "0123456789"
	symbolSet  = "!@#$%^&*()-_=+[]{}:,.?"
	defaultSet = lowerSet + upperSet + digitSet + symbolSet
)

// Charset assembles the charset matching the Python password_charset rules.
// If customSymbols is non-empty, it replaces the default symbolSet when
// symbols are enabled.
func Charset(lower, upper, digits, symbols bool, customSymbols string) string {
	symSet := symbolSet
	if customSymbols != "" {
		symSet = customSymbols
	}
	var groups []string
	if lower {
		groups = append(groups, lowerSet)
	}
	if upper {
		groups = append(groups, upperSet)
	}
	if digits {
		groups = append(groups, digitSet)
	}
	if symbols {
		groups = append(groups, symSet)
	}
	if len(groups) == 0 {
		return defaultSet
	}
	var b strings.Builder
	for _, g := range groups {
		b.WriteString(g)
	}
	return b.String()
}

// Generate mirrors Python generate_password.
// If customSymbols is non-empty, it replaces the default symbolSet when
// symbols are enabled.
func Generate(length int, lower, upper, digits, symbols bool, customSymbols string) (string, error) {
	if length <= 0 {
		return "", fmt.Errorf("password length must be positive")
	}
	charset := Charset(lower, upper, digits, symbols, customSymbols)
	symSet := symbolSet
	if customSymbols != "" {
		symSet = customSymbols
	}
	var required []string
	if lower {
		required = append(required, lowerSet)
	}
	if upper {
		required = append(required, upperSet)
	}
	if digits {
		required = append(required, digitSet)
	}
	if symbols {
		required = append(required, symSet)
	}
	if len(required) > 0 && length < len(required) {
		return "", fmt.Errorf("password length is shorter than the number of required character groups")
	}

	out := make([]byte, 0, length)
	for _, g := range required {
		c, err := randomChar(g)
		if err != nil {
			return "", err
		}
		out = append(out, c)
	}
	for len(out) < length {
		c, err := randomChar(charset)
		if err != nil {
			return "", err
		}
		out = append(out, c)
	}
	// Shuffle (Fisher–Yates with crypto/rand).
	for i := len(out) - 1; i > 0; i-- {
		jBig, err := rand.Int(randReader, big.NewInt(int64(i+1)))
		if err != nil {
			return "", err
		}
		j := int(jBig.Int64())
		out[i], out[j] = out[j], out[i]
	}
	return string(out), nil
}

func randomChar(s string) (byte, error) {
	n := big.NewInt(int64(len(s)))
	idx, err := rand.Int(randReader, n)
	if err != nil {
		return 0, err
	}
	return s[idx.Int64()], nil
}
