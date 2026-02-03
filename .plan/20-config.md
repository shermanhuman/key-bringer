# Config Plan

## Goals

- Deterministic deploys.
- Minimal moving parts.
- Avoid leaking secrets into files.

## Files

- `.keybringer/config.yaml` (committed, no secrets)

## `.keybringer/config.yaml` (v1 sketch)

- `version`: schema version
- `gcp`:
  - `projectId`
  - `region` (default `us-central1`)

- `secrets` (GSM references; **numeric version pins**)
  - `zfsMasterKey`:
    - `secretId`
    - `version` (integer)
  - `totpSeed`:
    - `secretId`
    - `version` (integer)
  - `agentSecret`:
    - `secretId`
    - `version` (integer)
  - `telnyxApiKey`, `telnyxFromNumber`, `telnyxPublicKey`, `adminPhone` (same shape)

- `machines`:
  - list or map of `machineId` → unlock target config

- `runtime`:
  - `requireIamAuth`: true
  - `maxPendingMinutes`: 10
    - Source of truth for SessionStore TTL (and any other short-lived best-effort state like webhook event-id deduplication).

## Environment variables

- Environment variables remain acceptable for Cloud Run wiring (non-secret config).
- Secret values should be fetched via GSM API at runtime.
- Avoid wiring secrets to env vars via `--set-secrets` with `:latest`.

Webhook path tokens:

- For per-unlock-session rotation, webhook path tokens are generated at runtime and kept only in memory (current + previous overlap window).
- If a static/fixed token is supported for manual rotation, treat it as a secret: store in GSM with a pinned numeric version and fetch at runtime.

## Version pinning policy

- Never use GSM aliases like `latest`.
- Rotating a secret means:
  - create a new GSM version
  - update `.keybringer/config.yaml` to point at the new version
  - redeploy
  - validate the new secret works
  - optionally disable old GSM version after grace period

## Secrets Rotation Schedule

See [60-security.md](60-security.md#secrets-rotation-schedule-planned) for comprehensive rotation timing recommendations and procedures.

Quick reference:
- **ZFS Master Key**: On compromise/decommission only
- **TOTP Seed**: Annually or on compromise
- **Telnyx API Key**: Every 90 days
- **Agent Secret**: Every 180 days
- **Webhook Path Token**: Per unlock session (automatic)
- **Telnyx Public Key**: When Telnyx rotates (monitor release notes)
