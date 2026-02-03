package config

import "testing"

func TestLoadRejectsUnknownFields(t *testing.T) {
	yaml := []byte(`version: 1
unknownTop: true
`)

	cfg, err := loadBytes(yaml)
	if err == nil {
		t.Fatalf("expected error, got cfg=%v", cfg)
	}
}

func TestLoadRejectsBadSecretRef(t *testing.T) {
	yaml := []byte(`version: 1

gcp:
  projectId: p

runtime:
  requireIamAuth: true
  maxPendingMinutes: 10
  iamAudience: https://example

http:
  publicURL: https://example

telnyx:
  messagingProfileId: mp

secrets:
  zfsMasterKey: { secretId: zfs-master-key, version: 0 }
  totpSeed: { secretId: totp-seed, version: 1 }
  telnyxApiKey: { secretId: telnyx-api-key, version: 1 }
  telnyxFromNumber: { secretId: telnyx-from-number, version: 1 }
  telnyxPublicKey: { secretId: telnyx-public-key, version: 1 }
  adminPhone: { secretId: admin-phone, version: 1 }

machines: [ny1]
`)

	_, err := loadBytes(yaml)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func loadBytes(b []byte) (*Config, error) {
	dec := newDecoder(b)
	var cfg Config
	if err := dec.Decode(&cfg); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}
