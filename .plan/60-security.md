# Security Plan

## Authentication

- KeyBringer endpoints that deliver secrets must require a Google-signed ID token.
- key-seeker authenticates using a Google-signed ID token (audience = server URL).
- Server verifies the ID token (standard `net/http` middleware).

Webhook reality check:

- Telnyx cannot present Google Cloud IAM credentials.
- Therefore, the Telnyx webhook must be reachable without Cloud Run IAM.
- Security for `/webhooks/telnyx` (and `/webhooks/telnyx/<token>` when enabled) comes from signature verification + replay protection + idempotency.

Operational guidance:

- Keep `/unlock` and `/poll` locked behind ID token auth.
- Treat the Telnyx webhook route (`/webhooks/telnyx/<token>`) as public-but-verified.

## Authorization

- Defense-in-depth: retain `X-Agent-Secret` header for host authentication.
- Restrict `machineId` to an allow-list from config.

Notes on `X-Agent-Secret` (v1):

- This is optional defense-in-depth, not the primary control.
- Only enable it if `key-seeker` can supply the value securely (without introducing a new “secret at rest on hosts” problem).

## Secret handling

- Secrets are only fetched from GSM at the moment they are needed.
- Secrets are never persisted to disk.
- Secrets are never logged.

## Secret Manager best practices applied

- Least-privilege IAM.
- Prefer ADC / workload identity. Avoid downloading service account keys.
- Pin secret versions (no aliases like `latest`).

## Abuse resistance

- Rate limit unlock requests per machineId.
- Lock out on repeated invalid TOTP attempts.
- Validate Telnyx webhook signatures and reject replays.

### Public Telnyx webhook endpoint hardening (v1)

We should assume `/webhooks/telnyx` (and tokenized variants) are publicly reachable (Telnyx cannot use Cloud Run IAM).

Add a defense-in-depth “unguessable path token”:

- Prefer routing the webhook to a path like `/webhooks/telnyx/<token>` where `<token>` is a long random URL-safe value.
- Requests to unknown paths should fail fast (e.g., `404`) without expensive work and without logging the token.
- This does not replace signature verification. Always verify `telnyx-signature-ed25519` + `telnyx-timestamp` and enforce replay protection.

**Rotation approach (per unlock session for single-user v1):**

For a single-user system where one unlock session completes before the next begins, rotate the webhook token *before* sending each SMS challenge:

1. Generate a new webhook token.
2. Update the Telnyx messaging profile `webhook_url` (via API).
3. Test the new endpoint is reachable (quick health loop with timeout).
4. Only after (2) and (3) succeed, send the SMS challenge.
5. Accept the reply via the new webhook URL.

This ensures each unlock session uses a fresh endpoint, minimizing exposure window.

**Overlap handling (60-second grace):**

- Accept both current and previous tokens during rotation to tolerate Telnyx propagation delay and in-flight retries.
- Track `WEBHOOK_PATH_TOKEN_PREVIOUS` and `WEBHOOK_PATH_TOKEN_PREVIOUS_VALID_UNTIL` (Unix timestamp).
- After the grace window expires, reject the previous token.

**Observed rotation timing (validated 2026-02-02):**

- Telnyx messaging profile `webhook_url` API update: ~300-600ms (API call latency)
- Propagation delay: ~7-10 seconds (time until first webhook delivery hits new endpoint)
- Recommended overlap window: 60 seconds (provides safety margin for retries + concurrent unlocks)

Operational note:

- Per-session rotation changes the messaging profile webhook destination. This is only appropriate when you control the whole flow (single-user v1) and you fail closed on update/test errors.

**Token sizing:**

- The Telnyx API reference models `webhook_url` as `string<url>` but does not publish a max length constraint.
- Use a conservative size that fits comfortably in common infra limits. A 32-byte random token (Base64URL ~43 chars) is more than enough.

Input validation notes:

- Only accept approvals from the configured admin phone number.
- Require the SMS text to include both `machineId` and a TOTP code (reject anything else).

## Telnyx webhook authenticity (v1)

Telnyx signs webhook events so we can verify authenticity.

