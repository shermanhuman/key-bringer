# key-bringer – Planning Overview

## System goal

Key-bringer provides secure, automated unlock of encrypted storage at boot time.

- Source of truth for sensitive values: Google Secret Manager (GSM)
- Unlock decision: 2-way SMS challenge + TOTP verification
- Host agent (`key-seeker`) unlocks locally after authorization

## Non-negotiable invariants

- Never write the ZFS master key (or other unlock keys) to disk.
- Never log secrets.
- Cloud Run is private-by-default (IAM-authenticated). No `allUsers`.
- No user-managed service account keys in files or repos.
- Secret Manager references are deterministic: **pinned numeric versions** (no `latest`).

## Opinionated v1 scope

This is a lightweight system; v1 deliberately avoids additional infrastructure.

- No Redis.
- No database.
- One Cloud Run instance only (enforced via deployment settings).
  - `max-instances=1`
  - `concurrency=1`

Rationale: the only state we need is short-lived “pending approval” state during an unlock window.
If Cloud Run restarts, the operator can retry.

Default unlock window (v1):

- SessionStore TTL comes from config `runtime.maxPendingMinutes` (default 10 minutes).

## HTTP stack decision (v1)

Use the Go standard library HTTP stack:

- Router: `net/http` `http.ServeMux` (Go 1.22+ patterns).
- Server: `http.Server` with explicit timeouts and request body size limits.

Deliberate non-choice:

- Do **not** introduce a web framework (Gin, Echo, Chi, etc.) in v1.

Rationale:

- Small API surface area (a handful of endpoints) doesn’t justify the dependency surface.
- Raw request body handling must be explicit for Telnyx signature verification.
- Fewer defaults makes “never log secrets” and “fail closed” easier to enforce and test.

## Session/state decision (no Redis)

We still keep a **small session abstraction**, but it is intentionally implemented as in-memory only in v1.

- `SessionStore` exists primarily for testability and to keep the handler logic clean.
- v1 implementation: `memory`.
- Non-goal (v1): multi-instance horizontal scaling.

Security note:

- Do not store the secret value inside the session.
- Sessions store only metadata: `machine_id`, timestamps, approval state.
- When the session is approved and the client polls, fetch the unlock key from GSM *at that moment*.

## SMS challenge flow (v1)

- `POST /unlock` creates a pending session and sends an SMS challenge to the configured admin phone.
- Admin replies via SMS (delivered to the Telnyx webhook) with `machineId` + TOTP to approve.

## GCP setup (operator workflow)

Key-bringer should provide deterministic, scriptless provisioning:

- `key-bringer gcp bootstrap` shells out to `gcloud` (assumed installed and authenticated).
- It enables APIs, creates service accounts, configures IAM (least privilege), and prints the deploy command.

## Roadmap boundaries

If/when multi-instance support is needed later, revisit the state model.
Prefer a serverless persistence option (e.g., Firestore) over self-managed Redis, but this is out of scope for v1.
