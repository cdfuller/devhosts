package caddy

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cdfuller/devhosts/internal/cmdutil"
	"github.com/cdfuller/devhosts/internal/filesystem"
	"github.com/cdfuller/devhosts/internal/state"
)

type fakeRunner struct {
	err error
}

func (f fakeRunner) Run(ctx context.Context, name string, args ...string) (cmdutil.Result, error) {
	return cmdutil.Result{}, f.err
}

func TestGenerateInclude(t *testing.T) {
	mgr := NewManager(filesystem.OS{}, fakeRunner{})
	content := mgr.GenerateInclude([]state.Host{{Name: "user", Upstream: "http://localhost:8000", TLS: true}, {Name: "staff", Upstream: "http://127.0.0.1:9000"}})
	expected := "user {\n  tls internal\n\n  reverse_proxy http://localhost:8000\n}\n\n" +
		"staff {\n  reverse_proxy http://127.0.0.1:9000\n}\n"
	if content != expected {
		t.Fatalf("unexpected include content:\n%s", content)
	}
}

func TestEnsureBaseReadyDetectsImport(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "Caddyfile")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	include := filepath.Join(home, ".devhosts.test.caddy")
	content := "import " + include + "\n"
	if err := os.WriteFile(base, []byte(content), 0o644); err != nil {
		t.Fatalf("write base: %v", err)
	}
	mgr := NewManager(filesystem.OS{}, fakeRunner{})
	if err := mgr.EnsureBaseReady(base, include, []state.Host{{Name: "user"}}); err != nil {
		t.Fatalf("expected base to be accepted: %v", err)
	}

	if err := os.WriteFile(base, []byte("# empty"), 0o644); err != nil {
		t.Fatalf("rewrite base: %v", err)
	}
	if err := mgr.EnsureBaseReady(base, include, nil); err == nil {
		t.Fatalf("expected missing import to error")
	}

	if err := os.WriteFile(base, []byte("import "+include+"\nuser {\n}\n"), 0o644); err != nil {
		t.Fatalf("rewrite base with conflict: %v", err)
	}
	if err := mgr.EnsureBaseReady(base, include, []state.Host{{Name: "user"}}); err == nil {
		t.Fatalf("expected conflict to error")
	}

	if err := os.WriteFile(base, []byte("import ~/.devhosts.test.caddy\n"), 0o644); err != nil {
		t.Fatalf("rewrite base with tilde: %v", err)
	}
	if err := mgr.EnsureBaseReady(base, include, nil); err == nil {
		t.Fatalf("expected tilde import to error")
	} else if !strings.Contains(err.Error(), "replace with absolute path") {
		t.Fatalf("expected tilde warning, got %v", err)
	}
}

func TestUpdateAndRestoreInclude(t *testing.T) {
	dir := t.TempDir()
	include := filepath.Join(dir, "include.caddy")
	mgr := NewManager(filesystem.OS{}, fakeRunner{})

	res, err := mgr.UpdateInclude(include, "user {\n}\n")
	if err != nil {
		t.Fatalf("UpdateInclude returned error: %v", err)
	}
	data, err := os.ReadFile(include)
	if err != nil {
		t.Fatalf("read include: %v", err)
	}
	if string(data) != "user {\n}\n" {
		t.Fatalf("include not written: %s", string(data))
	}
	if res.Existed {
		t.Fatalf("file should be marked as new")
	}

	if err := mgr.RestoreInclude(res); err != nil {
		t.Fatalf("RestoreInclude returned error: %v", err)
	}
	if _, err := os.Stat(include); !os.IsNotExist(err) {
		t.Fatalf("expected include to be removed on restore, got %v", err)
	}

	if err := os.WriteFile(include, []byte("old"), 0o644); err != nil {
		t.Fatalf("seed include: %v", err)
	}
	res, err = mgr.UpdateInclude(include, "new")
	if err != nil {
		t.Fatalf("UpdateInclude returned error: %v", err)
	}
	if !res.Existed {
		t.Fatalf("expected file to exist previously")
	}
	if err := mgr.RestoreInclude(res); err != nil {
		t.Fatalf("RestoreInclude returned error: %v", err)
	}
	restored, err := os.ReadFile(include)
	if err != nil {
		t.Fatalf("read include: %v", err)
	}
	if strings.TrimSpace(string(restored)) != "old" {
		t.Fatalf("expected include to be restored, got %s", string(restored))
	}
}
