// Thin fetch wrapper returning status + parsed JSON (or raw text).

export interface HttpResult {
  status: number;
  ok: boolean;
  json: any;
  text: string;
}

export async function http(
  method: string,
  url: string,
  opts: { token?: string; body?: any; headers?: Record<string, string> } = {},
): Promise<HttpResult> {
  const res = await fetch(url, {
    method,
    headers: {
      ...(opts.body !== undefined ? { "content-type": "application/json" } : {}),
      ...(opts.token ? { authorization: `Bearer ${opts.token}` } : {}),
      ...(opts.headers || {}),
    },
    body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
  });
  const text = await res.text();
  let json: any;
  try {
    json = text ? JSON.parse(text) : undefined;
  } catch {
    json = undefined;
  }
  return { status: res.status, ok: res.ok, json, text };
}

export const sleep = (ms: number) => new Promise<void>((r) => setTimeout(r, ms));
