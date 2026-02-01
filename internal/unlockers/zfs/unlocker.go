package zfs

import (
	"bytes"
	"fmt"
	"os/exec"
)

// Unlocker implements core.Unlocker for ZFS encrypted datasets.
type Unlocker struct{}

// NewUnlocker creates a new ZFS unlocker.
func NewUnlocker() *Unlocker {
	return &Unlocker{}
}

// ApplyKey loads the encryption key for a ZFS dataset and mounts it.
func (u *Unlocker) ApplyKey(dataset, secret string) error {
	if dataset == "" {
		return fmt.Errorf("zfs: dataset name required")
	}

	// Run: echo "$secret" | zfs load-key $dataset
	cmd := exec.Command("zfs", "load-key", dataset)
	cmd.Stdin = bytes.NewBufferString(secret + "\n")

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zfs load-key failed: %w (output: %s)", err, string(output))
	}

	// Mount the dataset
	mountCmd := exec.Command("zfs", "mount", dataset)
	_ = mountCmd.Run() // Ignore mount errors (may already be mounted)

	return nil
}
