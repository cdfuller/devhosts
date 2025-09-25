## devhosts (Go) — Product Requirements Document

### 1) Overview

devhosts is a Go CLI that lets you add/remove/list local bare hostnames and map them to upstreams for Caddy. It:
- Writes those hostnames to /etc/hosts (IPv4 only) within a managed block.
- Regenerates a dedicated include Caddyfile (devhosts.caddy) with one site block per host.
- Reloads Caddy via `caddy reload --config <BaseCaddyfile> --adapter caddyfile`.
- Optionally enables tls internal per host (HTTP→HTTPS handled by Caddy’s auto-HTTPS).

### 2) In-Scope (v1)
- macOS (arm64/amd64) support.
- Bare hostname management (no dots): e.g., user, staff, admin.
- /etc/hosts management with:
  - Single managed block
  - Backups + automatic restore on write failure
  - Sudo escalation when needed
  - No IPv6 writes (per your choice)
- Caddy integration via files:
  - Base Caddyfile path is configurable (default ~/.Caddyfile for you)
  - Separate include file fully owned by devhosts (default ~/.devhosts.caddy)
  - Base must contain `import ~/.devhosts.caddy` (set up once by you); devhosts errors out if missing
  - One site block per host (no aggregation)
  - Optional tls internal per host; no automatic trust install
