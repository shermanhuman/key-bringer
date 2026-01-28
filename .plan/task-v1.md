# Tasks

## Planning

- [x] Research & Design
- [x] Security Model
- [x] Create `.agents/Go.md`
- [x] Create `.plan/setup-guide.md`
- [x] Define Testing Strategy

## Implementation

### 1. Scaffold

- [ ] `go mod init`
- [ ] Directory structure
- [ ] `.env.example`
- [ ] **Test**: `go build ./...` compiles

### 2. Core Interfaces

- [ ] `internal/core/notifier.go`
- [ ] `internal/core/secretstore.go`
- [ ] `internal/core/verifier.go`
- [ ] `internal/core/unlocker.go`
- [ ] **Test**: `go build ./...` compiles

### 3. TOTP Verifier

- [ ] `internal/totp/verifier.go`
- [ ] `internal/totp/verifier_test.go`
- [ ] **Test**: `go test ./internal/totp/...` passes

### 4. GSM Client

- [ ] `internal/gsm/client.go`
- [ ] `internal/gsm/client_test.go`
- [ ] **Test**: `go test ./internal/gsm/...` passes

### 5. Telnyx Client

- [ ] `internal/telnyx/client.go`
- [ ] `internal/telnyx/webhook.go`
- [ ] `internal/telnyx/webhook_test.go`
- [ ] **Test**: `go test ./internal/telnyx/...` passes

### 6. HTTP Server

- [ ] `internal/server/router.go`
- [ ] `internal/server/middleware.go`
- [ ] `internal/server/handlers.go`
- [ ] `internal/server/handlers_test.go`
- [ ] **Test**: `go test ./internal/server/...` passes

### 7. key-seeker CLI

- [ ] `cmd/key-seeker/main.go`
- [ ] CLI flags (`--totp`, `--monitor`)
- [ ] **Test**: Manual `--totp` mode works

### 8. ZFS Unlocker

- [ ] `internal/zfs/unlocker.go`
- [ ] **Test**: Unlock testpool on Linux

### 9. Integration

- [ ] Systemd unit
- [ ] Dockerfile
- [ ] Cloud Run deploy
- [ ] **Test**: Full SMS flow on dev phone
