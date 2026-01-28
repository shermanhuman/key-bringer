package telnyx

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

const (
	// SignatureHeader is the HTTP header containing the ED25519 signature.
	SignatureHeader = "Telnyx-Signature-Ed25519"
	// TimestampHeader is the HTTP header containing the Unix timestamp.
	TimestampHeader = "Telnyx-Timestamp"
	// MaxTimestampAge is the maximum age of a webhook before it's rejected.
	MaxTimestampAge = 5 * time.Minute
)

// WebhookVerifier validates Telnyx webhook signatures.
type WebhookVerifier struct {
	publicKey ed25519.PublicKey
}

// NewWebhookVerifier creates a verifier with the given Base64-encoded public key.
func NewWebhookVerifier(publicKeyBase64 string) (*WebhookVerifier, error) {
	keyBytes, err := base64.StdEncoding.DecodeString(publicKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("telnyx: invalid public key encoding: %w", err)
	}

	if len(keyBytes) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("telnyx: invalid public key size: got %d, want %d", len(keyBytes), ed25519.PublicKeySize)
	}

	return &WebhookVerifier{
		publicKey: ed25519.PublicKey(keyBytes),
	}, nil
}

// Verify checks if the webhook request has a valid signature.
func (v *WebhookVerifier) Verify(r *http.Request, body []byte) error {
	signature := r.Header.Get(SignatureHeader)
	if signature == "" {
		return fmt.Errorf("telnyx: missing %s header", SignatureHeader)
	}

	timestampStr := r.Header.Get(TimestampHeader)
	if timestampStr == "" {
		return fmt.Errorf("telnyx: missing %s header", TimestampHeader)
	}

	// Check timestamp age
	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return fmt.Errorf("telnyx: invalid timestamp: %w", err)
	}

	webhookTime := time.Unix(timestamp, 0)
	if time.Since(webhookTime) > MaxTimestampAge {
		return fmt.Errorf("telnyx: webhook timestamp too old (age: %v)", time.Since(webhookTime))
	}

	// Decode signature
	sigBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("telnyx: invalid signature encoding: %w", err)
	}

	// Build the signed payload: timestamp.payload
	signedPayload := []byte(timestampStr + "." + string(body))

	// Verify signature
	if !ed25519.Verify(v.publicKey, signedPayload, sigBytes) {
		return fmt.Errorf("telnyx: invalid signature")
	}

	return nil
}

// IsTimestampValid checks if a Unix timestamp is within the allowed age.
func IsTimestampValid(timestamp int64, maxAge time.Duration) bool {
	webhookTime := time.Unix(timestamp, 0)
	return time.Since(webhookTime) <= maxAge
}
