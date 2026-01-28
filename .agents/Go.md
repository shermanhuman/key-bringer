---
description: Go development guidelines for key-bringer project
---

# Go Development Guidelines

## Stack

- **Go**: 1.25.6 (latest stable, Jan 2026)
- **HTTP**: `github.com/gin-gonic/gin` v1.10+
- **TOTP**: `github.com/pquerna/otp` v1.4+
- **GCP**: `cloud.google.com/go/secretmanager` v1.14+
- **Telnyx**: `github.com/team-telnyx/telnyx-go` v3+
- **Env**: `github.com/joho/godotenv` v1.5+

## Local Development

Use `.env` for local environment variables (loaded by `godotenv`):

```bash
# Copy template and fill values
cp .env.example .env
```

`.env.example` is committed; `.env` is gitignored.

## Project Structure

```
cmd/           # Entrypoints (minimal, wiring only)
internal/      # Private packages (enforced by Go)
  core/        # Interfaces (domain contracts)
  <impl>/      # Implementations by provider
deploy/        # Dockerfile, cloudbuild.yaml
```

## Idioms

- Keep `cmd/*/main.go` thin: wire dependencies, call `server.Run()`
- Define interfaces in `internal/core/`, implementations elsewhere
- Use `context.Context` for all I/O-bound operations
- Return `error` as last return value; wrap with `fmt.Errorf("context: %w", err)`
- Use `slog` (stdlib) for structured logging

## Testing

- Unit tests: `*_test.go` alongside source
- Use interfaces to inject mocks
- Table-driven tests preferred

## Security

- Never log secrets
- Validate all inputs
- Use `crypto/ed25519` for Telnyx webhook signature verification
- Check timestamp to prevent replay attacks (reject > 5 min old)

## Commands

```bash
# Run tests
go test ./...

# Build binaries
go build -o bin/key-bringer ./cmd/key-bringer
go build -o bin/key-seeker ./cmd/key-seeker

# Lint
golangci-lint run
```

## Workflow

After completing each task:

1. Run `go build ./...` to verify compilation
2. Run `go test ./...` to verify tests pass
3. Generate a commit message using conventional commits format:
   - `feat:` for new features
   - `fix:` for bug fixes
   - `test:` for adding tests
   - `docs:` for documentation
   - `refactor:` for refactoring

Example: `feat(totp): implement TOTP verifier with RFC 6238 compliance`
