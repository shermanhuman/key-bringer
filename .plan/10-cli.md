# CLI Plan

## Goals

- Make setup repeatable and non-mysterious.
- Keep the operational surface area small.
- Prefer a single “happy path” onboarding flow.

## Operator quick links

When setup requires an external credential, point operators at the exact UI.

- Telnyx API keys: https://portal.telnyx.com/#/api-keys
- Telnyx portal (numbers, messaging profiles): https://portal.telnyx.com/
- Google Secret Manager docs: https://cloud.google.com/secret-manager/docs
- Cloud Run security overview: https://cloud.google.com/run/docs/securing

## CLI shape

Adopt Cobra for both binaries:

- `key-bringer` (admin/operator CLI + server runner)
- `key-seeker` (host agent CLI)

Config approach:

- Cobra only; no Viper.
- Repo config is parsed directly from YAML into structs and fails closed on unknown fields.

## Proposed commands (v1)

### key-bringer

- `key-bringer serve`
  - Starts the Cloud Run HTTP service.
  - Reads config from `.keybringer/config.yaml` by default.
  - Optional: `--config <path>` overrides the default path.

- `key-bringer init`
  - Guided setup:
    - validates `gcloud` presence and auth
    - runs `gcp bootstrap`
    - creates required GSM secrets (interactive values piped via stdin)
      - If a TOTP seed is not provided, generate one locally (cryptographically random) and store it as a GSM secret version.
      - Show an `otpauth://` URI / QR code **once** so the admin can enroll it in an authenticator app.
    - prints a deterministic `gcloud run deploy ...` command
    - offers a verification run (recommended)

### Verification run (recommended)

After the first deploy, operators should perform a quick end-to-end check while they are at the keyboard and have their phone.

Prefer implementing this as a repeatable `doctor` sub-check so it’s usable later for troubleshooting.

- `key-bringer doctor --verify`
  - Verifies `/healthz` responds.
  - Verifies Telnyx outbound SMS credentials by sending a single “test” message to the configured admin phone.
  - Verifies Telnyx inbound webhook by waiting for a reply and confirming signature verification succeeds.
  - Verifies TOTP end-to-end by prompting the admin to reply with `APPROVE <machineId> <totp>`.
  - Pass criteria: a test session transitions `pending` → `approved` and can be observed via `GET /poll`.

### Webhook rotation (per unlock session OR incident response)

Telnyx webhooks must be publicly reachable, so we treat the Telnyx webhook as public-but-verified.

Defense-in-depth: use an unguessable path token for the webhook endpoint, and rotate it before each unlock session (single-user v1) or on incident/maintenance.

**Per-unlock-session rotation flow (recommended for v1 single-user system):**

1. key-seeker requests unlock via `POST /unlock` (IAM-authenticated).
2. key-bringer generates a new webhook path token and updates the Telnyx messaging profile `webhook_url` (via Telnyx API).
3. key-bringer tests the new endpoint is reachable (quick health check loop with timeout).
4. Only after (2) and (3) succeed, key-bringer sends the SMS challenge to the admin phone.
5. User replies with TOTP code; Telnyx delivers to the new webhook URL.
6. After approval (or timeout), the webhook token can be rotated again for the next session.

Notes:

- Updating the Telnyx `webhook_url` affects all inbound messages for that messaging profile; per-unlock-session rotation is only appropriate when the operator controls the flow (single-user v1).
- If Telnyx update fails, fail closed: do not send the SMS challenge.

**Overlap handling (60-second grace window):**

- Accept both current and previous tokens during rotation to tolerate in-flight deliveries/retries.
- Store `WEBHOOK_PATH_TOKEN_PREVIOUS` and `WEBHOOK_PATH_TOKEN_PREVIOUS_VALID_UNTIL` (Unix timestamp).
- After the grace window expires, only the current token is accepted.

**Token storage:**

- Per-unlock-session rotation: the server generates a fresh token per unlock, keeps it in memory, and optionally keeps the previous token valid for ~60 seconds.
- Do not commit tokens to `.keybringer/config.yaml`.
- If you support a manual/incident “fixed token” mode for troubleshooting, treat the token as a secret (store in GSM with a pinned numeric version; load at runtime). Avoid plaintext env vars for long-lived tokens.
- Generate tokens locally when needed: `openssl rand -base64 32 | tr '+/' '-_' | tr -d '='`
- Never log the full webhook URL/token.

**Rotation drill (optional but recommended):**

- Update the Telnyx messaging profile `webhook_url`.
- Send an inbound SMS immediately after the change.
- Confirm the webhook arrives at the expected URL (`meta.delivered_to`) and observe timing.
- This helps you understand Telnyx propagation delay in your environment.

- `key-bringer gcp bootstrap`
  - Opinionated provisioning wrapper around `gcloud`:
    - enable APIs
    - create service accounts
    - assign least-privilege IAM for GSM access
    - configure Cloud Run service identity + invoker policy
  - Must be safe to re-run.
  - `--dry-run` prints commands.

- `key-bringer doctor`
  - Non-destructive validation:
    - required APIs enabled
    - required secrets exist and are pinned to numeric versions
    - service account bindings exist

### key-seeker

- `key-seeker unlock`
  - Requests unlock. Supports two modes:
    - `--totp <code>` immediate unlock
    - `--monitor` (poll until approved)

- `key-seeker doctor`
  - Verifies it can reach the server and obtain an ID token.

## Cloud Run auth decision (v1)

- key-bringer requires IAM authentication.
- key-seeker must obtain a Google-signed ID token (Google auth library) and fails closed if it cannot.
  - Audience (`aud`) must be the Cloud Run service URL (or a configured custom audience).
  - If not using custom audiences, keep `aud` as the base service URL even when calling a specific traffic tag.
  - Prefer ADC / workload identity federation; avoid service account keys.
  - Do not accept credential JSON/config from untrusted external sources.
- `X-Agent-Secret` header can remain as defense-in-depth, but it is not the primary access control.

### Auth header choice

- Preferred: send the ID token via `Authorization: Bearer <token>`.
- Alternative: `X-Serverless-Authorization: Bearer <token>` if the application needs to reserve `Authorization` for its own scheme.
  - If both headers are present, Cloud Run checks `X-Serverless-Authorization`.
