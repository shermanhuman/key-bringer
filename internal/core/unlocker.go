package core

// Unlocker handles applying encryption keys to unlock storage.
type Unlocker interface {
	// ApplyKey takes the decryption secret and unlocks the target.
	ApplyKey(secret string) error
}
