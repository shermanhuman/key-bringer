package core

// Unlocker handles applying encryption keys to unlock storage.
type Unlocker interface {
	// ApplyKey takes the target (e.g., ZFS dataset, BitLocker volume) and
	// the decryption secret, then unlocks the target.
	ApplyKey(target, secret string) error
}
