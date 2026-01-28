package core

import "context"

// SecretStore handles fetching secrets from a secure backend.
type SecretStore interface {
	// GetSecret retrieves a secret by name.
	GetSecret(ctx context.Context, name string) (string, error)
}
