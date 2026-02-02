# Security Plan

## Authentication

- Cloud Run is private-by-default.
- key-seeker authenticates using a Google-signed ID token (audience = server URL).
- Server verifies the ID token (framework/middleware).

## Authorization

- Defense-in-depth: retain `X-Agent-Secret` header for host authentication.
- Restrict `machineId` to an allow-list from config.

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
