package sandbox

import (
	"context"
	"os/exec"
	"strconv"
)

// ContainerRuntime runs the stdio MCP server inside a container. The isolation
// boundary is the OCI runtime — `runsc` (gVisor) or `kata` (Firecracker microVM)
// — combined with network/resource confinement. Requires a Docker daemon with
// the chosen runtime registered (a Linux VM on macOS; see docs/local-sandbox.md).
//
// The container is launched with default-deny egress (`--network none`), a
// read-only rootfs, all capabilities dropped, no-new-privileges, and CPU/mem/pid
// limits — covering much of T049 (egress) and T050 (limits) via container flags.
type ContainerRuntime struct {
	DockerBin  string // default "docker"
	Image      string // base image with the server's language runtime
	OCIRuntime string // runsc | kata | runc
	Network    string // default "none" (default-deny egress)
	Memory     string // default "512m"
	PidsLimit  int    // default 256
}

func (r ContainerRuntime) bin() string {
	if r.DockerBin != "" {
		return r.DockerBin
	}
	return "docker"
}

// buildArgs constructs the `docker run` arguments for spec.
func (r ContainerRuntime) buildArgs(spec Spec) []string {
	network := r.Network
	if network == "" {
		network = "none"
	}
	memory := r.Memory
	if memory == "" {
		memory = "512m"
	}
	pids := r.PidsLimit
	if pids == 0 {
		pids = 256
	}

	args := []string{"run", "-i", "--rm"}
	if r.OCIRuntime != "" {
		args = append(args, "--runtime", r.OCIRuntime)
	}
	args = append(args,
		"--network", network,
		"--memory", memory,
		"--pids-limit", strconv.Itoa(pids),
		"--read-only",
		"--tmpfs", "/tmp",
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges",
	)
	for _, e := range spec.Env {
		args = append(args, "-e", e)
	}
	args = append(args, r.Image, spec.Command)
	return append(args, spec.Args...)
}

// Launch starts the container and returns its bridged stdio.
func (r ContainerRuntime) Launch(ctx context.Context, spec Spec) (*Instance, error) {
	cmd := exec.CommandContext(ctx, r.bin(), r.buildArgs(spec)...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &Instance{
		Stdin:  stdin,
		Stdout: stdout,
		Stop: func() error {
			_ = stdin.Close()
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return nil
		},
	}, nil
}
