# CLI Plan

## Goals

- Make setup repeatable and non-mysterious.
- Keep the operational surface area small.
- Prefer a single “happy path” onboarding flow.

## CLI shape

Adopt Cobra for both binaries:

- `key-bringer` (admin/operator CLI + server runner)
- `key-seeker` (host agent CLI)

Config approach:

- Cobra only; no Viper.
- Repo config (if used) is parsed directly from YAML into structs and fails closed on unknown fields.

## Proposed commands (v1)

### key-bringer

- `key-bringer serve`
  - Starts the Cloud Run HTTP service.
  - Reads config from environment or repo config (final decision in config plan).

- `key-bringer init`
  - Guided setup:
    - validates `gcloud` presence and auth
    - runs `gcp bootstrap` (optional but recommended)
    - creates required GSM secrets (interactive values piped via stdin)
    - outputs a deploy command or writes Cloud Build substitutions

- `key-bringer gcp bootstrap`
  - Opinionated provisioning wrapper around `gcloud`:
    - enable APIs
    - create service accounts
    - assign least-privilege IAM for GSM access
    - configure Cloud Run service identity + invoker policy
    - configure Artifact Registry (if using it)
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
- key-seeker must obtain an ID token (Google auth library) and fails closed if it cannot.
- `X-Agent-Secret` header can remain as defense-in-depth, but it is not the primary access control.
