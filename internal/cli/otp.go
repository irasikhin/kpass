package cli

import (
	"time"

	"github.com/irasikhin/kpass/internal/otp"
)

// OtpCoder is the test injection seam; when nil, the real otp package is
// used.
var OtpCoder func(uri string) (string, error)

func otpCode(uri string) (string, error) {
	if OtpCoder != nil {
		return OtpCoder(uri)
	}
	return otp.Generate(uri, time.Time{})
}

func isOtpField(field string) bool {
	return field == "otp" || field == "totp" || field == "code"
}

// resolveFieldValue returns the OTP code if field is an OTP field, otherwise
// the raw value unchanged. Collapses the (isOtpField → otpCode) dispatch that
// appears at every command site that prints or copies a field value.
func resolveFieldValue(field, raw string) (string, error) {
	if !isOtpField(field) {
		return raw, nil
	}
	return otpCode(raw)
}
