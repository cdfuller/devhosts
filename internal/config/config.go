package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/cdfuller/devhosts/internal/filesystem"
	"github.com/cdfuller/devhosts/internal/state"
)

// LoadOptions captures user-provided path overrides.
type LoadOptions struct {
	ConfigPath               string
	BaseCaddyfileOverride    string
	IncludeCaddyfileOverride string
}

// Loaded encapsulates a parsed snapshot and the resolved config path.
type Loaded struct {
	Snapshot state.Snapshot
	Path     string
}

// Loader reads and writes devhosts configuration files.
type Loader struct {
	FS filesystem.FS
}

// NewLoader creates a Loader backed by the provided filesystem.
func NewLoader(fs filesystem.FS) Loader {
	return Loader{FS: fs}
}

// Load reads devhosts.json (creating defaults if missing) and validates the snapshot.
func (l Loader) Load(opts LoadOptions) (Loaded, error) {
	if l.FS == nil {
		l.FS = filesystem.OS{}
	}

	configPath, err := resolveConfigPath(opts.ConfigPath)
	if err != nil {
		return Loaded{}, err
	}

	snapshot, err := l.readSnapshot(configPath)
	if errors.Is(err, fs.ErrNotExist) {
		snapshot, err = defaultSnapshot()
		if err != nil {
			return Loaded{}, err
		}
	} else if err != nil {
		return Loaded{}, fmt.Errorf("read config: %w", err)
	}

	if opts.BaseCaddyfileOverride != "" {
		path, err := filesystem.ExpandUser(opts.BaseCaddyfileOverride)
		if err != nil {
			return Loaded{}, fmt.Errorf("resolve base caddyfile: %w", err)
		}
		snapshot.BaseCaddyfile = path
	}
	if opts.IncludeCaddyfileOverride != "" {
		path, err := filesystem.ExpandUser(opts.IncludeCaddyfileOverride)
		if err != nil {
			return Loaded{}, fmt.Errorf("resolve include caddyfile: %w", err)
		}
		snapshot.IncludeCaddyfile = path
	}

	if err := state.ValidateSnapshot(snapshot); err != nil {
		return Loaded{}, err
	}

	return Loaded{Snapshot: snapshot, Path: configPath}, nil
}

// Save writes the snapshot back to disk with stable formatting.
func (l Loader) Save(path string, snapshot state.Snapshot) error {
	if l.FS == nil {
		l.FS = filesystem.OS{}
	}
	if err := state.ValidateSnapshot(snapshot); err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := l.FS.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("ensure config dir: %w", err)
	}

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}

	return l.FS.WriteFile(path, append(data, '\n'), 0o600)
}

func (l Loader) readSnapshot(path string) (state.Snapshot, error) {
	data, err := l.FS.ReadFile(path)
	if err != nil {
		return state.Snapshot{}, err
	}
	var snapshot state.Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return state.Snapshot{}, err
	}

	if snapshot.BaseCaddyfile == "" || snapshot.IncludeCaddyfile == "" {
		// Support historical configs missing paths by injecting defaults.
		defaults, err := defaultSnapshot()
		if err != nil {
			return state.Snapshot{}, err
		}
		if snapshot.BaseCaddyfile == "" {
			snapshot.BaseCaddyfile = defaults.BaseCaddyfile
		}
		if snapshot.IncludeCaddyfile == "" {
			snapshot.IncludeCaddyfile = defaults.IncludeCaddyfile
		}
	}

	return snapshot, nil
}

func resolveConfigPath(input string) (string, error) {
	if input == "" {
		def, err := defaultConfigPath()
		if err != nil {
			return "", err
		}
		input = def
	}
	return filesystem.ExpandUser(input)
}

func defaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "devhosts.json"), nil
}

func defaultSnapshot() (state.Snapshot, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return state.Snapshot{}, err
	}
	return state.Snapshot{
		Version:          1,
		Hosts:            []state.Host{},
		BaseCaddyfile:    filepath.Join(home, ".Caddyfile"),
		IncludeCaddyfile: filepath.Join(home, ".devhosts.caddy"),
	}, nil
}
