package system

import (
	"errors"
	"fmt"
	"os"
)

// ErrNeedsSudo indicates the caller must retry with elevated permissions.
type ErrNeedsSudo struct {
	Op   string
	Path string
	Err  error
}

func (e *ErrNeedsSudo) Error() string {
	return fmt.Sprintf("%s %s: %v (sudo required)", e.Op, e.Path, e.Err)
}

func (e *ErrNeedsSudo) Unwrap() error { return e.Err }

// WrapPermission converts a permission error into ErrNeedsSudo for consistent handling.
func WrapPermission(op, path string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, os.ErrPermission) {
		return &ErrNeedsSudo{Op: op, Path: path, Err: err}
	}
	return err
}

// IsErrNeedsSudo reports whether the error indicates sudo escalation is needed.
func IsErrNeedsSudo(err error) bool {
	var target *ErrNeedsSudo
	return errors.As(err, &target)
}
