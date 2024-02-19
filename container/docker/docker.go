package docker

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
)

type Action = string

const (
	Start Action = "Start"
	Stop  Action = "Stop"
)

type ResultTypes = string

const (
	Success ResultTypes = "Success"
)

type Config struct {
	Name          string
	AttachStdin   bool
	AttachStdout  bool
	AttachStderr  bool
	ExposedPorts  nat.PortSet
	Cmd           []string
	Image         string
	Cpu           float64
	Memory        int64
	Disk          int64
	Env           []string
	RestartPolicy container.RestartPolicyMode
	Runtime
}

type Runtime struct {
	ContainerID string
}

type Result struct {
	Error       error
	Action      string
	ContainerId string
	Result      string
}

type Docker struct {
	Client *client.Client
	Config Config
}

func (d *Docker) Run(ctx context.Context) Result {
	reader, err := d.Client.ImagePull(ctx, d.Config.Image, types.ImagePullOptions{})
	if err != nil {
		msg := fmt.Sprintf("error pulling the docker image: %q", d.Config.Image)
		return Result{
			Error: fmt.Errorf("msg: %s, %w", msg, err),
		}
	}
	io.Copy(os.Stdout, reader)
	rp := container.RestartPolicy{
		Name: d.Config.RestartPolicy,
	}
	r := container.Resources{
		Memory:   d.Config.Memory,
		NanoCPUs: int64(d.Config.Cpu * math.Pow(10, 9)),
	}
	cc := container.Config{
		Image:        d.Config.Image,
		Tty:          false,
		Env:          d.Config.Env,
		ExposedPorts: d.Config.ExposedPorts,
	}
	hc := container.HostConfig{
		RestartPolicy:   rp,
		Resources:       r,
		PublishAllPorts: true,
	}

	resp, err := d.Client.ContainerCreate(ctx, &cc, &hc, nil, nil, d.Config.Name)
	if err != nil {
		msg := fmt.Sprintf("error creating the docker image: %q", d.Config.Image)
		return Result{
			Error: fmt.Errorf("msg: %s, %w", msg, err),
		}
	}

	if err := d.Client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		msg := fmt.Sprintf("error starting the container: %q", resp.ID)
		return Result{
			Error: fmt.Errorf("msg: %s, %w", msg, err),
		}
	}

	d.Config.Runtime.ContainerID = resp.ID

	logs, err := d.Client.ContainerLogs(ctx, resp.ID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		msg := fmt.Sprintf("error fetching the container logs: %q", resp.ID)
		return Result{
			Error: fmt.Errorf("msg: %s, %w", msg, err),
		}
	}

	if _, err := stdcopy.StdCopy(os.Stdout, os.Stderr, logs); err != nil {
		msg := fmt.Sprintf("error copying the docker logs to stdout and stderr")
		return Result{
			Error: fmt.Errorf("msg: %s, %w", msg, err),
		}
	}

	return Result{
		ContainerId: resp.ID,
		Action:      Start,
		Result:      Success,
	}
}

func (d *Docker) Stop(ctx context.Context, id string) Result {
	if err := d.Client.ContainerStop(ctx, id, container.StopOptions{}); err != nil {
		return Result{
			Error: fmt.Errorf("error stopping the container: %w", err),
		}
	}

	if err := d.Client.ContainerRemove(ctx, id, container.RemoveOptions{}); err != nil {
		return Result{
			Error: fmt.Errorf("error removing the container: %w", err),
		}
	}
	return Result{
		Action: Stop,
		Result: Success,
		Error:  nil,
	}
}
