package gcp

import (
	"context"
	"os"
	"testing"
)

func TestFetchSecret(t *testing.T) {
	projectID := os.Getenv("GCP_PROJECT")
	if projectID == "" {
		t.Skip("GCP_PROJECT not set, skipping integration test")
	}

	ctx := context.Background()
	client, err := NewClient(ctx, projectID)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	// This test requires a secret named "test-secret" to exist in your GCP project
	// Create it with: echo -n "test-value" | gcloud secrets create test-secret --data-file=-
	secret, err := client.GetSecret(ctx, "test-secret")
	if err != nil {
		t.Fatalf("failed to get secret: %v", err)
	}

	if secret == "" {
		t.Error("expected non-empty secret value")
	}

	t.Logf("Successfully retrieved secret (length: %d)", len(secret))
}

func TestSecretNotFound(t *testing.T) {
	projectID := os.Getenv("GCP_PROJECT")
	if projectID == "" {
		t.Skip("GCP_PROJECT not set, skipping integration test")
	}

	ctx := context.Background()
	client, err := NewClient(ctx, projectID)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	_, err = client.GetSecret(ctx, "nonexistent-secret-that-should-not-exist-12345")
	if err == nil {
		t.Error("expected error for nonexistent secret, got nil")
	}
}
