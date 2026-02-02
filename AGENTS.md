# KeyBringer Agent Instructions

This repo contains KeyBringer (Cloud Run service) + KeySeeker (host agent) for securely unlocking ZFS at boot.

## Non-negotiables

- **No plaintext secrets**: never paste real credentials or key material into code, logs, docs, issues, or commits.
- **Determinism**: Secret Manager access must use pinned numeric versions (avoid `versions/latest` and aliases).
- **Authentication**: prefer ADC / workload identity; avoid user-managed service account keys.
- **Config**: use Cobra for CLI; do **not** use Viper.

## Where to look first

- Go guidelines: [.agents/Go.md](.agents/Go.md)
- Implementation plan: `.plan/` (overview, security, testing)

## When in doubt

- Prefer explicit behavior over magic defaults.
- Fail closed on parsing/validation errors.
- Call out security-impacting changes clearly (auth, Secret Manager, session state).
