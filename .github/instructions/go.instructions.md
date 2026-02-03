---
applyTo: "**/*.go"
---

# Go implementation guidance

- Before implementing any new feature, read the relevant `.plan/` doc(s) first. If missing/unclear, stop and ask.
- Follow [.agents/Go.md](../../.agents/Go.md) conventions (Cobra, no Viper, `slog`, `context.Context`, error wrapping).
- Never log secrets or include secret values in returned errors.
- Secret Manager access should use pinned numeric versions (no `versions/latest`).
- Prefer small, testable helpers and interfaces in `internal/core/`.
