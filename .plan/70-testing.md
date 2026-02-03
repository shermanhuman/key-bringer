# Testing Plan

## Unit tests

- TOTP verifier correctness (windowing, invalid codes).
- Telnyx webhook signature verification and replay handling.
	- Signature is verified over `${timestamp}|${raw_body}` (raw bytes; whitespace changes should invalidate signature).
	- Reject missing/invalid headers and stale timestamps.
- Telnyx webhook idempotency/dedup behavior (event `id`).
- SMS approval parsing (requires `machineId` + TOTP; rejects malformed texts).
- SessionStore behavior (TTL, replace-on-new-request, approval flow).
- Handler tests for `/unlock` and `/poll` with fakes.

## Integration tests (optional / manual)

- GSM access using ADC in a test project.
- End-to-end: deploy to Cloud Run, request unlock, approve via SMS, poll and unlock.

### Operator verification run (recommended)

Include a short, human-run check in the onboarding workflow (best placed at the end of `key-bringer init`, and repeatable via `key-bringer doctor --verify`).

Goal: verify all “real world” moving parts while the operator can still fix setup quickly:

- Telnyx outbound SMS works (credentials + from-number configured).
- Telnyx inbound webhook works (URL reachable + signature verified).
- TOTP is enrolled correctly (admin can generate codes).
- `/unlock` → SMS challenge → SMS reply → `/poll` completes.

Notes:

- Never print or log the TOTP seed.
- Use obviously fake placeholders in docs (e.g., `+15555550123`, `KEY_REDACTED`).

### Webhook rotation drill (optional)

Goal: measure how Telnyx behaves when you change the messaging profile `webhook_url`, and verify your rotation procedure does not drop approvals.

Important constraints:

- Even if the configuration change takes effect quickly, there can still be in-flight deliveries and retries targeting the previous URL.
- Treat the result as an observation, not a guarantee of instantaneous cutover.

Suggested drill:

1. Deploy the service with support for *two* valid webhook path tokens (current + previous), both returning `2xx` quickly.
2. Set Telnyx messaging profile `webhook_url` to Token A and confirm inbound delivery works (observe `meta.delivered_to` and `meta.attempt`).
3. Rotate the messaging profile `webhook_url` to Token B.
4. Immediately send an inbound SMS and observe which token receives it.
5. Force an “in-flight” case:
	- send an inbound SMS while Token A is configured
	- immediately rotate to Token B
	- observe whether any deliveries/retries still target Token A

Rotate-before-challenge drill (matches the v1 workflow):

1. Rotate Telnyx `webhook_url` to Token B.
2. Wait until the server reports the new endpoint is reachable (health loop; bounded timeout).
3. Only then send the SMS challenge.
4. Reply immediately and confirm the inbound webhook hits Token B.

What to record:

- Timestamp when the Telnyx API update returns.
- Timestamp when the first inbound webhook is observed at Token B.
- Use that observed cutover time to choose an overlap window. Default overlap: 60 seconds.

Pass criteria:

- During and after rotation, inbound webhooks are still accepted and processed safely.
- Duplicate deliveries do not cause double-approval side effects.

## CI

- `go test ./...` on PR.
- Lint (optional) if you standardize on `golangci-lint`.
