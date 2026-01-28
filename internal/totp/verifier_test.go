package totp

import (
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

const testSeed = "JBSWY3DPEHPK3PXP" // Standard test seed

func TestValidCode(t *testing.T) {
	v := NewVerifier(testSeed)

	// Generate a valid code for right now
	code, err := totp.GenerateCode(testSeed, time.Now())
	if err != nil {
		t.Fatalf("failed to generate code: %v", err)
	}

	if !v.Validate(code) {
		t.Errorf("expected valid code %s to pass validation", code)
	}
}

func TestInvalidCode(t *testing.T) {
	v := NewVerifier(testSeed)

	// 000000 should never be valid (statistically improbable)
	if v.Validate("000000") {
		t.Error("expected invalid code 000000 to fail validation")
	}
}

func TestExpiredCode(t *testing.T) {
	v := NewVerifier(testSeed)

	// Generate a code from 2 minutes ago (outside 30s window)
	oldTime := time.Now().Add(-2 * time.Minute)
	oldCode, err := totp.GenerateCode(testSeed, oldTime)
	if err != nil {
		t.Fatalf("failed to generate old code: %v", err)
	}

	// The library allows a small skew, but 2 minutes should be rejected
	if v.Validate(oldCode) {
		t.Error("expected expired code to fail validation")
	}
}

func TestWrongSeed(t *testing.T) {
	v := NewVerifier(testSeed)

	// Generate code with different seed
	differentSeed := "GEZDGNBVGY3TQOJQ"
	code, err := totp.GenerateCode(differentSeed, time.Now())
	if err != nil {
		t.Fatalf("failed to generate code: %v", err)
	}

	if v.Validate(code) {
		t.Error("expected code from different seed to fail validation")
	}
}

func TestEmptyCode(t *testing.T) {
	v := NewVerifier(testSeed)

	if v.Validate("") {
		t.Error("expected empty code to fail validation")
	}
}

func TestMalformedCode(t *testing.T) {
	v := NewVerifier(testSeed)

	testCases := []string{
		"12345",    // Too short
		"1234567",  // Too long
		"abcdef",   // Non-numeric
		"12 34 56", // Spaces
		"123-456",  // Dashes
	}

	for _, code := range testCases {
		if v.Validate(code) {
			t.Errorf("expected malformed code %q to fail validation", code)
		}
	}
}
