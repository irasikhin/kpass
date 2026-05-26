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
