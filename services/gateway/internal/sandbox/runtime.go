// Package sandbox runs stdio MCP servers and bridges their stdio transport to
// the gateway. The execution boundary is pluggable (Runtime): an unsandboxed
// `exec` backend for local dev, and gVisor / Firecracker (Kata) backends for
// production on Linux (research R1) — the HC-3 isolation layer.
package sandbox

import (
	"context"
	"io"
)

// Spec describes a stdio MCP server to launch.
type Spec struct {
	Command string
	Args    []string
	Env     []string // KEY=VALUE entries appended to the base environment
}

// Instance is a launched process with its stdio bridged.
type Instance struct {
	Stdin  io.WriteCloser
	Stdout io.Reader
	Stop   func() error
}

// Runtime launches a Spec inside an isolation boundary.
type Runtime interface {
	Launch(ctx context.Context, spec Spec) (*Instance, error)
}
