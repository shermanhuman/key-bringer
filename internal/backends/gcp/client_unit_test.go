package gcp

import (
	"testing"

	"github.com/Applesauce-Labs/key-bringer/internal/core"
)

func TestSecretVersionName(t *testing.T) {
	got, err := secretVersionName("proj", core.SecretRef{SecretID: "my-secret", Version: 12})
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	want := "projects/proj/secrets/my-secret/versions/12"
	if got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}

func TestSecretVersionNameRejectsEmptyProject(t *testing.T) {
	_, err := secretVersionName("", core.SecretRef{SecretID: "s", Version: 1})
	if err == nil {
		t.Fatalf("expected error")
	}
}
