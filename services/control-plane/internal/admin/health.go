package admin

import (
	"bytes"
	"context"
	"net/http"
	"time"
)

// ProbeRemote checks a remote MCP endpoint's reachability with an MCP
// initialize request, classifying the result (T038).
func ProbeRemote(ctx context.Context, url string, header http.Header) (Health, string) {
	body := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"initialize",` +
		`"params":{"protocolVersion":"2025-03-26","capabilities":{},` +
		`"clientInfo":{"name":"acme-mcp-controlplane","version":"0.1.0"}}}`)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return HealthUnreachable, err.Error()
	}
	for k, vs := range header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		return HealthUnreachable, err.Error()
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return HealthAuthFailed, "remote returned " + resp.Status
	case resp.StatusCode/100 == 2:
		return HealthHealthy, ""
	default:
		return HealthUnreachable, "remote returned " + resp.Status
	}
}
