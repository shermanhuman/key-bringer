package gcp

import (
	"context"
	"os"
	"strconv"
	"testing"

	"github.com/Applesauce-Labs/key-bringer/internal/core"
)

func TestFetchSecret(t *testing.T) {
	projectID := os.Getenv("GCP_PROJECT")
	if projectID == "" {
		t.Skip("GCP_PROJECT not set, skipping integration test")
	}
	secretID := os.Getenv("GCP_TEST_SECRET_ID")
	if secretID == "" {
		t.Skip("GCP_TEST_SECRET_ID not set, skipping integration test")
	}
	versionStr := os.Getenv("GCP_TEST_SECRET_VERSION")
	if versionStr == "" {
		t.Skip("GCP_TEST_SECRET_VERSION not set, skipping integration test")
	}
	version, err := strconv.Atoi(versionStr)
	if err != nil || version <= 0 {
		t.Fatalf("invalid GCP_TEST_SECRET_VERSION")
	}

	ctx := context.Background()
	client, err := NewClient(ctx, projectID)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	secret, err := client.GetSecret(ctx, core.SecretRef{SecretID: secretID, Version: version})
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

	_, err = client.GetSecret(ctx, core.SecretRef{SecretID: "nonexistent-secret-that-should-not-exist-12345", Version: 1})
	if err == nil {
		t.Error("expected error for nonexistent secret, got nil")
	}
}
