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
- TTL: from config `runtime.maxPendingMinutes` (default 10 minutes).
- Limit: at most one active session per `machineId` (replace the old one on new request).

### Approval model

- SMS reply with valid TOTP approves the most recent pending session for that phone/machine.
- To avoid ambiguity when multiple machines exist, the SMS text must include the `machineId`.
	- Example operator reply format (v1): `APPROVE <machineId> <totp>`
- When `key-seeker` polls an approved session, the server fetches the pinned GSM secret version and returns it.

### Webhook delivery characteristics (Telnyx)

Telnyx may deliver webhook events concurrently, out-of-order, and more than once.

- Handler logic must be idempotent.
- Prefer deduplication keyed by the webhook event `id`.
	- v1 can keep an in-memory set with TTL aligned to `runtime.maxPendingMinutes`.
	- Dedup state is best-effort (Cloud Run restarts can forget it), so the approval operation itself should also be safe to re-apply.

## Metadata

Keep a single source-of-truth config file (`.keybringer/config.yaml`) for:

- what secrets exist
- which versions are currently deployed
- which machines are allowed

This makes behavior deterministic and reviewable.
