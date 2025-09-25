package cmdutil

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

// Runner executes external commands.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) (Result, error)
}

// Result captures stdout/stderr from a command invocation.
type Result struct {
	Stdout []byte
	Stderr []byte
}

// ExecRunner implements Runner using exec.CommandContext.
type ExecRunner struct{}

// Run executes the command and returns collected stdout/stderr when the command fails.
func (ExecRunner) Run(ctx context.Context, name string, args ...string) (Result, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	res := Result{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}
	if err != nil {
		return res, fmt.Errorf("%s %v: %w", name, args, err)
	}
	return res, nil
}
