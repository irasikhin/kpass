package otp

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"hash"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// NowHook is the test injection point for the current time. When non-nil and
// caller passes a zero time, Generate uses NowHook() instead of time.Now.
var NowHook func() time.Time

// Spec holds the parsed otpauth URI parameters.
type Spec struct {
	Secret    string
	Digits    int
	Period    int
	Algorithm string
}

// Parse mirrors Python parse_otpauth.
func Parse(uri string) (Spec, error) {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "otpauth" {
		return Spec{}, fmt.Errorf("oTP value is not an otpauth URI")
	}
	q := u.Query()
	secret := q.Get("secret")
	if secret == "" {
		return Spec{}, fmt.Errorf("oTP URI does not contain a secret")
	}
	digits := 6
	if v := q.Get("digits"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Spec{}, fmt.Errorf("invalid OTP digits: %s", v)
		}
		digits = n
	}
	period := 30
	if v := q.Get("period"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Spec{}, fmt.Errorf("invalid OTP period: %s", v)
		}
		period = n
	}
	algo := strings.ToUpper(q.Get("algorithm"))
	if algo == "" {
		algo = "SHA1"
	}
	return Spec{Secret: secret, Digits: digits, Period: period, Algorithm: algo}, nil
}

// Generate computes a TOTP code for `uri` at time `now`. If `now` is the zero
// time, the current wall time is used. Mirrors Python generate_totp.
func Generate(uri string, now time.Time) (string, error) {
	if uri == "" {
		return "", fmt.Errorf("entry does not contain OTP data")
	}
	spec, err := Parse(uri)
	if err != nil {
		return "", err
	}
	var hasher func() hash.Hash
	switch spec.Algorithm {
	case "SHA1":
		hasher = sha1.New
	case "SHA256":
		hasher = sha256.New
	case "SHA512":
		hasher = sha512.New
	default:
		return "", fmt.Errorf("unsupported OTP algorithm: %s", spec.Algorithm)
	}

	normalized := strings.ToUpper(spec.Secret)
	if pad := (8 - len(normalized)%8) % 8; pad > 0 {
		normalized += strings.Repeat("=", pad)
	}
	key, err := base32.StdEncoding.DecodeString(normalized)
	if err != nil {
		return "", fmt.Errorf("invalid OTP secret: %v", err)
	}

	if now.IsZero() {
		if NowHook != nil {
			now = NowHook()
		} else {
			now = time.Now()
		}
	}
	timestamp := now.Unix()
	counter := timestamp / int64(spec.Period)

	mac := hmac.New(hasher, key)
	payload := make([]byte, 8)
	binary.BigEndian.PutUint64(payload, uint64(counter))
	mac.Write(payload)
	digest := mac.Sum(nil)
	offset := digest[len(digest)-1] & 0x0F
	code := binary.BigEndian.Uint32(digest[offset:offset+4]) & 0x7FFFFFFF

	mod := uint32(1)
	for i := 0; i < spec.Digits; i++ {
		mod *= 10
	}
	out := fmt.Sprintf("%0*d", spec.Digits, code%mod)
	return out, nil
}
