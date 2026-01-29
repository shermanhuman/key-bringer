# Keybringer

Secure, automated ZFS encryption at rest with secure offsite passphrase storage in Google Cloud secret manager and 2-way SMS and TOTP verification.  For those of us who don't want to get up at 3AM to ssh in.

## Overview

Keybringer is a serverless solution for unlocking ZFS encrypted volumes at boot time. It consists of:

- **key-bringer**: Cloud Run service that holds secrets and handles SMS verification
- **key-seeker**: Host agent that requests keys and unlocks ZFS

## Architecture

```
┌─────────────────┐     HTTPS      ┌──────────────────┐
│  Debian Server  │◄──────────────►│   Cloud Run      │
│  (key-seeker)   │                │   (key-bringer)  │
└────────┬────────┘                └────────┬─────────┘
         │                                  │ SMS
         ▼                                 ▼
   ZFS Encrypted                    Admin Phone
     Volumes                     (Authenticator App)
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

### Create Secrets

| Secret Name          | Value                                   |
| -------------------- | --------------------------------------- |
| `zfs-master-key`     | Your ZFS encryption passphrase          |
| `agent-secret`       | Random password (share with host agent) |
| `totp-seed`          | Base32 seed (add to Authenticator app)  |
| `telnyx-api-key`     | Telnyx API Key (starts with `KEY...`)   |
| `telnyx-from-number` | Your Telnyx number (`+1...`)            |
| `telnyx-public-key`  | Telnyx Ed25519 Public Key               |
| `admin-phone`        | Your mobile number (`+1...`)            |

```bash
echo -n "your-passphrase" | gcloud secrets create zfs-master-key --data-file=-
echo -n "random-secret" | gcloud secrets create agent-secret --data-file=-
echo -n "BASE32SEED" | gcloud secrets create totp-seed --data-file=-
echo -n "KEY..." | gcloud secrets create telnyx-api-key --data-file=-
echo -n "+15551234567" | gcloud secrets create telnyx-from-number --data-file=-
echo -n "public-key" | gcloud secrets create telnyx-public-key --data-file=-
echo -n "+15559876543" | gcloud secrets create admin-phone --data-file=-
```

**Add TOTP to your Authenticator app:**

1. Open Google Authenticator / Authy / 1Password
2. Add manual entry: Name=`KeyBringer`, Key=`<your totp-seed>`, Type=Time-based

---

## 2. Telnyx Setup

1. **Buy a Number**: [Telnyx Portal](https://portal.telnyx.com/) → Numbers → Search & Buy
2. **Create Messaging Profile**:
   - Go to Messaging → Programmable Messaging → Create Profile
   - **Inbound**: Set Webhook URL to `https://<YOUR-CLOUD-RUN-URL>/webhooks/telnyx`
   - **Allowed Destinations**: Check your country
3. **Associate Number**: Link your number to the Messaging Profile
4. **Get Credentials**:
   - API Key: Account Settings → API Keys
   - Public Key: Account Settings (for webhook verification)

---

## 3. Deploy

```bash
gcloud builds submit --config deploy/cloudbuild.yaml
```

After deployment, get your service URL:

```bash
gcloud run services describe key-bringer --region=us-central1 --format="value(status.url)"
```

Make it publicly accessible:

```bash
gcloud run services add-iam-policy-binding key-bringer \
  --region=us-central1 \
  --member="allUsers" \
  --role="roles/run.invoker"
```

**Update Telnyx webhook** with your Cloud Run URL + `/webhooks/telnyx`.

---

## 4. Install Host Agent (Debian)

### Build

```bash
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/key-seeker ./cmd/key-seeker
scp bin/key-seeker root@your-server:/usr/local/bin/
```

### Configure

Create `/etc/key-seeker/env`:

```ini
SERVER_URL=https://key-bringer-xxxx.a.run.app
MACHINE_ID=ny1
AGENT_SECRET=<same-as-gcp-secret>
ZFS_DATASET=zroot/encrypted
```

```bash
chmod 600 /etc/key-seeker/env
```

### Enable Service

```bash
cp systemd/key-seeker.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable key-seeker
```

---

## 5. Usage

**Boot Time**: Service starts automatically, sends SMS. Reply with 6-digit TOTP code.

**Manual Unlock**:

```bash
key-seeker --totp 123456
```

---

## Development

```bash
go test ./...
go build ./...
```

# Status

This is software is a barely tested proof of concept, so treat it as such.  If you aren't a fan of Telnyx or ZFS it shouldn't be to hard to add modules for other providers or maybe some other form of encryption.  I tried to keep concerns separated.

Right now it only accepts a single key from a single server and only sends to a single admin phone.

## License

MIT
