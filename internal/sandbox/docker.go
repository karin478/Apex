package sandbox

import (
	"context"
	"os"
)

// DockerSandbox wraps commands in a docker run container.
type DockerSandbox struct {
	Image       string // default: "ubuntu:22.04"
	MemoryLimit string // default: "2g"
	CPULimit    string // default: "2"
	WorkDir     string // host dir to mount as /workspace
}

func (d *DockerSandbox) Level() Level { return Docker }

func (d *DockerSandbox) Wrap(_ context.Context, binary string, args []string) (string, []string, error) {
	image := d.Image
	if image == "" {
		image = "ubuntu:22.04"
	}
	mem := d.MemoryLimit
	if mem == "" {
		mem = "2g"
	}
	cpu := d.CPULimit
	if cpu == "" {
		cpu = "2"
	}
	workdir := d.WorkDir
	if workdir == "" {
		workdir, _ = os.Getwd()
	}

	dockerArgs := []string{
		"run", "--rm",
		"--network=none",
		"--memory=" + mem,
		"--cpus=" + cpu,
		"-v", workdir + ":/workspace:ro",
		"-w", "/workspace",
		image,
		binary,
	}
	dockerArgs = append(dockerArgs, args...)

	return "docker", dockerArgs, nil
}
