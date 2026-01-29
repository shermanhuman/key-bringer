# Tasks

## Planning

- [x] Research & Design
- [x] Security Model
- [x] Create `.agents/Go.md`
- [x] Create `.plan/setup-guide.md`
- [x] Define Testing Strategy

## Implementation

### 1. Scaffold

- [x] `go mod init`
- [x] Directory structure
- [x] `.env.example`
- [x] **Test**: `go build ./...` compiles ✅

### 2. Core Interfaces

- [x] `internal/core/notifier.go`
- [x] `internal/core/secretstore.go`
- [x] `internal/core/verifier.go`
- [x] `internal/core/unlocker.go`
- [x] **Test**: `go build ./...` compiles ✅

### 3. TOTP Verifier

- [x] `internal/totp/verifier.go`
- [x] `internal/totp/verifier_test.go`
- [x] **Test**: `go test ./internal/totp/...` passes ✅

### 4. GSM Client

- [x] `internal/gsm/client.go`
- [x] `internal/gsm/client_test.go`
- [x] **Test**: `go test ./internal/gsm/...` passes ✅

### 5. Telnyx Client

- [x] `internal/telnyx/client.go`
- [x] `internal/telnyx/webhook.go`
- [x] `internal/telnyx/webhook_test.go`
- [x] **Test**: `go test ./internal/telnyx/...` passes ✅

### 6. HTTP Server

- [x] `internal/server/router.go`
- [x] `internal/server/middleware.go`
- [x] `internal/server/handlers.go`
- [x] `internal/server/handlers_test.go`
- [x] **Test**: `go test ./internal/server/...` passes ✅

### 7. key-seeker CLI

- [x] `cmd/key-seeker/main.go`
- [x] CLI flags (`--totp`, `--monitor`)
- [ ] **Test**: Manual `--totp` mode works (requires Linux)

### 8. ZFS Unlocker

- [x] Integrated in `cmd/key-seeker/main.go`
- [ ] **Test**: Unlock testpool on Linux

### 9. Deploy Artifacts

- [x] `deploy/Dockerfile` (key-bringer)
- [x] `deploy/Dockerfile.seeker` (Debian Trixie)
- [x] `deploy/cloudbuild.yaml`
- [x] `systemd/key-seeker.service`
- [x] `systemd/key-seeker.env.example`
- [x] `scripts/build-seeker.sh`
- [x] `.plan/host-install.md`
- [x] `README.md`

### 10. Production Deployment

- [ ] Install gcloud CLI
- [ ] Deploy to Cloud Run
- [ ] Configure Telnyx webhook URL
- [ ] Install key-seeker on Debian host
- [ ] **Test**: Full SMS flow on real phone
