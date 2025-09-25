package caddy

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cdfuller/devhosts/internal/cmdutil"
	"github.com/cdfuller/devhosts/internal/filesystem"
	"github.com/cdfuller/devhosts/internal/state"
	"github.com/cdfuller/devhosts/internal/system"
)

// Manager orchestrates Caddy include generation and reloads.
type Manager struct {
	FS     filesystem.FS
	Runner cmdutil.Runner
}

// NewManager constructs a Manager with sensible defaults.
func NewManager(fs filesystem.FS, runner cmdutil.Runner) Manager {
	if fs == nil {
		fs = filesystem.OS{}
	}
	if runner == nil {
		runner = cmdutil.ExecRunner{}
	}
	return Manager{FS: fs, Runner: runner}
}

// GenerateInclude renders the managed include file content.
func (m Manager) GenerateInclude(hosts []state.Host) string {
	if len(hosts) == 0 {
		return ""
	}
	blocks := make([]string, 0, len(hosts))
	for _, h := range hosts {
		lines := []string{
			fmt.Sprintf("%s {", h.Name),
		}
		if h.TLS {
			lines = append(lines, "  tls internal", "")
		}
		lines = append(lines, fmt.Sprintf("  reverse_proxy %s", h.Upstream), "}")
		blocks = append(blocks, strings.Join(lines, "\n"))
	}
	return strings.Join(blocks, "\n\n") + "\n"
}

// UpdateInclude writes the include file atomically and returns the previous contents for rollback.
func (m Manager) UpdateInclude(path string, content string) (UpdateResult, error) {
	resolved, err := filesystem.ExpandUser(path)
	if err != nil {
		return UpdateResult{}, err
	}

	previous, readErr := m.FS.ReadFile(resolved)
	if readErr != nil && !errors.Is(readErr, fs.ErrNotExist) {
		return UpdateResult{}, system.WrapPermission("read", resolved, readErr)
	}

	existed := readErr == nil

	if existed && string(previous) == content {
		return UpdateResult{Changed: false, Path: resolved, Previous: previous, Existed: true}, nil
	}

	dir := filepath.Dir(resolved)
	if err := m.FS.MkdirAll(dir, 0o755); err != nil {
		return UpdateResult{}, system.WrapPermission("mkdir", dir, err)
	}

	tempPath := fmt.Sprintf("%s.devhosts.tmp-%d", resolved, time.Now().UnixNano())
	if err := m.FS.WriteFile(tempPath, []byte(content), 0o644); err != nil {
		_ = m.FS.Remove(tempPath)
		return UpdateResult{}, system.WrapPermission("write", tempPath, err)
	}

	if err := m.FS.Rename(tempPath, resolved); err != nil {
		_ = m.FS.Remove(tempPath)
		return UpdateResult{}, system.WrapPermission("replace", resolved, err)
	}

	return UpdateResult{Changed: true, Path: resolved, Previous: previous, Existed: existed}, nil
}

// RestoreInclude attempts to put the include file back to its previous bytes.
func (m Manager) RestoreInclude(res UpdateResult) error {
	if res.Path == "" {
		return nil
	}
	if !res.Existed {
		// File did not exist previously; remove it if present.
		if err := m.FS.Remove(res.Path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return system.WrapPermission("remove", res.Path, err)
		}
		return nil
	}
	if err := m.FS.WriteFile(res.Path, res.Previous, 0o644); err != nil {
		return system.WrapPermission("restore", res.Path, err)
	}
	return nil
}

// Reload invokes `caddy reload` with the provided base Caddyfile.
func (m Manager) Reload(ctx context.Context, baseCaddyfile string) (cmdutil.Result, error) {
	resolved, err := filesystem.ExpandUser(baseCaddyfile)
	if err != nil {
		return cmdutil.Result{}, err
	}
	if m.Runner == nil {
		m.Runner = cmdutil.ExecRunner{}
	}
	return m.Runner.Run(ctx, "caddy", "reload", "--config", resolved, "--adapter", "caddyfile")
}

// EnsureBaseReady validates the base Caddyfile contains the include and no conflicting site blocks.
func (m Manager) EnsureBaseReady(basePath, includePath string, hosts []state.Host) error {
	resolvedBase, err := filesystem.ExpandUser(basePath)
	if err != nil {
		return err
	}
	data, err := m.FS.ReadFile(resolvedBase)
	if err != nil {
		return system.WrapPermission("read", resolvedBase, err)
	}

	baseContent := string(data)
	if err := ensureImportPresent(baseContent, includePath); err != nil {
		return fmt.Errorf("base caddyfile %s invalid: %w", resolvedBase, err)
	}
	if usesTildeImport(baseContent, includePath) {
		alt := altHomeToken(includePath)
		return fmt.Errorf("base caddyfile %s imports %s using ~; replace with absolute path %s", resolvedBase, alt, includePath)
	}
	if conflicts := detectConflicts(string(data), hosts); len(conflicts) > 0 {
		sort.Strings(conflicts)
		return fmt.Errorf("base caddyfile %s already defines hosts: %s", resolvedBase, strings.Join(conflicts, ", "))
	}
	return nil
}

func ensureImportPresent(content, includePath string) error {
	tokens := []string{includePath}
	if alt := altHomeToken(includePath); alt != "" {
		tokens = append(tokens, alt)
	}
	for _, token := range tokens {
		if strings.Contains(content, "import \""+token+"\"") || strings.Contains(content, "import "+token) {
			return nil
		}
	}
	return fmt.Errorf("missing required import for %s", includePath)
}

func altHomeToken(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	if strings.HasPrefix(path, home+"/") {
		return "~" + strings.TrimPrefix(path, home)
	}
	return ""
}

func usesTildeImport(content, includePath string) bool {
	alt := altHomeToken(includePath)
	if alt == "" {
		return false
	}
	quoted := fmt.Sprintf("\"%s\"", alt)
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "import") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 2 {
			continue
		}
		target := fields[1]
		if target == alt || target == quoted {
			return true
		}
	}
	return false
}

func detectConflicts(content string, hosts []state.Host) []string {
	if len(hosts) == 0 {
		return nil
	}
	lines := strings.Split(content, "\n")
	lookup := make(map[string]struct{}, len(hosts))
	for _, h := range hosts {
		if h.Name != "" {
			lookup[h.Name] = struct{}{}
		}
	}
	conflicts := make([]string, 0)
	for _, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		for name := range lookup {
			if trimmed == name+"{" || strings.HasPrefix(trimmed, name+" ") || strings.HasPrefix(trimmed, name+"{") {
				conflicts = append(conflicts, name)
			}
		}
	}
	return unique(conflicts)
}

func unique(values []string) []string {
	if len(values) == 0 {
		return values
	}
	sort.Strings(values)
	out := values[:0]
	var prev string
	for _, v := range values {
		if v != prev {
			out = append(out, v)
			prev = v
		}
	}
	return out
}

// UpdateResult captures include file update metadata.
type UpdateResult struct {
	Changed  bool
	Path     string
	Previous []byte
	Existed  bool
}
