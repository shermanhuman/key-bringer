package gsm

import (
	"context"
	"fmt"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
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

// GetSecret retrieves the latest version of a secret by name.
func (c *Client) GetSecret(ctx context.Context, name string) (string, error) {
	// Build the resource name
	resourceName := fmt.Sprintf("projects/%s/secrets/%s/versions/latest", c.projectID, name)

	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: resourceName,
	}

	result, err := c.client.AccessSecretVersion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("gsm: failed to access secret %q: %w", name, err)
	}

	return string(result.Payload.Data), nil
}

// Close closes the underlying gRPC connection.
func (c *Client) Close() error {
	return c.client.Close()
}