- Local-only upstreams (e.g., http://localhost:8000, http://127.0.0.1:9000)
- No multiple upstreams/load-balancing in v1
- No “doctor”, no dry-run in v1 (planned later)
- Open-source (MIT)

### 3) Out of Scope (v1)
- Windows, Linux (can come later)
- Admin API JSON patching
- dnsmasq/wildcards
- Profiles/environments, per-project config, direnv
- JSON output, shell completions
- Non-local upstreams

### 4) User Stories
- As a dev, I can add `user:8000 admin:8000 staff:9000` (some with TLS) in one command and immediately use those names in my browser.
- As a dev, I can remove a hostname and it stops resolving/serving.
- As a dev, I can list the current managed hostnames and their upstreams at a glance.
- As a dev, I can apply (re-sync) after manually editing config.

### 5) CLI Commands (v1)

**devhosts list**
- Prints hostnames with upstream and TLS flag from devhosts.json.
- Exits 0 even if nothing is configured (prints “No hosts managed”).

**devhosts add <host[:port]> [<host[:port]> ...] [--tls | --no-tls]**
- Normalizes hostnames (lowercase, trims whitespace); rejects dotted names.
- Requires explicit upstream per host (no global default).
- Updates devhosts.json.
- Ensures /etc/hosts contains the managed block with all hosts on 127.0.0.1.
- Regenerates ~/.devhosts.caddy with one block per host:

```caddyfile
user {
  # optional when TLS enabled
  tls internal

  reverse_proxy http://localhost:8000
}
```

- If TLS is enabled for a host, we rely on Caddy’s auto-HTTPS for HTTP→HTTPS redirect (meets your “redirect” requirement without extra rules).
- Runs `caddy reload --config "<BaseCaddyfile>" --adapter caddyfile`.
- Fails if any hostname already exists elsewhere in the base Caddyfile (to avoid shadowing).

**devhosts remove <host> [<host> ...]**
- Removes from devhosts.json.
- Rewrites managed block in /etc/hosts (removes host).
- Regenerates ~/.devhosts.caddy.
- Reloads Caddy.
- If the host isn’t managed, prints a helpful warning and continues.

**devhosts apply**
- Rebuilds /etc/hosts managed block and ~/.devhosts.caddy from devhosts.json.
- Reloads Caddy.
- Useful after manual edits of devhosts.json.

**devhosts path**
- Prints the resolved paths used:
  - devhosts.json
  - Base Caddyfile
  - Include Caddyfile (~/.devhosts.caddy)

**Global flags**
- `--config <path>`: overrides devhosts.json path (default: ~/devhosts.json)
- `--caddyfile <path>`: path to base Caddyfile (default: ~/.Caddyfile)

### 6) Config Files

**devhosts.json (single source of truth)**

```json
{
  "version": 1,
  "hosts": [
    { "name": "user",  "upstream": "http://localhost:8000", "tls": true  },
    { "name": "staff", "upstream": "http://127.0.0.1:9000"               },
    { "name": "admin", "upstream": "http://localhost:8000", "tls": false }
  ],
  "base_caddyfile": "/Users/codyfuller/.Caddyfile",
  "include_caddyfile": "/Users/codyfuller/.devhosts.caddy"
}
```

- `name`: bare hostname (no dots).
- `upstream`: required, must be local (enforce localhost/127.0.0.1).
- `tls`: optional; default false.

**Base Caddyfile (your file)**

Must include (once, during setup):

```caddyfile
import /Users/codyfuller/.devhosts.caddy
```

devhosts does not auto-insert this; it errors with instructions if missing.

**Include Caddyfile (owned by devhosts)**

Regenerated on each add/remove/apply. One site block per host. If tls: true, we include `tls internal`. No aggregation.

### 7) /etc/hosts Management
- Writes a single block:

```
# >>> devhosts BEGIN
127.0.0.1    user staff admin
# <<< devhosts END
```

- No IPv6 lines (per your choice).
- Atomic write: create temp, validate, replace.
- Backup before write: `/etc/hosts.devhosts.bak-YYYYmmdd-HHMMSS`.
- Auto-restore if the write fails.
- Never touches lines outside the managed block.
- Sudo: if not writable, the tool re-execs with sudo (after a Y/N prompt on first run).

### 8) Caddy Reload & Safety
- `caddy reload --config "<BaseCaddyfile>" --adapter caddyfile`
- If reload fails:
  - Restore ~/.devhosts.caddy from previous content
  - Restore /etc/hosts from backup (to avoid drift)
  - Print the adapter/parse error with a hint to run `caddy adapt --config "<BaseCaddyfile>" --adapter caddyfile` for debugging
- We do not auto-run `brew services restart caddy` (per your answer).

### 9) Validation Rules
- Hostnames: `[a-z0-9-]+`, no dots, must start/end with alphanumeric.
- Upstreams: must be `http://localhost:<port>` or `http://127.0.0.1:<port>`.
- If a hostname appears anywhere else in the base Caddyfile, fail with a clear message (avoid duplicate site definitions).

### 10) Examples

Add three hosts (TLS for user/admin):

```bash
devhosts add user:8000 admin:8000 --tls
devhosts add staff:9000 --no-tls
```

- /etc/hosts managed block becomes `127.0.0.1 user admin staff`
- ~/.devhosts.caddy:

```caddyfile
user {
  tls internal
  reverse_proxy http://localhost:8000
}

admin {
  tls internal
  reverse_proxy http://localhost:8000
}

staff {
  reverse_proxy http://127.0.0.1:9000
}
```

Remove one:

```bash
devhosts remove staff
```

Re-apply after hand-editing devhosts.json:

```bash
devhosts apply
```

### 11) Milestones & Acceptance

**v0.1 (MVP)**
- Commands: list / add / remove / apply / path
- /etc/hosts managed block (IPv4), backups, sudo escalation
- Separate include Caddyfile regenerated per state
- Caddy reload via CLI; rollbacks on failure
- Fail-fast if base import missing or hostname conflicts detected

**Acceptance Tests**
- Adding hosts updates /etc/hosts, regenerates include file, and reloads Caddy successfully.
- TLS hosts serve HTTPS with tls internal (browser trust left to user).
- Removing hosts cleans up both files and reloads.
- Re-running apply is idempotent (no diff).
- Backup/restore triggers on simulated write error.

**v0.2 (Next)**
- --dry-run (show diff for hosts + include file)
- Linux support
- Optional doctor command (syntax check, permissions, caddy presence)
