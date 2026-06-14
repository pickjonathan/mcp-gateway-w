package sandbox

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// argsFor renders the docker-run args a ContainerRuntime would use for a probe spec,
// so we can assert on the --network flag the egress allowlist produces.
func argsFor(t *testing.T, rt Runtime) string {
	t.Helper()
	cr, ok := rt.(ContainerRuntime)
	if !ok {
		t.Fatalf("expected ContainerRuntime, got %T", rt)
	}
	return strings.Join(cr.buildArgs(Spec{Command: "server"}), " ")
}

// TestSelect_EgressDefaultIsNone proves the additive allowlist defaults OFF:
// with no egress network configured, the sandbox still runs `--network none`
// (byte-for-byte today's behavior; FR-016 — no existing deployment changes).
func TestSelect_EgressDefaultIsNone(t *testing.T) {
	args := argsFor(t, Select("gvisor", "img", "", zerolog.Nop()))
	if !strings.Contains(args, "--network none") {
		t.Fatalf("default egress MUST be --network none; got: %s", args)
	}
}

// TestSelect_EgressAllowlist proves that when an egress network IS configured,
// the sandbox joins exactly that network (the explicit allowlist — Constitution II)
// and is NOT left on `none`.
func TestSelect_EgressAllowlist(t *testing.T) {
	args := argsFor(t, Select("gvisor", "img", "mcp-sandbox-egress", zerolog.Nop()))
	if !strings.Contains(args, "--network mcp-sandbox-egress") {
		t.Fatalf("expected --network mcp-sandbox-egress; got: %s", args)
	}
	if strings.Contains(args, "--network none") {
		t.Fatalf("must not fall back to none when egress network is set; got: %s", args)
	}
}

// TestSelect_KataHonorsEgress ensures the egress network applies to every
// container backend, not just gvisor.
func TestSelect_KataHonorsEgress(t *testing.T) {
	args := argsFor(t, Select("kata", "img", "mcp-sandbox-egress", zerolog.Nop()))
	if !strings.Contains(args, "--network mcp-sandbox-egress") {
		t.Fatalf("kata must honor egress network; got: %s", args)
	}
}

// TestSelect_ExecIgnoresEgress: the unsandboxed exec backend has no container
// network; passing an egress network must not change its type.
func TestSelect_ExecIgnoresEgress(t *testing.T) {
	if _, ok := Select("exec", "img", "mcp-sandbox-egress", zerolog.Nop()).(ExecRuntime); !ok {
		t.Fatal("exec runtime expected regardless of egress network")
	}
}

// TestSandboxEgressContainment is the live adversarial proof (FR-017 / SC-010):
// a container on the egress network reaches ONLY the emulator — the control
// plane, cloud metadata (169.254.169.254), and the public internet are
// unreachable. Gated on a real gVisor/Docker stack.
//
//	MCP_TEST_SANDBOX_DOCKER=1
//	MCP_TEST_SANDBOX_EGRESS_NETWORK=mcp-sandbox-egress
//	MCP_TEST_AWS_HOSTPORT=ministack:4566
//	MCP_TEST_SANDBOX_IMAGE=acme/mcp-sandbox:dev   (needs bash for /dev/tcp)
func TestSandboxEgressContainment(t *testing.T) {
	if os.Getenv("MCP_TEST_SANDBOX_DOCKER") == "" {
		t.Skip("set MCP_TEST_SANDBOX_DOCKER=1 (+ a gVisor Docker stack) to run the live egress-containment proof")
	}
	net := getenvOr("MCP_TEST_SANDBOX_EGRESS_NETWORK", "mcp-sandbox-egress")
	image := getenvOr("MCP_TEST_SANDBOX_IMAGE", "acme/mcp-sandbox:dev")
	emulator := getenvOr("MCP_TEST_AWS_HOSTPORT", "ministack:4566")

	// reachable: the allowlisted emulator
	if err := tcpProbe(t, net, image, emulator); err != nil {
		t.Fatalf("sandbox on %q MUST reach the emulator %q, but could not: %v", net, emulator, err)
	}
	// unreachable: cloud metadata + control plane + public internet
	for _, target := range []string{"169.254.169.254:80", "control-plane:8090", "1.1.1.1:53"} {
		if err := tcpProbe(t, net, image, target); err == nil {
			t.Fatalf("ISOLATION BREACH: sandbox on %q reached forbidden target %q", net, target)
		}
	}
}

// tcpProbe runs a short-lived container on the given network and attempts a TCP
// connect to host:port via bash /dev/tcp; nil error == reachable.
func tcpProbe(t *testing.T, network, image, hostport string) error {
	t.Helper()
	host, port, ok := strings.Cut(hostport, ":")
	if !ok {
		t.Fatalf("bad hostport %q", hostport)
	}
	script := "timeout 3 bash -c 'exec 3<>/dev/tcp/" + host + "/" + port + "'"
	cmd := exec.Command("docker", "run", "--rm", "--network", network,
		"--entrypoint", "bash", image, "-c", script)
	done := make(chan error, 1)
	if err := cmd.Start(); err != nil {
		t.Fatalf("docker run failed to start: %v", err)
	}
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-time.After(15 * time.Second):
		_ = cmd.Process.Kill()
		return exec.ErrNotFound // treat hang as unreachable
	}
}

func getenvOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
