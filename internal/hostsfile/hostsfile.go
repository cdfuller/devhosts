package hostsfile

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cdfuller/devhosts/internal/filesystem"
	"github.com/cdfuller/devhosts/internal/state"
	"github.com/cdfuller/devhosts/internal/system"
)

const (
	blockStart = "# >>> devhosts BEGIN"
	blockEnd   = "# <<< devhosts END"
	newline    = "\n"
)

// Clock abstracts time for deterministic testing.
type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// Manager rewrites the managed hosts block while preserving user edits outside it.
type Manager struct {
	FS    filesystem.FS
	Clock Clock
}

// NewManager creates a Manager backed by the provided filesystem.
func NewManager(fs filesystem.FS) Manager {
	if fs == nil {
		fs = filesystem.OS{}
	}
	return Manager{FS: fs, Clock: realClock{}}
}

// Apply ensures the managed block reflects the provided host list.
func (m Manager) Apply(path string, hosts []state.Host) (ApplyResult, error) {
	if m.FS == nil {
		m.FS = filesystem.OS{}
	}
	if m.Clock == nil {
		m.Clock = realClock{}
	}

	resolved, err := filesystem.ExpandUser(path)
	if err != nil {
		return ApplyResult{}, err
	}

	original, readErr := m.FS.ReadFile(resolved)
	if readErr != nil && !errors.Is(readErr, fs.ErrNotExist) {
		return ApplyResult{}, system.WrapPermission("read", resolved, readErr)
	}

	withoutBlock, _ := stripManagedBlock(string(original))
	hostnames := extractHostnames(hosts)
	block := buildBlock(hostnames)

	final := withoutBlock
	if block != "" {
		if final != "" && !strings.HasSuffix(final, newline) {
			final += newline
		}
		final += block
	}
	if final != "" && !strings.HasSuffix(final, newline) {
		final += newline
	}

	if string(original) == final {
		return ApplyResult{Changed: false}, nil
	}

	var backupPath string
	if !errors.Is(readErr, fs.ErrNotExist) {
		backupPath = fmt.Sprintf("%s.devhosts.bak-%s", resolved, m.Clock.Now().Format("20060102-150405"))
		if writeErr := m.FS.WriteFile(backupPath, original, 0o644); writeErr != nil {
			return ApplyResult{}, system.WrapPermission("backup", backupPath, writeErr)
		}
	}

	dir := filepath.Dir(resolved)
	if mkdirErr := m.FS.MkdirAll(dir, 0o755); mkdirErr != nil {
		return ApplyResult{}, system.WrapPermission("mkdir", dir, mkdirErr)
	}

	tempPath := fmt.Sprintf("%s.devhosts.tmp-%d", resolved, m.Clock.Now().UnixNano())
	if writeErr := m.FS.WriteFile(tempPath, []byte(final), 0o644); writeErr != nil {
		_ = m.FS.Remove(tempPath)
		return ApplyResult{}, system.WrapPermission("write", tempPath, writeErr)
	}

	if renameErr := m.FS.Rename(tempPath, resolved); renameErr != nil {
		_ = m.FS.Remove(tempPath)
		if backupPath != "" {
			_ = m.FS.WriteFile(resolved, original, 0o644)
		}
		return ApplyResult{}, system.WrapPermission("replace", resolved, renameErr)
	}

	return ApplyResult{Changed: true, BackupPath: backupPath, Path: resolved}, nil
}

// ApplyResult contains metadata about a hosts file update attempt.
type ApplyResult struct {
	Changed    bool
	BackupPath string
	Path       string
}

// Restore reverts the hosts file using the supplied result metadata.
func (m Manager) Restore(res ApplyResult) error {
	if res.Path == "" {
		return nil
	}
	if res.BackupPath == "" {
		// No backup was created; remove managed block by reapplying empty set.
		_, err := m.Apply(res.Path, nil)
		return err
	}
	data, err := m.FS.ReadFile(res.BackupPath)
	if err != nil {
		return system.WrapPermission("read", res.BackupPath, err)
	}
	if err := m.FS.WriteFile(res.Path, data, 0o644); err != nil {
		return system.WrapPermission("restore", res.Path, err)
	}
	return nil
}

func extractHostnames(hosts []state.Host) []string {
	names := make([]string, 0, len(hosts))
	for _, h := range hosts {
		if h.Name != "" {
			names = append(names, h.Name)
		}
	}
	sort.Strings(names)
	return names
}

func buildBlock(names []string) string {
	if len(names) == 0 {
		return ""
	}
	return fmt.Sprintf("%s\n127.0.0.1    %s\n%s\n", blockStart, strings.Join(names, " "), blockEnd)
}

func stripManagedBlock(content string) (string, bool) {
	lines := strings.Split(content, "\n")
	inside := false
	removed := false
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		switch {
		case line == blockStart:
			if inside {
				// Nested start marker: abort to avoid data loss.
				return content, false
			}
			inside = true
			removed = true
			continue
		case line == blockEnd:
			if !inside {
				// Unmatched end marker, keep line to avoid corruption.
				out = append(out, line)
				continue
			}
			inside = false
			continue
		}
		if inside {
			continue
		}
		out = append(out, line)
	}

	if inside {
		// Unterminated block; leave file untouched.
		return content, false
	}

	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}

	if len(out) == 0 {
		return "", removed
	}

	result := strings.Join(out, "\n")
	if result != "" {
		result += "\n"
	}
	return result, removed
}
