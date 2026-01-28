# Key Seeker Installation Guide (Debian Trixie)

## Prerequisites

- Debian Trixie with ZFS installed
- Network access to key-bringer Cloud Run service
- Agent secret from GCP Secret Manager

---

## Option A: Install from Binary

### 1. Download Binary

```bash
# Download latest release (replace URL with actual release)
curl -L https://github.com/Applesauce-Labs/key-bringer/releases/latest/download/key-seeker-linux-amd64 \
  -o /usr/local/bin/key-seeker
chmod +x /usr/local/bin/key-seeker
```

### 2. Create Config Directory

```bash
sudo mkdir -p /etc/key-seeker
sudo chmod 700 /etc/key-seeker
```

### 3. Configure Environment

```bash
sudo cp systemd/key-seeker.env.example /etc/key-seeker/env
sudo chmod 600 /etc/key-seeker/env
sudo nano /etc/key-seeker/env
```

Fill in:

- `SERVER_URL`: Your Cloud Run URL
- `MACHINE_ID`: This machine's identifier (e.g., `ny1`)
- `AGENT_SECRET`: From `gcloud secrets versions access latest --secret=agent-secret`
- `ZFS_DATASET`: The encrypted dataset (e.g., `tank/encrypted`)

### 4. Install Systemd Unit

```bash
sudo cp systemd/key-seeker.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable key-seeker
```

---

## Option B: Build from Source

```bash
# Install Go (1.25+)
sudo apt install golang-go

# Clone and build
git clone https://github.com/Applesauce-Labs/key-bringer.git
cd key-bringer
go build -o /usr/local/bin/key-seeker ./cmd/key-seeker

# Then follow steps 2-4 from Option A
```

---

## Testing

### Manual Test with TOTP

```bash
# Generate TOTP code with oathtool
sudo apt install oathtool

# Get code and unlock
key-seeker --totp $(oathtool --totp -b YOUR_TOTP_SEED)
```

### Test Systemd Service

```bash
# Start service manually
sudo systemctl start key-seeker

# Check logs
journalctl -u key-seeker -f
```

---

## Boot Unlock Flow

1. Server boots, ZFS datasets are locked
2. `key-seeker.service` starts after network is up
3. key-seeker contacts key-bringer, SMS is sent to admin
4. Admin replies with TOTP code
5. key-bringer verifies and returns ZFS key
6. key-seeker runs `zfs load-key` and `zfs mount`
7. System continues boot with unlocked storage

---

## Troubleshooting

### "connection refused"

- Check `SERVER_URL` is correct
- Verify Cloud Run service is deployed

### "401 Unauthorized"

- Verify `AGENT_SECRET` matches the secret in GCP

### "zfs load-key failed"

- Ensure ZFS dataset exists and is encrypted with `keyformat=passphrase`
- Verify the secret value matches the original passphrase
