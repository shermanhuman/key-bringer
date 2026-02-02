# Data Model Plan

## Session model (no Redis)

Goal: represent a short-lived unlock request without introducing external state.

### Session

Fields (v1):

- `sessionId` (opaque)
- `machineId`
- `requestedAt`
- `approvedAt` (optional)

Rules:

- Never store the unlock secret itself inside the session.
- TTL: 10 minutes.
- Limit: at most one active session per `machineId` (replace the old one on new request).

### Approval model

- SMS reply with valid TOTP approves the most recent pending session for that phone/machine.
- When `key-seeker` polls an approved session, the server fetches the pinned GSM secret version and returns it.

## Metadata

Keep a single source-of-truth config file (`.keybringer/config.yaml`) for:

- what secrets exist
- which versions are currently deployed
- which machines are allowed

This makes behavior deterministic and reviewable.
