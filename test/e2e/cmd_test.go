package e2e

import (
	"context"
	"os"
	"os/exec"
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

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
