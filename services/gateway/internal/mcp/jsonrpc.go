// Package mcp implements the MCP JSON-RPC dispatch the gateway exposes,
// aggregating and routing across a tenant's downstream servers.
package mcp

import "encoding/json"

// Request is a JSON-RPC 2.0 request (MCP Streamable HTTP transport).
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// Error is a JSON-RPC 2.0 error object.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// JSON-RPC / MCP error codes.
const (
	CodeParse          = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternal       = -32603
	CodeRateLimited    = -32000 // implementation-defined: per-tenant rate/quota exceeded
)

// MCP / JSON-RPC method names.
const (
	MethodInitialize  = "initialize"
	MethodPing        = "ping"
	MethodInitialized = "notifications/initialized"
	MethodToolsList   = "tools/list"
	MethodToolsCall   = "tools/call"
)

// IsNotification reports whether the request is a notification (no id), which
// receives no response.
func (r *Request) IsNotification() bool { return len(r.ID) == 0 }

func newError(id json.RawMessage, code int, msg string) *Response {
	return &Response{JSONRPC: "2.0", ID: id, Error: &Error{Code: code, Message: msg}}
}

func newResult(id json.RawMessage, result any) *Response {
	return &Response{JSONRPC: "2.0", ID: id, Result: result}
}
