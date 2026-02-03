package gcp

import (
	"context"
	"fmt"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/Applesauce-Labs/key-bringer/internal/core"
)

// Client implements core.SecretStore using Google Secret Manager.
type Client struct {
	projectID string
	client    *secretmanager.Client
}

// NewClient creates a new GSM client for the given GCP project.
// Uses Application Default Credentials (ADC) for authentication.
func NewClient(ctx context.Context, projectID string) (*Client, error) {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("gsm: failed to create client: %w", err)
	}
	return &Client{
		projectID: projectID,
		client:    client,
	}, nil
}

// GetSecret retrieves a pinned numeric version of a secret.
func (c *Client) GetSecret(ctx context.Context, ref core.SecretRef) (string, error) {
	if err := ref.Validate(); err != nil {
		return "", fmt.Errorf("gsm: invalid secret ref: %w", err)
	}

	resourceName, err := secretVersionName(c.projectID, ref)
	if err != nil {
		return "", err
	}

	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: resourceName,
	}

	result, err := c.client.AccessSecretVersion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("gsm: failed to access secret %q v%d: %w", ref.SecretID, ref.Version, err)
	}

	return string(result.Payload.Data), nil
}

func secretVersionName(projectID string, ref core.SecretRef) (string, error) {
	if err := ref.Validate(); err != nil {
		return "", fmt.Errorf("gsm: invalid secret ref: %w", err)
	}
	if projectID == "" {
		return "", fmt.Errorf("gsm: projectID is required")
	}
	return fmt.Sprintf("projects/%s/secrets/%s/versions/%d", projectID, ref.SecretID, ref.Version), nil
}

// Close closes the underlying gRPC connection.
func (c *Client) Close() error {
	return c.client.Close()
}
