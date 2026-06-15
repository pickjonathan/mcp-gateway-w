package sandbox

import (
	"context"
	"os/exec"

	"github.com/rs/zerolog"
)

// ExecRuntime runs the command directly with NO isolation. Dev/local only —
// it MUST NOT be used for untrusted code in production (HC-3); use the
// gVisor/Firecracker backends there.
type ExecRuntime struct{}

// Launch starts the command and returns its bridged stdio.
func (ExecRuntime) Launch(ctx context.Context, spec Spec) (*Instance, error) {
	cmd := exec.CommandContext(ctx, spec.Command, spec.Args...)
	if len(spec.Env) > 0 {
		cmd.Env = append(cmd.Environ(), spec.Env...)
	}
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

// Select returns the Runtime for the configured backend and base image:
//   - gvisor: container via the runsc OCI runtime (gVisor) — the HC-3 boundary
//   - kata: container via Kata-Containers (Firecracker microVM; needs nested virt)
//   - runc/container: plain container (dev; NOT an isolation boundary for untrusted code)
//   - exec: unsandboxed local process (dev only)
//
// gVisor/Kata require a Docker daemon with that runtime registered — on macOS,
// a Linux VM (see docs/local-sandbox.md). gVisor needs no nested virtualization.
//
// egressNetwork is the Docker network the container joins for egress. Empty keeps
// the default-deny `--network none`; a non-empty value is the explicit allowlist
// (Constitution II) — the sandbox can reach only that network's members. It is
// ignored by the unsandboxed exec backend (which has no container network).
func Select(name, image, egressNetwork string, log zerolog.Logger) Runtime {
	switch name {
	case "gvisor":
		return ContainerRuntime{Image: image, OCIRuntime: "runsc", Network: egressNetwork}
	case "kata":
		return ContainerRuntime{Image: image, OCIRuntime: "kata", Network: egressNetwork}
	case "runc", "container":
		log.Warn().Msg("plain container runtime is NOT an isolation boundary for untrusted code (HC-3)")
		return ContainerRuntime{Image: image, OCIRuntime: "runc", Network: egressNetwork}
	case "exec", "":
		log.Warn().Msg("using UNSANDBOXED exec runtime (dev only) — do NOT run untrusted code")
		return ExecRuntime{}
	default:
		log.Warn().Str("requested", name).Msg("unknown sandbox runtime; falling back to UNSANDBOXED exec (dev only)")
		return ExecRuntime{}
	}
}
