// Wrapper around `npx @modelcontextprotocol/inspector --cli` (FR-007/008).
import { execFile } from "node:child_process";

export interface InspectorResult {
  ok: boolean; // process exited 0
  exitCode: number | string | null;
  stdout: string;
  stderr: string;
  json: any; // parsed stdout if JSON
}

function run(args: string[], timeoutMs = 60000): Promise<InspectorResult> {
  return new Promise((resolve) => {
    execFile(
      "npx",
      ["--yes", "@modelcontextprotocol/inspector", "--cli", ...args],
      { timeout: timeoutMs, maxBuffer: 16 * 1024 * 1024 },
      (err: any, stdout, stderr) => {
        let json: any;
        try {
          json = stdout ? JSON.parse(stdout) : undefined;
        } catch {
          json = undefined;
        }
        resolve({
          ok: !err,
          exitCode: err?.code ?? 0,
          stdout: stdout || "",
          stderr: stderr || "",
          json,
        });
      },
    );
  });
}

const authHeader = (token: string) => `Authorization: Bearer ${token}`;

export function toolsList(url: string, token: string): Promise<InspectorResult> {
  return run([url, "--transport", "http", "--header", authHeader(token), "--method", "tools/list"]);
}

export function toolsCall(
  url: string,
  token: string,
  tool: string,
  args: Record<string, string>,
): Promise<InspectorResult> {
  const a = [url, "--transport", "http", "--header", authHeader(token), "--method", "tools/call", "--tool-name", tool];
  for (const [k, v] of Object.entries(args)) a.push("--tool-arg", `${k}=${v}`);
  return run(a);
}

/** Convenience: run an AWS CLI command through the AWS MCP server's call_aws tool. */
export function callAws(url: string, token: string, cliCommand: string): Promise<InspectorResult> {
  return toolsCall(url, token, "call_aws", { cli_command: cliCommand });
}
