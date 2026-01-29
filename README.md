# Key Bringer

Secure, automated ZFS unlocking via 2-way SMS and TOTP verification.

## 1. Google Cloud Setup

### Create Project & Secrets

Create a project `key-bringer-prod` and enable **Secret Manager**, **Cloud Run**, and **Cloud Build** APIs.

Create these 7 secrets in **Secret Manager**:

| Secret Name          | Value to Store                                  |
| -------------------- | ----------------------------------------------- |
| `zfs-master-key`     | Your actual ZFS encryption passphrase           |
| `agent-secret`       | A long random password (shared with host agent) |
| `totp-seed`          | Base32 TOTP seed (e.g. `JBSWY3DPEHPK3PXP`)      |
| `telnyx-api-key`     | Telnyx V2 API Key (starts with `KEY...`)        |
| `telnyx-from-number` | Your purchased Telnyx number (E.164: `+1...`)   |
| `telnyx-public-key`  | Telnyx Ed25519 Public Key (for verification)    |
| `admin-phone`        | Your personal mobile number (E.164: `+1...`)    |

### Service Account

Create a service account `key-bringer-sa` and grant it the **Secret Manager Secret Accessor** role.

## 2. Telnyx Setup

1.  **Buy a Number**: Ensure it has SMS capabilities.
2.  **Create Messaging Profile**:
    - **Inbound**: Set Webhook URL to `https://<YOUR-CLOUD-RUN-URL>/webhooks/telnyx` (after deployment).
    - **Allowed Destinations**: Check your country (e.g., United States).
3.  **Associate Number**: Link your purchased number to this profile.

## 3. Deploy Server

```bash
gcloud builds submit --config deploy/cloudbuild.yaml
```

_Note the Service URL from the output and update your Telnyx Webhook._

## 4. Install Host Agent (Debian)

### Build

```bash
# Cross-compile for Linux (run on local)
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/key-seeker ./cmd/key-seeker
scp bin/key-seeker root@your-server:/usr/local/bin/
```

### Configure

On the server, create `/etc/key-seeker/env`:

```ini
SERVER_URL=https://<YOUR-CLOUD-RUN-URL>
MACHINE_ID=ny1
AGENT_SECRET=<value-from-gcp-secret-manager>
ZFS_DATASET=zroot/encrypted
```

_Permissions: `chmod 600 /etc/key-seeker/env`_

### Enable Service

Copy `systemd/key-seeker.service` to `/etc/systemd/system/` and run:

```bash
systemctl daemon-reload
systemctl enable key-seeker
```

## 5. Usage

**Boot Time**: The service starts automatically, sends an SMS to `admin-phone`. Reply with your 6-digit TOTP code to unlock.

**Manual Unlock**:

```bash
key-seeker --totp 123456
```

## Development

```bash
go test ./...
go build ./...
```

## License

MIT
