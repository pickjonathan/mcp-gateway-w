// Package remotehttp implements a downstream.Downstream that speaks the MCP
// Streamable HTTP transport to a remote MCP server (US2). Egress is expected to
// be constrained by the platform's egress controls (research R7).
package remotehttp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	"github.com/acme-corp/mcp-runtime/services/gateway/internal/aggregate"
	"github.com/acme-corp/mcp-runtime/services/gateway/internal/mcp"
)

const clientProtocolVersion = "2025-03-26"

var errSessionExpired = errors.New("remotehttp: session expired")

// Client is a remote MCP server reached over Streamable HTTP.
type Client struct {
	url    string
	http   *http.Client
	header http.Header

	mu        sync.RWMutex
	sessionID string
	inited    bool
}

// Option configures a Client.
type Option func(*Client)

// WithHeader adds static headers (e.g. Authorization) to every request.
func WithHeader(h http.Header) Option { return func(c *Client) { c.header = h } }

// WithHTTPClient overrides the underlying HTTP client.
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.http = h } }

// WithTimeout sets the per-request timeout (failure isolation, T041).
func WithTimeout(d time.Duration) Option { return func(c *Client) { c.http.Timeout = d } }

// WithBlockPrivate, when block is true, installs a dialer that refuses
// connections to non-public IPs (loopback/private/link-local/metadata) — SSRF
// protection for admin-supplied endpoints (T049). A no-op when block is false,
// so dev/tests can still reach loopback servers.
func WithBlockPrivate(block bool) Option {
	return func(c *Client) {
		if block {
			c.http.Transport = guardedTransport()
		}
	}
}

// New builds a remote MCP client for url.
func New(url string, opts ...Option) *Client {
	c := &Client{url: url, http: &http.Client{Timeout: 30 * time.Second}, header: http.Header{}}
	for _, o := range opts {
		o(c)
	}
	return c
}

// ListTools implements downstream.Downstream.
func (c *Client) ListTools(ctx context.Context) ([]aggregate.Tool, error) {
	res, err := c.rpc(ctx, "tools/list", nil)
	if err != nil {
		return nil, err
	}
	var out struct {
		Tools []aggregate.Tool `json:"tools"`
	}
	if err := json.Unmarshal(res, &out); err != nil {
		return nil, fmt.Errorf("remotehttp: decode tools/list: %w", err)
	}
	return out.Tools, nil
}

// CallTool implements downstream.Downstream, passing the result through verbatim.
func (c *Client) CallTool(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
	if len(args) == 0 {
		args = json.RawMessage(`{}`)
	}
	params, _ := json.Marshal(map[string]any{"name": name, "arguments": args})
	return c.rpc(ctx, "tools/call", params)
}

func (c *Client) rpc(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	if err := c.ensureInit(ctx); err != nil {
		return nil, err
	}
	res, _, err := c.doRPC(ctx, method, params)
	if errors.Is(err, errSessionExpired) {
		c.reset()
		if err = c.ensureInit(ctx); err != nil {
			return nil, err
		}
		res, _, err = c.doRPC(ctx, method, params)
	}
	return res, err
}

func (c *Client) ensureInit(ctx context.Context) error {
	c.mu.RLock()
	inited := c.inited
	c.mu.RUnlock()
	if inited {
		return nil
	}
	initParams := json.RawMessage(fmt.Sprintf(
		`{"protocolVersion":%q,"capabilities":{},"clientInfo":{"name":"acme-mcp-gateway","version":"0.1.0"}}`,
		clientProtocolVersion))
	_, hdr, err := c.doRPC(ctx, "initialize", initParams)
	if err != nil {
		return err
	}
	c.mu.Lock()
	if sid := hdr.Get("Mcp-Session-Id"); sid != "" {
		c.sessionID = sid
	}
	c.inited = true
	c.mu.Unlock()
	c.notify(ctx, "notifications/initialized")
	return nil
}

func (c *Client) reset() {
	c.mu.Lock()
	c.inited = false
	c.sessionID = ""
	c.mu.Unlock()
}

func (c *Client) session() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sessionID
}

func (c *Client) doRPC(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, http.Header, error) {
	reqObj := map[string]any{"jsonrpc": "2.0", "id": 1, "method": method}
	if len(params) > 0 {
		reqObj["params"] = params
	}
	body, _ := json.Marshal(reqObj)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	c.applyHeaders(httpReq)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	if sid := c.session(); sid != "" {
		httpReq.Header.Set("Mcp-Session-Id", sid)
	}

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, resp.Header, errSessionExpired
	}
	if resp.StatusCode/100 != 2 {
		return nil, resp.Header, fmt.Errorf("remotehttp: status %d from %s", resp.StatusCode, c.url)
	}

	r, err := parseResponse(resp)
	if err != nil {
		return nil, resp.Header, err
	}
	if r.Error != nil {
		return nil, resp.Header, fmt.Errorf("remotehttp: rpc error %d: %s", r.Error.Code, r.Error.Message)
	}
	return r.Result, resp.Header, nil
}

func (c *Client) notify(ctx context.Context, method string) {
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": method})
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return
	}
	c.applyHeaders(httpReq)
	httpReq.Header.Set("Content-Type", "application/json")
	if sid := c.session(); sid != "" {
		httpReq.Header.Set("Mcp-Session-Id", sid)
	}
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}

func (c *Client) applyHeaders(req *http.Request) {
	for k, vs := range c.header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	// Propagate the active trace context downstream (W3C traceparent) so a trace
	// spans the gateway and the remote MCP server. No-op when tracing is disabled.
	otel.GetTextMapPropagator().Inject(req.Context(), propagation.HeaderCarrier(req.Header))
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *mcp.Error      `json:"error,omitempty"`
}

func parseResponse(resp *http.Response) (rpcResponse, error) {
	if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		var data strings.Builder
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			if line := sc.Text(); strings.HasPrefix(line, "data:") {
				data.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			}
		}
		if err := sc.Err(); err != nil {
			return rpcResponse{}, err
		}
		var r rpcResponse
		if err := json.Unmarshal([]byte(data.String()), &r); err != nil {
			return rpcResponse{}, fmt.Errorf("remotehttp: parse sse: %w", err)
		}
		return r, nil
	}
	var r rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return rpcResponse{}, err
	}
	return r, nil
}