- Read `telnyx-timestamp` header (Unix timestamp).
	- Reject if missing or outside an allowed window (e.g., > 5 minutes old) to prevent replay.
- Read `telnyx-signature-ed25519` header (base64-encoded signature).
	- Reject if missing or cannot be base64-decoded.
- Verify the signature using the configured Telnyx public key (Ed25519) over the exact message bytes:
	- message = `${telnyx-timestamp}|${raw_request_body}`
	- `raw_request_body` must be the exact bytes received (do not re-marshal JSON; whitespace/newlines matter).
- Treat Telnyx webhooks as potentially duplicated and out-of-order.
	- Prefer idempotent processing keyed by the webhook event `id`.

Operational note:

- Telnyx retries deliveries if the endpoint does not return `2xx` within ~2 seconds.
	- Prefer verifying + acknowledging quickly, then doing slower work after.

## Secrets Rotation Schedule (Planned)

Establish a rotation schedule for all cryptographic material and credentials. Rotation creates new GSM secret versions and updates `.keybringer/config.yaml` to pin the new version.

### Rotation Timing Recommendations

**ZFS Master Key:**
- **Frequency**: On compromise or host decommission only (not routine)
- **Rationale**: High-impact change (requires re-encrypting ZFS dataset or migrating data); rotate only when necessary
- **Procedure**: Create new GSM version → update config → redeploy → re-key ZFS (or migrate data to new encrypted dataset)

**TOTP Seed:**
- **Frequency**: Annually or on compromise
- **Rationale**: TOTP seed compromise allows attacker to generate valid codes; annual rotation limits exposure window
- **Procedure**: Generate new seed → create GSM version → update config → redeploy → operator re-enrolls in authenticator app (scan new QR code)
- **Caution**: Coordinate with operator (requires re-enrollment); consider doing during maintenance window

**Telnyx API Key:**
- **Frequency**: Every 90 days or on compromise
- **Rationale**: API key grants control over messaging profile and incurs costs; regular rotation limits blast radius
- **Procedure**: Generate new key in Telnyx portal → create GSM version → update config → redeploy → revoke old key after validation

**Agent Secret (if used):**
- **Frequency**: Every 180 days or on suspected host compromise
- **Rationale**: Defense-in-depth credential; less critical than IAM tokens but should still be rotated
- **Procedure**: Generate new secret → create GSM version → update config → redeploy server → update `key-seeker` on all hosts → validate → revoke old secret

**Webhook Path Token:**
- **Frequency**: Per unlock session (automatic) OR on incident (manual)
- **Rationale**: Ephemeral token rotated automatically; manual rotation if suspicious traffic detected
- **Procedure**: Automatic (part of unlock flow) or manual via operator command

**Telnyx Public Key (webhook verification):**
- **Frequency**: Only when Telnyx rotates their signing key (monitor Telnyx release notes)
- **Rationale**: Telnyx controls this key; update when they publish new key
- **Procedure**: Obtain new public key from Telnyx → create GSM version → update config → redeploy

### Rotation Procedure (General)

1. Generate/obtain new secret value (never via command line; use stdin or secure generation)
2. Create new GSM secret version:
   ```bash
   echo -n "new-secret-value" | gcloud secrets versions add SECRET_NAME --data-file=-
   ```
3. Note the new version number (e.g., `2`)
4. Update `.keybringer/config.yaml` to pin new version
5. Commit config change (no secrets in commit)
6. Redeploy Cloud Run service
7. Validate new secret works (run `key-bringer doctor --verify` or targeted test)
8. Document rotation in changelog/audit log
9. (Optional) Disable old GSM secret version after grace period

### Emergency Rotation (Compromise Response)

If a secret is compromised:

1. Immediately rotate the compromised secret (follow procedure above)
2. Review Cloud Run logs and GSM audit logs for unauthorized access
3. Identify what data/systems were exposed
4. If ZFS key or TOTP seed compromised: consider rotating all secrets and re-evaluating trust boundaries
5. If webhook token compromised: automatic per-session rotation limits damage; monitor for unusual traffic
6. Document incident and lessons learned
