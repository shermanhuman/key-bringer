package totp

import (
	"time"

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
func (v *Verifier) Validate(code string) bool {
	return totp.Validate(code, v.seed)
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
