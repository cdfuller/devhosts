package hostsfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cdfuller/devhosts/internal/filesystem"
	"github.com/cdfuller/devhosts/internal/state"
)

type fixedClock struct{ t time.Time }

func (f fixedClock) Now() time.Time { return f.t }

func TestApplyWritesManagedBlock(t *testing.T) {
	dir := t.TempDir()
	hostsPath := filepath.Join(dir, "hosts")
	seed := "127.0.0.1 localhost\n"
	if err := os.WriteFile(hostsPath, []byte(seed), 0o644); err != nil {
		t.Fatalf("write seed: %v", err)
	}

	mgr := NewManager(filesystem.OS{})
	mgr.Clock = fixedClock{t: time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)}

	res, err := mgr.Apply(hostsPath, []state.Host{{Name: "user", Upstream: "http://localhost:8000"}})
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if !res.Changed {
		t.Fatalf("expected change to be detected")
	}
	data, err := os.ReadFile(hostsPath)
	if err != nil {
		t.Fatalf("read hosts: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "# >>> devhosts BEGIN") || !strings.Contains(content, "user") {
		t.Fatalf("managed block missing: %s", content)
	}
	if res.BackupPath == "" {
		t.Fatalf("expected backup path to be recorded")
	}
	backup, err := os.ReadFile(res.BackupPath)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(backup) != seed {
		t.Fatalf("backup should preserve original content")
	}
}

func TestApplyRemovesBlockWhenEmpty(t *testing.T) {
	dir := t.TempDir()
	hostsPath := filepath.Join(dir, "hosts")
	initial := "# >>> devhosts BEGIN\n127.0.0.1    user\n# <<< devhosts END\n"
	if err := os.WriteFile(hostsPath, []byte(initial), 0o644); err != nil {
		t.Fatalf("write initial: %v", err)
	}

	mgr := NewManager(filesystem.OS{})
	if _, err := mgr.Apply(hostsPath, nil); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	data, err := os.ReadFile(hostsPath)
	if err != nil {
		t.Fatalf("read hosts: %v", err)
	}
	if strings.Contains(string(data), "devhosts BEGIN") {
		t.Fatalf("expected managed block to be removed, got %s", string(data))
	}
}
