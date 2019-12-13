package e2e

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

type cmdParams struct {
	args []string
	envs []string
	dir  string
}

// runCommand executes a command in a way that resembles an interactive usage
// more closely. Specifically, it shows stdout/stderr during command execution.
func runCommand(ctx context.Context, name string, params cmdParams) error {
	cmd := exec.CommandContext(ctx, name, params.args...)
	cmd.Env = append(os.Environ(), params.envs...)
	cmd.Dir = params.dir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %s", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %s", err)
	}

	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start command: %s", err)
	}

	// Copy stdout/stderr output while the command is running. The io.Copy
	// operations will terminate once the command finishes.
	err = utilerrors.AggregateGoroutines(
		func() error {
			_, err := io.Copy(os.Stdout, stdout)
			if err != nil {
				return fmt.Errorf("failed to copy from stdout pipe: %s", err)
			}
			return nil
		},
		func() error {
			_, err := io.Copy(os.Stderr, stderr)
			if err != nil {
				return fmt.Errorf("failed to copy from stderr pipe: %s", err)
			}
			return nil
		},
	)
	if err != nil {
		return fmt.Errorf("failed to copy output: %s", err)
	}

	err = cmd.Wait()
	if err != nil {
		return fmt.Errorf("failed to wait for command: %s", err)
	}

	return nil
}
