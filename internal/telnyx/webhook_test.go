package telnyx

import (
	"crypto/ed25519"
	"encoding/base64"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

// generateTestKeys creates a new ED25519 key pair for testing.
func generateTestKeys() (ed25519.PublicKey, ed25519.PrivateKey) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	return pub, priv
}

func TestValidSignature(t *testing.T) {
	pub, priv := generateTestKeys()
	pubBase64 := base64.StdEncoding.EncodeToString(pub)

	verifier, err := NewWebhookVerifier(pubBase64)
	if err != nil {
		t.Fatalf("failed to create verifier: %v", err)
	}

	// Create a valid signed payload
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	body := []byte(`{"data":{"payload":{"from":"+15551234567","text":"123456"}}}`)
	signedPayload := []byte(timestamp + "." + string(body))
	signature := ed25519.Sign(priv, signedPayload)
	sigBase64 := base64.StdEncoding.EncodeToString(signature)

	// Create request
	req := httptest.NewRequest("POST", "/webhooks/telnyx", nil)
	req.Header.Set(SignatureHeader, sigBase64)
	req.Header.Set(TimestampHeader, timestamp)

	err = verifier.Verify(req, body)
	if err != nil {
		t.Errorf("expected valid signature to pass: %v", err)
	}
}

func TestTamperedPayload(t *testing.T) {
	pub, priv := generateTestKeys()
	pubBase64 := base64.StdEncoding.EncodeToString(pub)

	verifier, err := NewWebhookVerifier(pubBase64)
	if err != nil {
		t.Fatalf("failed to create verifier: %v", err)
	}

	// Sign original payload
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	originalBody := []byte(`{"data":"original"}`)
	signedPayload := []byte(timestamp + "." + string(originalBody))
	signature := ed25519.Sign(priv, signedPayload)
	sigBase64 := base64.StdEncoding.EncodeToString(signature)

	// But verify with tampered body
	tamperedBody := []byte(`{"data":"tampered"}`)

	req := httptest.NewRequest("POST", "/webhooks/telnyx", nil)
	req.Header.Set(SignatureHeader, sigBase64)
	req.Header.Set(TimestampHeader, timestamp)

	err = verifier.Verify(req, tamperedBody)
	if err == nil {
		t.Error("expected tampered payload to fail verification")
	}
}

func TestOldTimestamp(t *testing.T) {
	pub, priv := generateTestKeys()
	pubBase64 := base64.StdEncoding.EncodeToString(pub)

	verifier, err := NewWebhookVerifier(pubBase64)
	if err != nil {
		t.Fatalf("failed to create verifier: %v", err)
	}

	// Use timestamp from 10 minutes ago
	oldTime := time.Now().Add(-10 * time.Minute)
	timestamp := strconv.FormatInt(oldTime.Unix(), 10)
	body := []byte(`{"data":"test"}`)
	signedPayload := []byte(timestamp + "." + string(body))
	signature := ed25519.Sign(priv, signedPayload)
	sigBase64 := base64.StdEncoding.EncodeToString(signature)

	req := httptest.NewRequest("POST", "/webhooks/telnyx", nil)
	req.Header.Set(SignatureHeader, sigBase64)
	req.Header.Set(TimestampHeader, timestamp)

	err = verifier.Verify(req, body)
	if err == nil {
		t.Error("expected old timestamp to fail verification")
	}
}

func TestMissingHeaders(t *testing.T) {
	pub, _ := generateTestKeys()
	pubBase64 := base64.StdEncoding.EncodeToString(pub)

	verifier, err := NewWebhookVerifier(pubBase64)
	if err != nil {
		t.Fatalf("failed to create verifier: %v", err)
	}

	body := []byte(`{"data":"test"}`)

	// Missing signature header
	req1 := httptest.NewRequest("POST", "/webhooks/telnyx", nil)
	req1.Header.Set(TimestampHeader, strconv.FormatInt(time.Now().Unix(), 10))
	if verifier.Verify(req1, body) == nil {
		t.Error("expected missing signature header to fail")
	}

	// Missing timestamp header
	req2 := httptest.NewRequest("POST", "/webhooks/telnyx", nil)
	req2.Header.Set(SignatureHeader, "somesig")
	if verifier.Verify(req2, body) == nil {
		t.Error("expected missing timestamp header to fail")
	}
}

func TestIsTimestampValid(t *testing.T) {
	// Fresh timestamp
	if !IsTimestampValid(time.Now().Unix(), 5*time.Minute) {
		t.Error("expected fresh timestamp to be valid")
	}

	// Old timestamp
	oldTime := time.Now().Add(-10 * time.Minute).Unix()
	if IsTimestampValid(oldTime, 5*time.Minute) {
		t.Error("expected old timestamp to be invalid")
	}
}
