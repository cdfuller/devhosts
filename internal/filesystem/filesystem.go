package filesystem

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// FS abstracts interactions with the local filesystem to support testing.
type FS interface {
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte, perm fs.FileMode) error
	MkdirAll(path string, perm fs.FileMode) error
	Stat(path string) (fs.FileInfo, error)
	Rename(oldPath, newPath string) error
	Remove(path string) error
}

// OS implements FS using the real operating system.
type OS struct{}

func (OS) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }

func (OS) WriteFile(path string, data []byte, perm fs.FileMode) error {
	if err := os.WriteFile(path, data, perm); err != nil {
		return err
	}
	return nil
}

func (OS) MkdirAll(path string, perm fs.FileMode) error { return os.MkdirAll(path, perm) }

func (OS) Stat(path string) (fs.FileInfo, error) { return os.Stat(path) }

func (OS) Rename(oldPath, newPath string) error { return os.Rename(oldPath, newPath) }

func (OS) Remove(path string) error { return os.Remove(path) }

// ExpandUser replaces a leading ~ with the current user's home directory.
func ExpandUser(path string) (string, error) {
	if path == "" || path[0] != '~' {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if path == "~" {
		return home, nil
	}
	if len(path) > 1 && path[1] != '/' {
		return "", errors.New("cannot expand home for other users")
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
}
