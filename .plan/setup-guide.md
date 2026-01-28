# Operator Setup Guide

This guide walks you through setting up the external dependencies for `key-bringer`.

---

## 1. Google Cloud Setup

### 1.1 Create a GCP Project (if needed)

```bash
gcloud projects create key-bringer-prod --name="Key Bringer"
gcloud config set project key-bringer-prod
```

### 1.2 Enable Required APIs

```bash
gcloud services enable \
  secretmanager.googleapis.com \
  run.googleapis.com \
  cloudbuild.googleapis.com
```

### 1.3 Create Secrets in Secret Manager

You need to store 3 secrets:

| Secret Name      | Description                                     |
| ---------------- | ----------------------------------------------- |
| `zfs-master-key` | The ZFS encryption passphrase                   |
| `agent-secret`   | Shared secret for key-seeker → key-bringer auth |
| `totp-seed`      | Base32 TOTP seed for Google Authenticator       |

```bash
# Create the ZFS key
echo -n "your-zfs-passphrase" | gcloud secrets create zfs-master-key --data-file=-

# Create the agent secret (generate a random one)
openssl rand -base64 32 | gcloud secrets create agent-secret --data-file=-

# Create TOTP seed (for Google Authenticator)
# Use a tool like `oathtool` or generate via https://totp.danhersam.com/
echo -n "JBSWY3DPEHPK3PXP" | gcloud secrets create totp-seed --data-file=-
```

### 1.4 Create a Service Account for Cloud Run

```bash
# Create service account
gcloud iam service-accounts create key-bringer-sa \
  --display-name="Key Bringer Service Account"

# Grant access to secrets
for secret in zfs-master-key agent-secret totp-seed; do
  gcloud secrets add-iam-policy-binding $secret \
    --member="serviceAccount:key-bringer-sa@key-bringer-prod.iam.gserviceaccount.com" \
    --role="roles/secretmanager.secretAccessor"
done
```

### 1.5 Add TOTP to Google Authenticator

1. Open Google Authenticator on your phone
2. Tap **+** → **Enter a setup key**
3. Name: `key-bringer`
4. Key: `JBSWY3DPEHPK3PXP` (same as `totp-seed` above)
5. Type: Time-based
6. Save

---

## 2. Telnyx Setup

### 2.1 Get Your API Key

1. Log in to [Telnyx Mission Control](https://portal.telnyx.com/)
2. Go to **Account** → **Keys & Credentials**
3. Copy your **API Key** (starts with `KEY...`)
4. Note your **Public Key** (for webhook verification)

### 2.2 Buy a Phone Number

1. Go to **Numbers** → **Search & Buy**
2. Purchase a number with SMS capability
3. Note the number (e.g., `+12025551234`)

### 2.3 Configure Messaging Profile

1. Go to **Messaging** → **Messaging Profiles**
2. Create a new profile (e.g., `key-bringer`)
3. Under **Inbound Settings**:
   - Webhook URL: `https://<your-cloud-run-url>/webhooks/telnyx`
   - Webhook API Version: **v2**
4. Assign your phone number to this profile

### 2.4 Store Telnyx Secrets in GCP

```bash
# Store API key
echo -n "KEY_YOUR_API_KEY" | gcloud secrets create telnyx-api-key --data-file=-

# Store the from number
echo -n "+12025551234" | gcloud secrets create telnyx-from-number --data-file=-

# Store the public key (for webhook verification)
echo -n "YOUR_PUBLIC_KEY" | gcloud secrets create telnyx-public-key --data-file=-
```

---

## 3. Deploy to Cloud Run

After building the Docker image:

```bash
gcloud run deploy key-bringer \
  --image gcr.io/key-bringer-prod/key-bringer:latest \
  --service-account key-bringer-sa@key-bringer-prod.iam.gserviceaccount.com \
  --region us-central1 \
  --allow-unauthenticated \
  --set-secrets="AGENT_SECRET=agent-secret:latest,TOTP_SEED=totp-seed:latest,ZFS_KEY=zfs-master-key:latest,TELNYX_API_KEY=telnyx-api-key:latest,TELNYX_FROM_NUMBER=telnyx-from-number:latest,TELNYX_PUBLIC_KEY=telnyx-public-key:latest"
```

---

## 4. Configure the Host (key-seeker)

1. Copy the `key-seeker` binary to `/usr/local/bin/`
2. Create config at `/etc/key-seeker/config.yaml`:
   ```yaml
   server_url: https://key-bringer-xyz.run.app
   machine_id: ny1
   agent_secret: <copy from `gcloud secrets versions access latest --secret=agent-secret`>
   ```
3. Install systemd unit:
   ```bash
   cp systemd/key-seeker.service /etc/systemd/system/
   systemctl enable key-seeker
   ```

---

## 5. Test the Flow

1. Reboot the host (or run `systemctl start key-seeker`)
2. You should receive an SMS
3. Reply with your 6-digit code from Google Authenticator
4. ZFS should unlock automatically
