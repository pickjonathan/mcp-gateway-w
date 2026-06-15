// Genuine MCP client over OAuth-protected Streamable HTTP (the same wire protocol
// the Inspector speaks). Used for tool CALLS because the Inspector CLI cannot pass
// --tool-arg to URL/HTTP servers; the Inspector is still used for tools/list.
import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StreamableHTTPClientTransport } from "@modelcontextprotocol/sdk/client/streamableHttp.js";

export async function connect(url: string, token: string): Promise<Client> {
  const transport = new StreamableHTTPClientTransport(new URL(url), {
    requestInit: { headers: { Authorization: `Bearer ${token}` } },
  });
  const client = new Client({ name: "isolation-proof", version: "0.0.0" }, { capabilities: {} });
  await client.connect(transport);
  return client;
}

export interface ListResult {
  ok: boolean;
  tools: any[];
  error?: string;
}
export async function listTools(url: string, token: string): Promise<ListResult> {
  let c: Client | undefined;
  try {
    c = await connect(url, token);
    const r: any = await c.listTools();
    return { ok: true, tools: r.tools || [] };
  } catch (e: any) {
    return { ok: false, tools: [], error: String(e?.message || e) };
  } finally {
    try {
      await c?.close();
    } catch {
      /* ignore */
    }
  }
}

export interface CallResult {
  ok: boolean;
  text: string;
  error?: string;
  code?: any;
}
export async function callTool(url: string, token: string, name: string, args: Record<string, any>): Promise<CallResult> {
  let c: Client | undefined;
  try {
    c = await connect(url, token);
    const r: any = await c.callTool({ name, arguments: args });
    return { ok: !r?.isError, text: JSON.stringify(r), code: r?.isError ? "tool_error" : undefined };
  } catch (e: any) {
    return { ok: false, text: "", error: String(e?.message || e), code: e?.code };
  } finally {
    try {
      await c?.close();
    } catch {
      /* ignore */
    }
  }
}

/** Attempt the MCP handshake; report whether the server accepted the token. */
export async function tryConnect(url: string, token: string): Promise<{ connected: boolean; error?: string; code?: any }> {
  let c: Client | undefined;
  try {
    c = await connect(url, token);
    await c.listTools();
    return { connected: true };
  } catch (e: any) {
    return { connected: false, error: String(e?.message || e), code: e?.code };
  } finally {
    try {
      await c?.close();
    } catch {
      /* ignore */
    }
  }
}

export function findTool(tools: any[], suffix: string): string | undefined {
  const t = tools.find((x) => String(x?.name || "").endsWith(suffix));
  return t?.name;
}
