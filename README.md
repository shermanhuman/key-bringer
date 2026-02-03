# KeyBringer

Secure, automated ZFS encryption at rest with secure offsite passphrase storage in Google Cloud Secret Manager and 2-way SMS + TOTP verification. For those of us who don't want to get up at 3AM to ssh in.

## Overview

KeyBringer is a serverless solution for unlocking ZFS encrypted volumes at boot time. It consists of:

- **key-bringer**: Cloud Run service that holds secrets and handles SMS verification
- **key-seeker**: Host agent that requests keys and unlocks ZFS
- **totp-gen**: Utility to generate TOTP secrets with scannable QR codes

## Architecture

```
┌─────────────────┐     HTTPS      ┌──────────────────┐
│  Debian Server  │◄──────────────►│   Cloud Run      │
│  (key-seeker)   │                │   (key-bringer)  │
└────────┬────────┘                └────────┬─────────┘
         │                                  │ SMS
         ▼                                  ▼
   ZFS Encrypted                      Admin Phone
     Volumes                       (Authenticator App)
```

---

## 1. Google Cloud Setup

### Create Project

```bash
gcloud projects create key-bringer --name="KeyBringer"
gcloud config set project key-bringer
```

### Link Billing

In the [Cloud Console](https://console.cloud.google.com/), link a billing account to the project.

### Enable APIs

```bash
gcloud services enable \
  secretmanager.googleapis.com \
  run.googleapis.com \
  cloudbuild.googleapis.com \
  artifactregistry.googleapis.com
```

### Create Artifact Registry Repository

```bash
gcloud artifacts repositories create key-bringer \
  --repository-format=docker \
  --location=us-central1
```

### Create Service Account

```bash
gcloud iam service-accounts create key-bringer-sa \
  --display-name="KeyBringer Service Account"

gcloud projects add-iam-policy-binding key-bringer \
  --member="serviceAccount:key-bringer-sa@key-bringer.iam.gserviceaccount.com" \
  --role="roles/secretmanager.secretAccessor"
```

### Grant Cloud Build Permissions

Replace `PROJECT_NUMBER` with your project number (find via `gcloud projects describe key-bringer --format="value(projectNumber)"`):

```bash
# Artifact Registry
gcloud projects add-iam-policy-binding key-bringer \
  --member="serviceAccount:PROJECT_NUMBER-compute@developer.gserviceaccount.com" \
  --role="roles/artifactregistry.writer"

# Cloud Run Admin
gcloud projects add-iam-policy-binding key-bringer \
  --member="serviceAccount:PROJECT_NUMBER-compute@developer.gserviceaccount.com" \
  --role="roles/run.admin"

# Service Account User (to deploy as key-bringer-sa)
gcloud iam service-accounts add-iam-policy-binding \
  key-bringer-sa@key-bringer.iam.gserviceaccount.com \
  --member="serviceAccount:PROJECT_NUMBER-compute@developer.gserviceaccount.com" \
  --role="roles/iam.serviceAccountUser"
```

### Generate TOTP Secret

Use the included `totp-gen` utility to create a TOTP secret with a scannable QR code:

```bash
go run ./cmd/totp-gen

# Or save QR code to a file:
go run ./cmd/totp-gen --output=totp-qr.png
```

This outputs:

- Base32 secret for GCP Secret Manager
- QR code to scan with your authenticator app

Security note:

- Treat the TOTP seed like a password. Don’t paste it into docs, issues, or logs.

### Create Secrets

| Secret Name          | Value                                   |
| -------------------- | --------------------------------------- |
| `zfs-master-key`     | Your ZFS encryption passphrase          |
| `agent-secret`       | Random password (share with host agent) |
| `totp-seed`          | Base32 seed from `totp-gen`             |
| `telnyx-api-key`     | Telnyx API Key (starts with `KEY...`)   |
| `telnyx-from-number` | Your Telnyx number (`+1...`)            |
| `telnyx-public-key`  | Telnyx Ed25519 Public Key               |
| `admin-phone`        | Your mobile number (`+1...`)            |

```bash
# Create secret containers (no values yet)
gcloud secrets create zfs-master-key --replication-policy="automatic"
gcloud secrets create agent-secret --replication-policy="automatic"
gcloud secrets create totp-seed --replication-policy="automatic"
gcloud secrets create telnyx-api-key --replication-policy="automatic"
gcloud secrets create telnyx-from-number --replication-policy="automatic"
gcloud secrets create telnyx-public-key --replication-policy="automatic"
gcloud secrets create admin-phone --replication-policy="automatic"

# Add secret versions by pasting values to stdin (avoids putting secrets in your shell history)
gcloud secrets versions add zfs-master-key --data-file=-
gcloud secrets versions add agent-secret --data-file=-
gcloud secrets versions add totp-seed --data-file=-
gcloud secrets versions add telnyx-api-key --data-file=-
gcloud secrets versions add telnyx-from-number --data-file=-
gcloud secrets versions add telnyx-public-key --data-file=-
gcloud secrets versions add admin-phone --data-file=-

# Important: pin numeric versions in config; do not use aliases like "latest".
```

---

## 2. Telnyx Setup

1. **Buy a Number**: [Telnyx Portal](https://portal.telnyx.com/) → Numbers → Search & Buy
2. **Create Messaging Profile**:
   - Go to Messaging → Programmable Messaging → Create Profile
   - **Inbound**: Set Webhook URL to `https://<YOUR-CLOUD-RUN-URL>/webhooks/telnyx`
   - **Allowed Destinations**: Check your country
3. **Associate Number**: Link your number to the Messaging Profile
4. **Get Credentials**:
  - API Key: https://portal.telnyx.com/#/api-keys
   - Public Key: Account Settings (for webhook verification)

Tip: keep the Telnyx portal handy: https://portal.telnyx.com/

---

## 3. Deploy

```bash
gcloud builds submit --config deploy/cloudbuild.yaml
```

After deployment, get your service URL:

```bash
gcloud run services describe key-bringer --region=us-central1 --format="value(status.url)"
```

Security note:

- The Telnyx webhook must be reachable by Telnyx.
- Telnyx cannot use Cloud Run IAM.
- Keep secret-delivering endpoints protected at the application layer (ID token auth) and keep the webhook protected by signature verification + replay protection.

### Rate Limiting (recommended)

Limit to 1 instance and 1 concurrent request to cap costs:

```bash
gcloud run services update key-bringer \
  --region=us-central1 \
  --max-instances=1 \
  --concurrency=1
```

### Billing Alert

Set a $10 budget alert in Cloud Console: Billing → Budgets & alerts → Create Budget.

**Update Telnyx webhook** with your Cloud Run URL + `/webhooks/telnyx`.

### Verification run (recommended)

Do a quick end-to-end test while you still have context loaded and can fix mistakes fast:

1. Start a test unlock (`key-seeker --monitor` on a test machineId).
2. Confirm you receive the SMS.
3. Reply with `APPROVE <machineId> <totp>`.
4. Confirm the poll completes and ZFS unlock succeeds.

---

## 4. Install key-seeker (Host Agent)

The `key-seeker` binary is cross-compiled on your development machine and copied to the Debian server. No Go installation required on the server.

### Build (on your dev machine)

```bash
# From the key-bringer repo directory (Linux/Mac)
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/key-seeker ./cmd/key-seeker

# PowerShell on Windows
$env:GOOS="linux"; $env:GOARCH="amd64"; go build -ldflags="-s -w" -o bin/key-seeker ./cmd/key-seeker

# Copy to server
scp bin/key-seeker root@your-server:/usr/local/bin/
ssh root@your-server chmod +x /usr/local/bin/key-seeker
```

### Configure

Create `/etc/key-seeker/env`:

```ini
SERVER_URL=https://key-bringer-xxxx.a.run.app
MACHINE_ID=myserver
AGENT_SECRET=OPTIONAL_DEFENSE_IN_DEPTH
ZFS_DATASET=zroot/encrypted
```

```bash
mkdir -p /etc/key-seeker
chmod 600 /etc/key-seeker/env
```

### Enable Service

```bash
scp systemd/key-seeker.service root@your-server:/etc/systemd/system/
ssh root@your-server 'systemctl daemon-reload && systemctl enable key-seeker'
```

---

## 5. Usage

**Boot Time**: Service starts automatically, sends SMS. Reply with 6-digit TOTP code.

**Manual Unlock with TOTP**:

```bash
# Source the env file first
set -a && source /etc/key-seeker/env && set +a

# Unlock with TOTP code from your authenticator
key-seeker --totp 123456
```

**Monitor Mode** (sends SMS, waits for reply):

```bash
key-seeker --monitor
```

---

## 6. Testing

### Create a Test Encrypted ZFS Dataset

To test without risking production data, create a test pool using a file:

```bash
# Create a 100MB file for the test pool
dd if=/dev/zero of=/tmp/zfs-test.img bs=1M count=100

# Create a zpool on that file
zpool create testpool /tmp/zfs-test.img

# Create an encrypted dataset (use the same passphrase as zfs-master-key)
zfs create -o encryption=aes-256-gcm -o keyformat=passphrase testpool/encrypted

# Verify encryption is enabled
zfs get encryption,keystatus testpool/encrypted
```

### Lock the Dataset (simulate reboot)

```bash
# Unmount first
zfs unmount testpool/encrypted

# Unload the key (locks the dataset)
zfs unload-key testpool/encrypted

# Verify it's locked
zfs get keystatus testpool/encrypted
# Should show: keystatus  unavailable
```

### Test Unlock

```bash
# Configure key-seeker for the test dataset
export SERVER_URL=https://key-bringer-xxxx.a.run.app
export MACHINE_ID=testserver
export AGENT_SECRET=your-agent-secret
export ZFS_DATASET=testpool/encrypted

# Unlock with a fresh TOTP code
key-seeker --totp <your-6-digit-code>

# Verify it's unlocked
zfs get keystatus testpool/encrypted
# Should show: keystatus  available
```

### Cleanup Test Pool

```bash
zpool destroy testpool
rm /tmp/zfs-test.img
```

---

## 7. Troubleshooting

### "allUsers" Policy Blocked (Google Workspace)

If you're in a Google Workspace organization, the `iam.allowedPolicyMemberDomains` org policy may block public access. To fix:

1. Grant yourself org policy admin:

   ```bash
   gcloud organizations add-iam-policy-binding YOUR_ORG_ID \
     --member="user:your-email@domain.com" \
     --role="roles/orgpolicy.policyAdmin"
   ```

2. Delete or modify the restriction:

   ```bash
   gcloud resource-manager org-policies delete iam.allowedPolicyMemberDomains \
     --organization=YOUR_ORG_ID
   ```

3. Retry adding allUsers to Cloud Run.

### Invalid TOTP Code

- **Check time sync**: TOTP is time-sensitive. Ensure your server and phone have accurate time.
- **Verify seed**: Check that your authenticator has the exact Base32 seed from `totp-seed` secret.
- **Regenerate with totp-gen**: Run `go run ./cmd/totp-gen` to create a fresh secret and update GCP.

### Cloud Run Returns 403 Forbidden

- Ensure `allUsers` has `roles/run.invoker` on the service
- Check if org policies are blocking (see above)
- Verify the service deployed successfully: `gcloud run services describe key-bringer --region=us-central1`

### key-seeker Errors

| Error                   | Cause                | Fix                                                  |
| ----------------------- | -------------------- | ---------------------------------------------------- |
| `server URL required`   | Env vars not loaded  | Use `set -a && source /etc/key-seeker/env && set +a` |
| `authentication failed` | Wrong agent secret   | Check `AGENT_SECRET` matches GCP secret              |
| `server returned 403`   | IAM blocking request | Add `allUsers` to Cloud Run                          |

---

## Development

```bash
# Run all tests
go test ./...

# Build all binaries
go build ./...

# Generate a TOTP secret
go run ./cmd/totp-gen

# Test TOTP validation locally
go run ./cmd/totp-test
```

---

## Utilities

### totp-gen

Generates a new TOTP secret with a scannable QR code:

```bash
# Display QR code in terminal
go run ./cmd/totp-gen

# Save QR code to PNG file
go run ./cmd/totp-gen --output=totp-qr.png

# Custom account name
go run ./cmd/totp-gen --name="MyServer" --issuer="KeyBringer"
```

---

## Status

This software is a proof of concept. If you aren't a fan of Telnyx or ZFS it shouldn't be too hard to add modules for other providers or encryption systems. Concerns are separated.

Current limitations:

- Single key from single server
- Single admin phone number
- In-memory session storage (sessions lost on Cloud Run cold start)

---

## License

MIT
