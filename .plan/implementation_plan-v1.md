# Implementation Plan - Key Bringer

Secure, automated ZFS unlocking via **2-Way SMS** and **Manual CLI Fallback**.

## Design Goals

1.  **SMS-Only UX**: Reply to SMS with TOTP code (no web UI).
2.  **Manual Fallback**: `key-seeker unlock --totp <code>` bypasses SMS.
3.  **Extensible**: Interfaces for Notifier (Telnyx/Twilio) and UnlockTarget (ZFS/LUKS).

---

## Environment Variables

| Variable             | Description                           |
| -------------------- | ------------------------------------- |
| `AGENT_SECRET`       | Shared secret for key-seeker auth     |
| `TOTP_SEED`          | Base32 seed for TOTP validation       |
| `ZFS_KEY`            | ZFS encryption passphrase             |
| `TELNYX_API_KEY`     | Telnyx API key                        |
| `TELNYX_FROM_NUMBER` | Outbound SMS number                   |
| `TELNYX_PUBLIC_KEY`  | ED25519 public key for webhook verify |
| `ADMIN_PHONE`        | Phone number for SMS challenges       |

---

## Implementation Steps with Testing Gates

### 1. Scaffold Project

**Goal**: Establish project structure and dependencies.

```bash
go mod init github.com/yourusername/key-bringer
```

**Test Gate**: `go build ./...` succeeds with no errors.

---

### 2. Core Interfaces

**Goal**: Define contracts so implementations can be swapped.

```go
// internal/core/notifier.go
type Notifier interface {
    SendSMS(ctx context.Context, to, message string) error
}

// internal/core/secretstore.go
type SecretStore interface {
    GetSecret(ctx context.Context, name string) (string, error)
}

// internal/core/verifier.go
type Verifier interface {
    Validate(code string) bool
}

// internal/core/unlocker.go
type Unlocker interface {
    ApplyKey(secret string) error
}
```

**Test Gate**:

- [ ] `go build ./...` compiles
- [ ] Interfaces exist in `internal/core/`
- [ ] No implementations yet (interfaces only)

---

### 3. TOTP Verifier

**Goal**: Validate 6-digit codes from Google Authenticator.

**What must work**:

- Accept valid codes within 30s window
- Reject expired codes
- Reject wrong codes

**Unit Tests** (`internal/totp/verifier_test.go`):

```go
func TestValidCode(t *testing.T) {
    seed := "JBSWY3DPEHPK3PXP"
    v := NewVerifier(seed)
    code, _ := totp.GenerateCode(seed, time.Now())
    assert.True(t, v.Validate(code))
}

func TestInvalidCode(t *testing.T) {
    v := NewVerifier("JBSWY3DPEHPK3PXP")
    assert.False(t, v.Validate("000000"))
}

func TestExpiredCode(t *testing.T) {
    seed := "JBSWY3DPEHPK3PXP"
    v := NewVerifier(seed)
    oldCode, _ := totp.GenerateCode(seed, time.Now().Add(-2*time.Minute))
    assert.False(t, v.Validate(oldCode))
}
```

**Test Gate**: `go test ./internal/totp/...` passes.

---

### 4. GSM Client

**Goal**: Fetch secrets from Google Secret Manager.

**What must work**:

- Authenticate to GCP (Application Default Credentials)
- Fetch secret by name
- Return error if secret doesn't exist

**Integration Test** (`internal/gsm/client_test.go`):

```go
func TestFetchSecret(t *testing.T) {
    if os.Getenv("GCP_PROJECT") == "" {
        t.Skip("GCP_PROJECT not set")
    }
    c := NewClient(os.Getenv("GCP_PROJECT"))
    secret, err := c.GetSecret(context.Background(), "test-secret")
    assert.NoError(t, err)
    assert.NotEmpty(t, secret)
}

func TestSecretNotFound(t *testing.T) {
    c := NewClient(os.Getenv("GCP_PROJECT"))
    _, err := c.GetSecret(context.Background(), "nonexistent-secret")
    assert.Error(t, err)
}
```

**Test Gate**:

- Create a test secret in GCP: `echo -n "test" | gcloud secrets create test-secret --data-file=-`
- `GCP_PROJECT=your-project go test ./internal/gsm/...` passes

---

### 5. Telnyx Client

**Goal**: Send SMS and verify incoming webhook signatures.

**What must work**:

- SendSMS delivers to Telnyx API
- Webhook signature verification accepts valid signatures
- Webhook signature verification rejects tampered payloads
- Timestamp validation rejects old webhooks

**Unit Tests** (`internal/telnyx/webhook_test.go`):

```go
func TestValidSignature(t *testing.T) {
    // Use known test vector
    publicKey := "..." // Telnyx public key
    signature := "..." // Valid signature
    timestamp := "..."
    payload := "..."

    assert.True(t, VerifySignature(publicKey, signature, timestamp, payload))
}

func TestTamperedPayload(t *testing.T) {
    // Same signature but modified payload
    assert.False(t, VerifySignature(publicKey, signature, timestamp, "tampered"))
}

func TestOldTimestamp(t *testing.T) {
    oldTimestamp := time.Now().Add(-10 * time.Minute).Unix()
    assert.False(t, IsTimestampValid(oldTimestamp, 5*time.Minute))
}
```

