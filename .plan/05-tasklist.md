# Implementation Tasklist (v1)

This file is the execution checklist for the v1 plan.

Plan status (as of 2026-02-02):

- The v1 plan is opinionated and implementation-ready.
- Key decisions are locked (no new frameworks or infra without a plan update).
- HTTP stack is `net/http` + `http.ServeMux` (Go 1.22+). Do not introduce Gin.

Rules for this tasklist:

- Every implementation task has an explicit test task immediately after it.
- Each test task must validate the stated success criteria (unit and/or manual).
- If a task would materially change scope or security posture, stop and update the relevant `.plan/` doc(s) first.

## 0) Baseline hygiene

1. [ ] Align README with plan defaults (private-by-default, no `versions/latest`, ADC/WIF preferred)
   - Success: README no longer recommends `allUsers` invoker and does not suggest `versions/latest` usage.
2. [ ] Test (manual): quick doc review
   - Steps: skim README sections for Cloud Run auth + Secret Manager usage.
   - Pass: no public invoker steps; secret references are described as pinned numeric versions.

## 1) Config (`.keybringer/config.yaml`)

3. [ ] Implement config schema structs + loader
   - Success: `.keybringer/config.yaml` parses into structs; unknown fields fail closed; required fields validated with actionable errors.
4. [ ] Test (unit): config parsing/validation
   - Cases: missing required fields; unknown field present; invalid version pin (non-integer/zero); malformed YAML.
   - Pass: loader returns errors without leaking secret values.

5. [ ] Define canonical “secret reference” type (secretId + numeric version)
   - Success: all secret references in config use a single type; no code path accepts `latest`/alias strings.
6. [ ] Test (unit): secret reference validation
   - Cases: version=0; version<0; empty secretId.
   - Pass: errors include context but not secret values.

## 2) Guardrails (determinism + no secret leakage)

7. [ ] Add hard gate: never allow `versions/latest` anywhere in runtime code paths
   - Success: code does not build or tests fail if `versions/latest` is introduced in Go source.
8. [ ] Test (unit): forbid `versions/latest` string
   - Approach: add a small test that scans Go files under `internal/` (and optionally `cmd/`) and fails if it finds `versions/latest`.
   - Pass: `go test ./...` fails loudly if the forbidden string is present.

9. [ ] Define and enforce a logging contract (no request bodies; never log secrets)
   - Success: logs are structured; request bodies are not logged; errors never include secret values.
10. [ ] Test (unit): log redaction / no-secret assertions
   - Approach: capture logger output in tests and assert known fake secrets do not appear.

## 3) Secret Manager access (deterministic)

11. [ ] Implement GSM client wrapper that only accepts pinned numeric versions
   - Success: all fetches build resource names using `versions/<number>`; callers cannot request `latest`.
12. [ ] Test (unit): GSM name construction
   - Cases: valid secretId/version produces correct resource name; invalid versions rejected.
   - Pass: no network required (pure function tests).

13. [ ] Wire runtime secret fetching (fetch only when needed)
   - Success: secrets are fetched only at “deliver” time; no secret values are persisted in session or stored globally.
14. [ ] Test (unit): secret material never enters session store
   - Approach: use fakes to assert session store APIs only receive metadata (machineId, timestamps, approval state).

## 4) Authn/Authz (Cloud Run IAM + defense-in-depth)

15. [ ] Implement IAM authentication requirement for server (verify Google ID token)
   - Success: unauthenticated requests are rejected; audience and issuer validated.
16. [ ] Test (unit): auth middleware logic
   - Approach: inject verifier interface; test “missing token”, “bad token”, “wrong aud” paths.

17. [ ] Retain `X-Agent-Secret` as defense-in-depth (optional but supported by plan)
   - Success: when configured, requests require correct header; header value is fetched from GSM at check time.
   - Note: only enable this in environments where `key-seeker` can provide the header value securely (otherwise keep it unset/disabled).
18. [ ] Test (unit): agent-secret enforcement
   - Cases: missing/wrong header rejected; correct header accepted; no secret values logged.

## 5) Session model (in-memory, metadata-only)

19. [ ] Implement `SessionStore` interface + in-memory v1 store
   - Success: TTL enforced (from `runtime.maxPendingMinutes`, default 10); replace-on-new-request per `machineId`; approvedAt transitions supported.
20. [ ] Test (unit): `SessionStore` behavior
   - Cases: TTL expiry (uses configured `maxPendingMinutes`); replace existing session for same machine; approve only latest; reject approving expired.

21. [ ] Ensure session contains metadata only (no unlock secret)
   - Success: session type contains only metadata (ids, timestamps, state); no field stores passphrases/keys.
22. [ ] Test (unit): structural guarantee
   - Approach: reflect over the session struct fields and fail the test if forbidden field names/types are introduced.

## 6) HTTP API (unlock + poll)

23. [ ] Define v1 endpoints and request/response shapes using `net/http` + `http.ServeMux` (`POST /unlock`, `GET /poll`, `POST /webhooks/telnyx/<token>`, `GET /healthz`)
   - Success: API supports creating a pending session and polling for approval.
   - Success: `/unlock` triggers an outbound SMS challenge to the configured admin phone.
   - Success: Telnyx can deliver inbound SMS replies to a webhook endpoint.
   - Success: errors are consistent and do not leak secrets.
24. [ ] Test (unit): handler contract
   - Cases: unknown machineId rejected; creates session; triggers SMS send; polling before approval returns “pending”; polling after approval returns key.

