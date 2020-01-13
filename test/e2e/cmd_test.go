/*
Copyright 2020 DigitalOcean

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