**Integration Test** (`internal/telnyx/client_test.go`):

```go
func TestSendSMS(t *testing.T) {
    if os.Getenv("TELNYX_API_KEY") == "" {
        t.Skip("TELNYX_API_KEY not set")
    }
    c := NewClient(os.Getenv("TELNYX_API_KEY"), os.Getenv("TELNYX_FROM_NUMBER"))
    err := c.SendSMS(context.Background(), os.Getenv("TEST_PHONE"), "Test message")
    assert.NoError(t, err)
}
```

**Test Gate**:

- Unit tests: `go test ./internal/telnyx/...` passes
- Integration: Receive test SMS on your phone

---

### 6. HTTP Server + Middleware

**Goal**: API endpoints with authentication.

**What must work**:

- `/api/v1/unlock` requires `X-Agent-Secret` header
- Missing header → 401
- Invalid header → 401
- Valid TOTP → 200 with secret
- Missing TOTP → 202 (SMS sent)
- Webhook endpoint accepts POST

**HTTP Tests** (`internal/server/handlers_test.go`):

```go
func TestUnlockWithoutAuth(t *testing.T) {
    router := setupTestRouter()
    req := httptest.NewRequest("POST", "/api/v1/unlock", nil)
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)
    assert.Equal(t, 401, w.Code)
}

func TestUnlockWithInvalidAuth(t *testing.T) {
    router := setupTestRouter()
    req := httptest.NewRequest("POST", "/api/v1/unlock", nil)
    req.Header.Set("X-Agent-Secret", "wrong-secret")
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)
    assert.Equal(t, 401, w.Code)
}

func TestUnlockWithValidTOTP(t *testing.T) {
    router := setupTestRouter()
    body := `{"machine_id":"test","totp_code":"<VALID>"}`
    req := httptest.NewRequest("POST", "/api/v1/unlock", strings.NewReader(body))
    req.Header.Set("X-Agent-Secret", "test-secret")
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)
    assert.Equal(t, 200, w.Code)
}

func TestUnlockWithoutTOTP(t *testing.T) {
    router := setupTestRouter() // with mock Notifier
    body := `{"machine_id":"test"}`
    req := httptest.NewRequest("POST", "/api/v1/unlock", strings.NewReader(body))
    req.Header.Set("X-Agent-Secret", "test-secret")
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)
    assert.Equal(t, 202, w.Code)
}
```

**Test Gate**: `go test ./internal/server/...` passes.

---

### 7. key-seeker CLI

**Goal**: CLI that polls server and calls ZFS.

**What must work**:

- `--totp` flag sends TOTP directly
- `--monitor` mode polls until approved
- Received secret is passed to Unlocker

**Integration Test** (manual):

```bash
# Start key-bringer locally
./bin/key-bringer &

# Test manual mode
./bin/key-seeker unlock --totp $(oathtool --totp -b $TOTP_SEED)
# Expected: "Key applied successfully"

# Test monitor mode (need to reply to SMS or simulate webhook)
./bin/key-seeker --monitor &
curl -X POST localhost:8080/webhooks/telnyx -d '{"from":"+15551234567","body":"123456"}'
# Expected: key-seeker logs "Key applied"
```

**Test Gate**: Both manual and monitor modes complete successfully.

---

### 8. ZFS Unlocker

**Goal**: Apply key to encrypted ZFS dataset.

**E2E Test** (on Linux with ZFS):

```bash
# Setup test pool
sudo truncate -s 1G /tmp/zfs-test.img
sudo zpool create testpool /tmp/zfs-test.img
sudo zfs create -o encryption=on -o keyformat=passphrase testpool/encrypted
# Passphrase: test-passphrase

# Lock it
sudo zfs unmount testpool/encrypted
sudo zfs unload-key testpool/encrypted

# Run unlock
echo "test-passphrase" | sudo ./bin/key-seeker unlock --stdin

# Verify
sudo zfs get keystatus testpool/encrypted
# Expected: keystatus available

# Cleanup
sudo zpool destroy testpool
rm /tmp/zfs-test.img
```

**Test Gate**: ZFS dataset unlocks and mounts.

---

## Summary: Test Gates

| Step               | Test Command                    | Pass Criteria       |
| ------------------ | ------------------------------- | ------------------- |
| 1. Scaffold        | `go build ./...`                | Compiles            |
| 2. Core Interfaces | `go build ./...`                | Compiles            |
| 3. TOTP Verifier   | `go test ./internal/totp/...`   | All pass            |
| 4. GSM Client      | `go test ./internal/gsm/...`    | All pass            |
| 5. Telnyx Client   | `go test ./internal/telnyx/...` | All pass            |
| 6. HTTP Server     | `go test ./internal/server/...` | All pass            |
| 7. key-seeker CLI  | Manual: `--totp` mode           | Key applied         |
| 8. ZFS Unlocker    | Manual: testpool unlock         | keystatus=available |
