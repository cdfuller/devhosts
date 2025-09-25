package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cdfuller/devhosts/internal/filesystem"
	"github.com/cdfuller/devhosts/internal/state"
)

func TestLoadCreatesDefaultsWhenMissing(t *testing.T) {
	dir := t.TempDir()
	loader := NewLoader(filesystem.OS{})
	opts := LoadOptions{ConfigPath: filepath.Join(dir, "devhosts.json")}
	loaded, err := loader.Load(opts)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if loaded.Snapshot.Version != 1 {
		t.Fatalf("expected version 1, got %d", loaded.Snapshot.Version)
	}
	if loaded.Snapshot.BaseCaddyfile == "" || loaded.Snapshot.IncludeCaddyfile == "" {
		t.Fatalf("expected default caddy paths to be populated")
	}
}

func TestSaveAndReload(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "devhosts.json")
	loader := NewLoader(filesystem.OS{})

	snap := state.Snapshot{
		Version:          1,
		Hosts:            []state.Host{{Name: "user", Upstream: "http://localhost:8000"}},
		BaseCaddyfile:    filepath.Join(dir, "Caddyfile"),
		IncludeCaddyfile: filepath.Join(dir, "devhosts.caddy"),
	}
	if err := loader.Save(configPath, snap); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	loaded, err := loader.Load(LoadOptions{ConfigPath: configPath})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(loaded.Snapshot.Hosts) != 1 || loaded.Snapshot.Hosts[0].Name != "user" {
		t.Fatalf("expected host to persist, got %+v", loaded.Snapshot.Hosts)
	}
}

func TestOverrides(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "devhosts.json")
	if err := os.WriteFile(configPath, []byte(`{"version":1,"hosts":[],"base_caddyfile":"/base","include_caddyfile":"/include"}`), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	loader := NewLoader(filesystem.OS{})
	loaded, err := loader.Load(LoadOptions{
		ConfigPath:               configPath,
		BaseCaddyfileOverride:    "/override/base",
		IncludeCaddyfileOverride: "/override/include",
	})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if loaded.Snapshot.BaseCaddyfile != "/override/base" {
		t.Fatalf("expected override applied, got %s", loaded.Snapshot.BaseCaddyfile)
	}
	if loaded.Snapshot.IncludeCaddyfile != "/override/include" {
		t.Fatalf("expected include override applied, got %s", loaded.Snapshot.IncludeCaddyfile)
	}
}
