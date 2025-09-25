# devhosts

`devhosts` is a Go CLI that keeps local hostnames and Caddy in sync. It automates the busywork of mapping bare names (e.g., `user`, `admin`) to local services by safely rewriting `/etc/hosts`, regenerating a dedicated Caddy include file, and reloading Caddy. The tool solves the drift that happens when you juggle host entries, TLS, and reverse proxies by hand.

## What It Solves
- **Single source of truth** – Manage every hostname, upstream URL, and TLS flag in `devhosts.json` rather than scattered across scripts and configs.
- **Safe `/etc/hosts` edits** – Enforces a managed block with atomic writes, backups, and sudo escalation hints.
- **Caddy integration** – Generates one Caddy site block per hostname and triggers `caddy reload` with rollback on failure.
- **Opinionated constraints** – Bare names only, local upstreams, TLS auto-enabled with `tls internal`, and clear errors when the base Caddyfile conflicts.

## Quick Start
1. Place the `devhosts` binary on your `$PATH` (e.g., `/usr/local/bin/devhosts`).
2. Add `import /Users/<you>/.devhosts.caddy` (absolute path required) to your main Caddyfile.
3. Run `devhosts path` to inspect where the config, base Caddyfile, and include live.
4. Add hosts – TLS is on by default; use `--no-tls` when you need plain HTTP:
   ```bash
   devhosts add staff:8080 admin=127.0.0.1:9090
   ```
5. Verify state and the current mapping:
   ```bash
   devhosts list
   ```

Configuration is stored at `~/devhosts.json` by default and can be overridden via `--config`.

## Development
```bash
# install dependencies and sync go.mod
GOFLAGS="-mod=mod" go mod tidy

# build the CLI
go build -o bin/devhosts ./cmd/devhosts

# run the test suite
go test ./...
```

Key internal packages:
- `internal/config` – load/save snapshots with overrides.
- `internal/state` – shared models and validation rules.
- `internal/hostsfile` – managed block writer with backup/restore helpers.
- `internal/caddy` – include generation, base validation, and reload orchestration.
- `internal/cli` – command parsing, host mutation, and rollback handling.

Before calling commands that touch `/etc/hosts`, ensure you have sudo access. The CLI raises `ErrNeedsSudo` when elevation is required.
