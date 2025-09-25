package state

import "testing"

func TestValidateSnapshot(t *testing.T) {
	snap := Snapshot{
		Version:          1,
		BaseCaddyfile:    "/tmp/Caddyfile",
		IncludeCaddyfile: "/tmp/devhosts.caddy",
		Hosts: []Host{{
			Name:     "user",
			Upstream: "http://localhost:8000",
			TLS:      true,
		}, {
			Name:     "admin",
			Upstream: "http://127.0.0.1:9000",
		}},
	}
	if err := ValidateSnapshot(snap); err != nil {
		t.Fatalf("expected snapshot to be valid: %v", err)
	}
	if snap.Hosts[0].Name != "admin" {
		t.Fatalf("expected hosts to be sorted, got %+v", snap.Hosts)
	}
}

func TestValidateSnapshotInvalidHost(t *testing.T) {
	snap := Snapshot{
		Version:          1,
		BaseCaddyfile:    "/tmp/Caddyfile",
		IncludeCaddyfile: "/tmp/devhosts.caddy",
		Hosts: []Host{{
			Name:     "bad.host",
			Upstream: "http://localhost:8000",
		}},
	}
	if err := ValidateSnapshot(snap); err == nil {
		t.Fatalf("expected error for dotted hostname")
	}
}

func TestValidateUpstreamRejectsExternal(t *testing.T) {
	snap := Snapshot{
		Version:          1,
		BaseCaddyfile:    "/tmp/Caddyfile",
		IncludeCaddyfile: "/tmp/devhosts.caddy",
		Hosts: []Host{{
			Name:     "user",
			Upstream: "https://example.com",
		}},
	}
	if err := ValidateSnapshot(snap); err == nil {
		t.Fatalf("expected error for non-local upstream")
	}
}
