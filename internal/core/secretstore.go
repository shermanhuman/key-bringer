package core

import "context"

// SecretRef is a canonical reference to a Secret Manager secret version.
// Version must be a pinned positive integer; aliases like "latest" are forbidden.
type SecretRef struct {
	SecretID string `json:"secretId" yaml:"secretId"`
	Version  int    `json:"version" yaml:"version"`
}

func (r SecretRef) Validate() error {
	if r.SecretID == "" {
		return ErrInvalidSecretRef("secretId is required")
	}
	if r.Version <= 0 {
		return ErrInvalidSecretRef("version must be a positive integer")
	}
	return nil
}

type ErrInvalidSecretRef string

func (e ErrInvalidSecretRef) Error() string { return string(e) }

// SecretStore handles fetching secrets from a secure backend.
type SecretStore interface {
	// GetSecret retrieves a secret by pinned numeric version.
	GetSecret(ctx context.Context, ref SecretRef) (string, error)
}
