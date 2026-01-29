package totp

import (
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// Verifier validates TOTP codes against a shared secret.
type Verifier struct {
	seed string
}

// NewVerifier creates a new TOTP verifier with the given Base32-encoded seed.
func NewVerifier(seed string) *Verifier {
	return &Verifier{seed: seed}
}

// Validate checks if the provided 6-digit code is valid for the current time window.
// Allows ±1 time window (90 seconds total) to handle minor clock drift.
func (v *Verifier) Validate(code string) bool {
	valid, _ := totp.ValidateCustom(code, v.seed, time.Now(), totp.ValidateOpts{
		Period:    30,
		Skew:      1, // Allow 1 period before/after
		Digits:    6,
		Algorithm: otp.AlgorithmSHA1,
	})
	return valid
}

// ValidateWithTime checks if the code is valid at a specific time (for testing).
func (v *Verifier) ValidateWithTime(code string, t time.Time) bool {
	// Generate the expected code for the given time and compare
	expected, err := totp.GenerateCode(v.seed, t)
	if err != nil {
		return false
	}
	return expected == code
}
