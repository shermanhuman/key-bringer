# Copilot Instructions (key-bringer)

You are assisting in the KeyBringer repo (Go services + CLI) that integrates with Google Cloud Secret Manager.

## Plan-First Workflow (Required)

- Before implementing any **new feature** (new command/flag, API, auth flow, data model, Secret Manager behavior, deployment posture), you MUST read the relevant `.plan/` doc(s) first.
- If there is no relevant plan section yet, or the plan is ambiguous, STOP and ask the user to confirm/extend the plan before writing code.
- If code or docs in the repo conflict with the plan, treat the plan as the source of truth and call out the mismatch explicitly.

Execution checklist:

- Use `.plan/05-tasklist.md` as the active implementation checklist.
- When completing work, keep tasks/tests in that file accurate and up to date.

## Hard Rules

- Do not generate or persist plaintext secrets. Never add real credentials to code, docs, tests, config, comments, or logs.
- Do not log secrets (including in error messages) and avoid printing full request bodies.
- Secret Manager usage must be deterministic: prefer pinned numeric secret versions; avoid `versions/latest` and aliases.
- Prefer ADC / workload identity; avoid instructions that require downloading/storing service account keys.
- CLI uses Cobra; do **not** introduce Viper.

## HTTP Stack (v1)

- Use the Go standard library HTTP stack: `net/http` + `http.ServeMux` (Go 1.22+ patterns).
- Do **not** introduce Gin (or other web frameworks/routers) unless the relevant `.plan/` docs are updated first.

## Repo conventions

- Keep `cmd/*` thin; put logic in `internal/` packages.
- Use `context.Context` for all I/O.
- Wrap errors with context: `fmt.Errorf("context: %w", err)`.

## Source of truth

- Follow [AGENTS.md](../AGENTS.md), [.agents/Go.md](../.agents/Go.md), and the `.plan/` docs.
