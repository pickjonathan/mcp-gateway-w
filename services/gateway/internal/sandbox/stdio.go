package sandbox

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"

	"github.com/acme-corp/mcp-runtime/services/gateway/internal/aggregate"
	"github.com/acme-corp/mcp-runtime/services/gateway/internal/mcp"
)

const clientProtocolVersion = "2025-03-26"

// conn speaks newline-delimited JSON-RPC over a process's stdio (the MCP stdio
// transport): one JSON message per line.
type conn struct {
	w  io.Writer
	r  *bufio.Reader
	mu sync.Mutex
	id int
}

func newConn(stdin io.Writer, stdout io.Reader) *conn {
	return &conn{w: stdin, r: bufio.NewReader(stdout)}
}

// call sends a request and returns the result for the matching id, skipping
// notifications and unrelated lines.
func (c *conn) call(method string, params json.RawMessage) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.id++
	wantID := strconv.Itoa(c.id)
	reqObj := map[string]any{"jsonrpc": "2.0", "id": c.id, "method": method}
	if len(params) > 0 {
		reqObj["params"] = params
	}
	line, _ := json.Marshal(reqObj)
	if _, err := c.w.Write(append(line, '\n')); err != nil {
		return nil, err
	}

	for {
		raw, err := c.r.ReadBytes('\n')
		if err != nil {
			return nil, err
		}
		var resp struct {
			ID     json.RawMessage `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *mcp.Error      `json:"error"`
		}
		if json.Unmarshal(raw, &resp) != nil || len(resp.ID) == 0 {
			continue // non-JSON, log line, or notification
		}
		if strings.TrimSpace(string(resp.ID)) != wantID {
			continue
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("sandbox: rpc error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	}
}

func (c *conn) notify(method string) {
	line, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": method})
	c.mu.Lock()
	_, _ = c.w.Write(append(line, '\n'))
	c.mu.Unlock()
}

// Server is a downstream.Downstream backed by a sandboxed stdio MCP process. It
// launches lazily on first use via the Runtime.
type Server struct {
	rt   Runtime
	spec Spec

	mu     sync.Mutex
	inst   *Instance
	c      *conn
	inited bool
	cancel context.CancelFunc // cancels the instance's lifetime context (see ensure)
}

// NewServer returns a stdio-backed downstream that launches spec via rt.
func NewServer(rt Runtime, spec Spec) *Server { return &Server{rt: rt, spec: spec} }

func (s *Server) ensure(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inited {
		return nil
	}
	// The sandbox instance is reused across requests, so it MUST outlive any single
	// request. Launch it under a lifetime context (cancelled by Close), not the
	// request ctx — otherwise the container is killed when the first request's
	// context is cancelled, and the next call writes to a dead process (broken pipe).
	runCtx, cancel := context.WithCancel(context.Background())
	inst, err := s.rt.Launch(runCtx, s.spec)
	if err != nil {
		cancel()
		return err
	}
	c := newConn(inst.Stdin, inst.Stdout)
	initParams := json.RawMessage(fmt.Sprintf(
		`{"protocolVersion":%q,"capabilities":{},"clientInfo":{"name":"acme-mcp-gateway","version":"0.1.0"}}`,
		clientProtocolVersion))
	if _, err := c.call("initialize", initParams); err != nil {
		_ = inst.Stop()
		cancel()
		return err
	}
	c.notify("notifications/initialized")
	s.inst, s.c, s.inited, s.cancel = inst, c, true, cancel
	return nil
}

// ListTools implements downstream.Downstream.
func (s *Server) ListTools(ctx context.Context) ([]aggregate.Tool, error) {
	if err := s.ensure(ctx); err != nil {
		return nil, err
	}
	res, err := s.c.call("tools/list", nil)
	if err != nil {
		return nil, err
	}
	var out struct {
		Tools []aggregate.Tool `json:"tools"`
	}
	if err := json.Unmarshal(res, &out); err != nil {
		return nil, err
	}
	return out.Tools, nil
}

// CallTool implements downstream.Downstream, passing the result through verbatim.
func (s *Server) CallTool(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
	if err := s.ensure(ctx); err != nil {
		return nil, err
	}
	if len(args) == 0 {
		args = json.RawMessage(`{}`)
	}
	params, _ := json.Marshal(map[string]any{"name": name, "arguments": args})
	return s.c.call("tools/call", params)
}

// Close stops the underlying process (idle reclamation / shutdown).
func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	if s.inst != nil {
		err := s.inst.Stop()
		s.inst, s.c, s.inited = nil, nil, false
		return err
	}
	return nil
}
