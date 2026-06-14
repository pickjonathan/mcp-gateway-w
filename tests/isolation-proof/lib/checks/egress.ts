// SC-010 / matrix V5 — sandbox egress containment (FR-017). A container on the
// egress network reaches ONLY the emulator; control plane, cloud metadata, and
// the internet are unreachable. Complements the Go test (egress_test.go).
import { execFileSync } from "node:child_process";
import { Report } from "../report.js";

function reachable(network: string, image: string, hostport: string): boolean {
  const [host, port] = hostport.split(":");
  try {
    execFileSync(
      "docker",
      ["run", "--rm", "--network", network, "--entrypoint", "bash", image, "-c",
        `timeout 3 bash -c 'exec 3<>/dev/tcp/${host}/${port}'`],
      { stdio: "ignore", timeout: 20000 },
    );
    return true;
  } catch {
    return false;
  }
}

export async function runEgress(report: Report): Promise<void> {
  const net = process.env.MCP_SANDBOX_EGRESS_NETWORK || "mcp-sandbox-egress";
  const image = process.env.MCP_SANDBOX_IMAGE || "acme/mcp-sandbox:dev";
  const emulator = process.env.MCP_TEST_AWS_HOSTPORT || "ministack:4566";

  report.add({
    id: "SC-010.emulator",
    ref: ["FR-017"],
    story: "SC-010",
    name: "sandbox reaches the emulator (allowlist works)",
    passed: reachable(net, image, emulator),
    detail: `network=${net} target=${emulator}`,
  });

  const forbidden = ["control-plane:8090", "169.254.169.254:80", "1.1.1.1:53"];
  const breaches = forbidden.filter((hp) => reachable(net, image, hp));
  report.add({
    id: "SC-010.containment",
    ref: ["FR-017", "SC-010"],
    story: "SC-010",
    name: "sandbox cannot reach control-plane / metadata / internet",
    passed: breaches.length === 0,
    detail: breaches.length ? `BREACH: reached ${breaches.join(", ")}` : "all forbidden targets blocked",
  });
}
