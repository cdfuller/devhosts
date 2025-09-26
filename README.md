# devhosts

`devhosts` is a Go CLI for assigning hostnames to local addresses.

```
localhost:8000 => https://user/
localhost:8000 => https://admin/
localhost:5000 => https://api/
localhost:3000 => https://frontend/
```

This tool keeps local hostnames and Caddy in sync. It cleanly automates the work of mapping bare names (e.g., `user`, `admin`) to local services by safely rewriting `/etc/hosts`, regenerating a dedicated Caddy include file, and reloading Caddy. The tool solves the drift that happens when you juggle host entries, TLS, and reverse proxies by hand.

## What It Solves
- **Single source of truth** – Manage every hostname, upstream URL, and TLS flag in `devhosts.json` rather than scattered across scripts and configs.
- **Safe `/etc/hosts` edits** – Enforces a managed block with atomic writes, backups, and sudo escalation hints.
- **Caddy integration** – Generates one Caddy site block per hostname and triggers `caddy reload` with rollback on failure.
- **Opinionated constraints** – Bare names only, local upstreams, TLS auto-enabled with `tls internal`, and clear errors when the base Caddyfile conflicts.

## Prerequisites
- macOS (arm64/amd64) with sudo access for modifying `/etc/hosts`.
- Caddy installed and configured with an `import ~/.devhosts.caddy` (absolute path) directive in your base Caddyfile.
- A writable `~/.devhosts.caddy` include file that Caddy can reload without manual edits.

## Quick Start
1. Place the `devhosts` binary on your `$PATH` (e.g., `/usr/local/bin/devhosts`).
2. Add `import /Users/<you>/.devhosts.caddy` (absolute path required) to your main Caddyfile.
3. Run `devhosts path` to inspect where the config, base Caddyfile, and include live.
4. Add hosts – TLS is on by default; use `--no-tls` when you need plain HTTP:
   ```bash
   devhosts add user:8000 admin:8000 --tls
   ```
5. Verify state and the current mapping:
   ```bash
   devhosts list
   ```

Host arguments follow `name:port` and default to `http://localhost:<port>`; pass an explicit address (e.g., `staff=http://127.0.0.1:9000`) when the target differs.

## Command Reference
- `devhosts add` – Adds or updates hosts defined as `name[:port]` pairs; combine with `--tls`/`--no-tls` per host list.
- `devhosts remove` – Removes one or more hosts from the managed state and reapplies system changes.
- `devhosts list` – Displays the current hosts, upstreams, and TLS flags stored in the config file.
- `devhosts apply` – Regenerates `/etc/hosts` and the include Caddyfile from the saved config without modifying it.
- `devhosts path` – Prints the resolved locations for the config, base Caddyfile, and include file; accepts `--config`/`--caddyfile` overrides.

## Configuration
Configuration is stored at `~/devhosts.json` by default and can be overridden with `--config`.

```json
{
  "version": 1,
  "hosts": [
    { "name": "user",  "upstream": "http://localhost:8000", "tls": true  },
    { "name": "staff", "upstream": "http://127.0.0.1:9000"               },
    { "name": "admin", "upstream": "http://localhost:8000", "tls": false }
  ],
  "base_caddyfile": "/Users/you/.Caddyfile",
  "include_caddyfile": "/Users/you/.devhosts.caddy"
}
```

- `hosts` – Bare hostnames with local upstreams; TLS defaults to `false` when omitted.
- `base_caddyfile` – The primary Caddyfile that already imports the devhosts include.
- `include_caddyfile` – The file managed by `devhosts`; the CLI overwrites it on each run.

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
- `internal/config` – load and persist devhosts.json with overrides.
- `internal/hostsfile` – manage the `/etc/hosts` block with backup/restore orchestration.
- `internal/caddy` – generate the include file, validate the base, and reload Caddy.
- `internal/system` – handle privilege escalation checks and other OS interactions.

Before calling commands that touch `/etc/hosts`, ensure you have sudo access. The CLI raises `ErrNeedsSudo` when elevation is required.
