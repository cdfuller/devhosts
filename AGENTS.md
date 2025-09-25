# Repository Guidelines

## Project Structure & Module Organization
Set up the Go module at the repo root so `go.mod` covers everything. Place the CLI entrypoint in `cmd/devhosts/main.go`, keeping flag parsing thin. Encapsulate host editing, Caddy integration, config loading, and sudo helpers under `internal/hostsfile`, `internal/caddy`, `internal/config`, and `internal/system` respectively so they can evolve independently. Store reusable fixtures under `testdata/` and keep documentation or sequence diagrams in `docs/` alongside the PRD.

## Build, Test, and Development Commands
Run `go mod tidy` after adding imports to ensure reproducible builds. Compile the CLI with `go build -o bin/devhosts ./cmd/devhosts` and smoke-test via `go run ./cmd/devhosts list`. Execute `go test ./...` for the full suite and use `go test -run '^TestApply' ./internal/...` when iterating on apply logic. Add `go vet ./...` before publishing to catch common mistakes.

## Coding Style & Naming Conventions
Follow idiomatic Go: gofmt’d source, tabs for indentation, exported identifiers only when packages need them. Favor `NewThing` constructors and `Option` structs for configuration, and keep package names lowercase and singular (`hostsfile`, not `Hosts`). When refactoring command behavior, keep CLI verbs aligned with the PRD (`add`, `remove`, `list`, `apply`, `path`).

## Testing Guidelines
Rely on table-driven tests in `_test.go` files colocated with the code. Use temp directories to stub `/etc/hosts` and Caddyfile writes; never touch the real filesystem in unit tests. Target ≥80% coverage on critical packages (`internal/hostsfile`, `internal/caddy`) and document any gaps in TODOs. For flows needing sudo, provide integration tests guarded by build tags such as `//go:build integration`.

## Commit & Pull Request Guidelines
Adopt Conventional Commits (`feat:`, `fix:`, `chore:`) to keep the history searchable once git is initialized. Reference PRD sections or issue IDs in commit bodies for context. PRs should describe the scenario, attach relevant command output (e.g., `go test ./...`), and include screenshots or snippets when Caddy diffs are relevant. Keep PR scope tight—one CLI command or subsystem per review.

## Security & Configuration Tips
Treat paths such as `/etc/hosts`, `~/.Caddyfile`, and `~/.devhosts.caddy` as configurable constants and gate writes behind permission checks. Surface dry-run previews before mutating files, and log exact paths for auditability. Never bundle user secrets; expect contributors to provide their own Caddy credentials and local TLS trust steps.
