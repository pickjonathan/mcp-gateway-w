package sandbox

import (
	"strings"
	"testing"
)

func TestContainerRuntime_BuildArgs(t *testing.T) {
	r := ContainerRuntime{Image: "acme/mcp-sandbox:dev", OCIRuntime: "runsc"}
	args := r.buildArgs(Spec{
		Command: "npx",
		Args:    []string{"-y", "@modelcontextprotocol/server-sequential-thinking"},
		Env:     []string{"FOO=bar"},
	})
	got := strings.Join(args, " ")

	for _, want := range []string{
		"run -i --rm",
		"--runtime runsc",
		"--network none",
		"--memory 512m",
		"--pids-limit 256",
		"--read-only",
		"--cap-drop ALL",
		"--security-opt no-new-privileges",
		"-e FOO=bar",
		"acme/mcp-sandbox:dev npx -y @modelcontextprotocol/server-sequential-thinking",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("args missing %q\n  got: %s", want, got)
		}
	}
}
