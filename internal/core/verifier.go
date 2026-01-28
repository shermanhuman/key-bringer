package core

// Verifier handles TOTP code validation.
type Verifier interface {
	// Validate checks if the provided TOTP code is valid.
	Validate(code string) bool
}