25. [ ] Enforce allow-listing by `machineId` from config
   - Success: only machines configured in `.keybringer/config.yaml` can request unlock.
26. [ ] Test (unit): machine allow-list
   - Cases: configured machine accepted; missing machine rejected; error does not reveal which IDs exist.

27. [ ] Add abuse resistance basics (rate limiting per machineId, lockout for invalid TOTP)
   - Success: repeated bad attempts cause temporary lockout; behavior is deterministic and configurable.
28. [ ] Test (unit): rate-limit/lockout
   - Cases: exceeds threshold; resets after window; does not lock out valid code.

## 7) Telnyx webhook + TOTP approval

29. [ ] Implement Telnyx webhook verification + safety (signature + replay + idempotency)
   - Success: invalid signatures rejected; timestamps older than allowed window rejected; signature uses `${timestamp}|${raw_body}`.
   - Success: duplicate webhook deliveries (same event `id`) are handled safely (no double-approval side effects).
30. [ ] Test (unit): Telnyx verification + idempotency
   - Cases: valid signature accepted; invalid signature rejected; stale timestamp rejected; whitespace/body changes break signature.
   - Cases: duplicate event `id` does not produce double-approval side effects.

31. [ ] Add a defense-in-depth "unguessable path token" to the Telnyx webhook route with per-session rotation support
   - Success: requests to `/webhooks/telnyx/<token>` require an exact token match.
   - Success: requests to unknown webhook paths fail fast without expensive work and without logging the token.
   - Success: accepts both current and previous tokens during 60-second grace window (configured via `WEBHOOK_PATH_TOKEN_PREVIOUS` + `WEBHOOK_PATH_TOKEN_PREVIOUS_VALID_UNTIL`).
   - Success: `/unlock` handler can rotate the webhook token *before* sending SMS challenge: generate new token, update Telnyx messaging profile, test reachability, then send SMS.
32. [ ] Test (unit): webhook path token + rotation
   - Cases: correct current token accepted; previous token accepted within grace window; previous token rejected after expiry; wrong/missing token rejected; rejection response does not include the token.

   - Test (manual, optional): rotate-before-challenge drill
	- Steps: rotate Telnyx `webhook_url`, verify new endpoint is reachable, send challenge, reply immediately.
	- Pass: reply is accepted at the new token; previous token remains accepted during overlap window.

33. [ ] Implement TOTP verification with correct windowing
   - Success: accepts valid codes within allowed drift; rejects invalid codes; configurable skew.
34. [ ] Test (unit): TOTP vectors
   - Cases: known RFC test vectors (where applicable) and edge-window drift behavior.

35. [ ] Implement approval flow: SMS reply with valid TOTP approves latest pending session
   - Success: valid code transitions session to approved; ties approval to configured admin phone.
   - Success: approval requires an explicit `machineId` in the SMS text (so multiple machines are unambiguous).
36. [ ] Test (unit): approval mapping
   - Cases: approval with wrong phone rejected; missing/unknown machineId rejected; approves latest pending for that machine; does not approve expired.

## 8) CLI (`key-bringer` and `key-seeker`) per plan

35. [ ] Implement `key-bringer serve` (wiring only in `cmd/`, logic in `internal/`)
   - Success: service starts with config loaded; handlers registered; logs are structured and non-secret.
36. [ ] Test (manual): local run smoke
   - Steps: run service locally with a fake config; hit health endpoint.
   - Pass: service starts; health responds; no secrets printed.

37. [ ] Implement `doctor` commands (`key-bringer doctor`, `key-seeker doctor`)
   - Success: non-destructive checks report missing prerequisites (APIs, secrets pinned, auth reachability).
38. [ ] Test (manual): doctor output
   - Steps: run in a misconfigured env and a configured env.
   - Pass: messages are actionable; no secret leakage.

39. [ ] Implement `key-seeker unlock` (`--totp` and `--monitor`)
   - Success: obtains ID token; fails closed if token cannot be obtained; does not attempt unauth mode.
40. [ ] Test (manual): host agent behavior
   - Steps: run against a dev instance; verify request fails without auth and succeeds with valid auth.

## 9) Provisioning (`key-bringer gcp bootstrap`, scriptless)

41. [ ] Implement `key-bringer gcp bootstrap` as `gcloud` wrapper with `--dry-run`
   - Success: prints deterministic `gcloud` commands; safe to re-run; does not require service account keys.
   - Success: deploy config enforces single-instance v1 defaults (`max-instances=1`, `concurrency=1`).
42. [ ] Test (manual): dry-run
   - Steps: run `--dry-run` and sanity-check command list.
   - Pass: no destructive calls made; output is complete and includes single-instance settings (`max-instances=1`, `concurrency=1`).

43. [ ] Implement `key-bringer init` (guided flow; optional bootstrap; creates GSM secrets)
   - Success: runs `gcp bootstrap`, creates required secrets without logging values, and encourages numeric version pinning into config.
44. [ ] Test (manual): init in a dev project
   - Steps: run in a throwaway GCP project; confirm secrets exist and config pins numeric versions.

## 10) End-to-end validation

45. [ ] E2E flow validation in Cloud Run (private)
   - Success: end-to-end unlock works with IAM auth; public invoker not required.
46. [ ] Test (manual): browser/agent-assisted walkthrough
   - Steps:
     - Deploy Cloud Run with IAM-only ingress.
     - Use an authenticated client to call `/unlock`.
     - Approve via Telnyx SMS + TOTP.
     - Poll `/poll` and verify key delivered only after approval.
   - Pass: key is never logged; key-seeker can unlock; failures are safe.
