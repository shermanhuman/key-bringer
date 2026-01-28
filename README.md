# Key Bringer

Secure, automated ZFS unlocking via 2-way SMS and TOTP verification.

## Overview

Key Bringer is a serverless solution for unlocking ZFS encrypted volumes at boot time. It consists of:

- **key-bringer**: Cloud Run service that holds secrets and handles SMS verification
- **key-seeker**: Host agent that requests keys and unlocks ZFS

## Architecture

```
┌─────────────────┐     HTTPS      ┌──────────────────┐
│  Debian Server  │◄──────────────►│   Cloud Run      │
│  (key-seeker)   │                │   (key-bringer)  │
└────────┬────────┘                └────────┬─────────┘
         │                                  │
         │                                  │ SMS
         │                                  ▼
         │                         ┌────────────────┐
         │                         │    Telnyx      │
         │                         └───────┬────────┘
         │                                 │
         ▼                                 ▼
   ZFS Encrypted                    Admin Phone
     Volumes                     (TOTP Authenticator)
```

## Quick Start

### 1. Deploy key-bringer to Cloud Run

See [.plan/setup-guide.md](.plan/setup-guide.md) for GCP and Telnyx setup.

```bash
gcloud builds submit --config deploy/cloudbuild.yaml
```

### 2. Install key-seeker on Debian Host

See [.plan/host-install.md](.plan/host-install.md) for detailed instructions.

```bash
# Copy binary
scp bin/key-seeker-linux-amd64 server:/usr/local/bin/key-seeker

# Configure
ssh server
sudo mkdir -p /etc/key-seeker
sudo nano /etc/key-seeker/env

# Install service
sudo cp systemd/key-seeker.service /etc/systemd/system/
sudo systemctl enable key-seeker
```

### 3. Test

```bash
# Manual TOTP unlock
key-seeker --totp 123456

# Or via systemd (sends SMS)
sudo systemctl start key-seeker
```

## Development

```bash
# Run tests
go test ./...

# Build
go build -o bin/key-bringer ./cmd/key-bringer
go build -o bin/key-seeker ./cmd/key-seeker

# Local server
cp .env.example .env
# Edit .env with test values
go run ./cmd/key-bringer
```

## License

MIT
