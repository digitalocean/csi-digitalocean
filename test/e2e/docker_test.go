package e2e

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

const dockerHost = "docker.io/"

type containerParams struct {
	image       string
	cmd         []string
	env         []string
	binds       map[string]string
	stopSignal  string
	stopTimeout time.Duration
}

// runContainer runs a container. It makes sure that any previously run
// container under the same name is deleted first and tries to ensure that the
// running container is removed after exection. Stdout/stderr is shown during
// the execution of the container.
func runContainer(ctx context.Context, p containerParams) (retErr error) {
	cli, err := client.NewEnvClient()
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %s", err)
	}

	// Find and remove any previous container.
	conts, err := cli.ContainerList(ctx, types.ContainerListOptions{
		All: true,
	})
	if err != nil {
		return fmt.Errorf("failed to list containers: %s", err)
	}

ContList:
	for _, cont := range conts {
		for _, name := range cont.Names {
			if strings.TrimPrefix(name, "/") == e2eContainerName {
				err := cli.ContainerRemove(ctx, cont.ID, types.ContainerRemoveOptions{
					Force: true,
				})
				if err != nil {
					return fmt.Errorf("failed to remove previous container: %s", err)
				}
				break ContList
			}
		}
	}

	// Check if image exists locally.
	summaries, err := cli.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list images: %s", err)
	}
	var imageExists bool
Summaries:
	for _, summary := range summaries {
		for _, repoTag := range summary.RepoTags {
			if imagesEqual(p.image, repoTag) {
				imageExists = true
				break Summaries
			}
		}
	}
	if !imageExists {
		// Pull image.
		pull, err := cli.ImagePull(ctx, p.image, types.ImagePullOptions{})
		if err != nil {
			return fmt.Errorf("failed to pull image %q: %s", p.image, err)
		}
		defer pull.Close()
		if _, err := io.Copy(os.Stdout, pull); err != nil {
			return fmt.Errorf("failed to write image pull output: %s", err)
		}
	}

	// Create and start container.
	var mounts []mount.Mount
	for source, target := range p.binds {
		mounts = append(mounts, mount.Mount{
			Type:     mount.TypeBind,
			Source:   source,
			Target:   target,
			ReadOnly: true,
		})
	}

	stopTimeoutSecs := int(p.stopTimeout.Seconds())
	cont, err := cli.ContainerCreate(ctx,
		&container.Config{
			Image:        p.image,
			Tty:          true,
			AttachStdout: true,
			AttachStderr: true,
			Env:          p.env,
			Cmd:          p.cmd,
			StopSignal:   p.stopSignal,
			StopTimeout:  &stopTimeoutSecs,
		},
		&container.HostConfig{
			AutoRemove: true,
			Mounts:     mounts,
		},
		&network.NetworkingConfig{},
		e2eContainerName,
	)
	if err != nil {
		return fmt.Errorf("failed to create container: %s", err)
	}

	err = cli.ContainerStart(ctx, cont.ID, types.ContainerStartOptions{})
	if err != nil {
		return fmt.Errorf("failed to start container: %s", err)
	}

	// Use a context for ContainerLogs independent of the passed in context.
	// This allows us to finish streaming the logs even when the parent context
	// gets canceled.
	logCtx, logCancel := context.WithCancel(context.Background())
	var isDeleted bool
	var r io.ReadCloser
	// Defer-stop container gracefully if needed.
	defer func() {
		defer logCancel()
		if r != nil {
			// Closing the reader terminates io.Copy below.
			defer r.Close()
		}
		if isDeleted {
			// isDeleted is set when the container stopped regularly, which
			// means there is nothing for us to clean up.
			return
		}

		fmt.Println("Stopping container")
		// A nil timeout means we use the timeout configured on the container
		// Config.
		err := cli.ContainerStop(context.Background(), cont.ID, nil)
		if err != nil {
			retErr = utilerrors.NewAggregate([]error{retErr, fmt.Errorf("failed to stop container: %s", err)})
			return
		}
		fmt.Println("Container stopped")
	}()

	r, err = cli.ContainerLogs(logCtx, cont.ID, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		// Follow is essential or otherwise the returned reader will close as
		// soon as it runs out of data, even if only temporarily.
		Follow: true,
	})
	if err != nil {
		return fmt.Errorf("failed to open container logs: %s", err)
	}

	// Show logs while container is running.
	go func() {
		_, err = io.Copy(os.Stdout, r)
		if err != nil {
			// io.Copy will error with context.Canceled if the context on
			// ContainerLogs gets canceled.
			retErr = utilerrors.NewAggregate([]error{retErr, fmt.Errorf("failed to copy container logs: %s", err)})
		}
	}()

	// Wait for container completion.
	_, err = cli.ContainerWait(ctx, cont.ID)
	if err != nil {
		return fmt.Errorf("failed to wait for container: %s", err)
	}
	isDeleted = true

	return nil
}

func imagesEqual(i1, i2 string) bool {
	return strings.TrimPrefix(i1, dockerHost) == strings.TrimPrefix(i2, dockerHost)
}
