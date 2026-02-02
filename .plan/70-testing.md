# Testing Plan

## Unit tests

- TOTP verifier correctness (windowing, invalid codes).
- Telnyx webhook signature verification and replay handling.
- SessionStore behavior (TTL, replace-on-new-request, approval flow).
- Handler tests for `/unlock` and `/poll` with fakes.

## Integration tests (optional / manual)

- GSM access using ADC in a test project.
- End-to-end: deploy to Cloud Run, request unlock, approve via SMS, poll and unlock.

## CI

- `go test ./...` on PR.
- Lint (optional) if you standardize on `golangci-lint`.
