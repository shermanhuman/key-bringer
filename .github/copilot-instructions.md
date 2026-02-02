# Copilot Instructions (key-bringer)

You are assisting in the KeyBringer repo (Go services + CLI) that integrates with Google Cloud Secret Manager.

## Hard Rules

- Do not generate or persist plaintext secrets. Never add real credentials to code, docs, tests, config, comments, or logs.
- Do not log secrets (including in error messages) and avoid printing full request bodies.
- Secret Manager usage must be deterministic: prefer pinned numeric secret versions; avoid `versions/latest` and aliases.
- Prefer ADC / workload identity; avoid instructions that require downloading/storing service account keys.
- CLI uses Cobra; do **not** introduce Viper.

## Repo conventions

- Keep `cmd/*` thin; put logic in `internal/` packages.
- Use `context.Context` for all I/O.
- Wrap errors with context: `fmt.Errorf("context: %w", err)`.

## Source of truth

- Follow [AGENTS.md](AGENTS.md), [.agents/Go.md](.agents/Go.md), and the `.plan/` docs.
