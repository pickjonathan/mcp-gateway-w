package sandbox

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

// fakeStdioServer is a minimal newline-delimited JSON-RPC MCP server.
func fakeStdioServer(in io.Reader, out io.Writer) {
	sc := bufio.NewScanner(in)
	for sc.Scan() {
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if json.Unmarshal(sc.Bytes(), &req) != nil || len(req.ID) == 0 {
			continue // skip non-JSON / notifications
		}
		id := string(req.ID)
		var resp string
		switch req.Method {
		case "initialize":
			resp = `{"jsonrpc":"2.0","id":` + id + `,"result":{"protocolVersion":"2025-03-26","capabilities":{"tools":{}},"serverInfo":{"name":"fake","version":"1"}}}`
		case "tools/list":
			resp = `{"jsonrpc":"2.0","id":` + id + `,"result":{"tools":[{"name":"ping"}]}}`
		case "tools/call":
			resp = `{"jsonrpc":"2.0","id":` + id + `,"result":{"content":[{"type":"text","text":"pong"}]}}`
		default:
			resp = `{"jsonrpc":"2.0","id":` + id + `,"error":{"code":-32601,"message":"nope"}}`
		}
		_, _ = io.WriteString(out, resp+"\n")
	}
}

type pipeRuntime struct {
	stdin  io.WriteCloser
	stdout io.Reader
}

func (p pipeRuntime) Launch(context.Context, Spec) (*Instance, error) {
	return &Instance{Stdin: p.stdin, Stdout: p.stdout, Stop: func() error { return p.stdin.Close() }}, nil
}

// TestStdioServer_OverPipe exercises the stdio JSON-RPC bridge against an
// in-process fake server (no subprocess).
func TestStdioServer_OverPipe(t *testing.T) {
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	go fakeStdioServer(inR, outW)

	s := NewServer(pipeRuntime{stdin: inW, stdout: outR}, Spec{Command: "fake"})
	defer func() { _ = s.Close() }()

	tools, err := s.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "ping" {
		t.Fatalf("unexpected tools: %+v", tools)
	}
	out, err := s.CallTool(context.Background(), "ping", nil)
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !strings.Contains(string(out), "pong") {
		t.Fatalf("expected pong passthrough, got %s", out)
	}
}

// TestExecRuntime_RealSubprocess launches the test binary as a real stdio MCP
// server via ExecRuntime and drives it over actual OS pipes.
func TestExecRuntime_RealSubprocess(t *testing.T) {
	s := NewServer(ExecRuntime{}, Spec{
		Command: os.Args[0],
		Args:    []string{"-test.run=TestStdioHelperProcess"},
		Env:     []string{"GO_WANT_STDIO_HELPER=1"},
	})
	defer func() { _ = s.Close() }()

	tools, err := s.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "ping" {
		t.Fatalf("unexpected tools: %+v", tools)
	}
	out, err := s.CallTool(context.Background(), "ping", nil)
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !strings.Contains(string(out), "pong") {
		t.Fatalf("expected pong passthrough, got %s", out)
	}
}

// TestStdioHelperProcess is not a real test: when GO_WANT_STDIO_HELPER is set it
// acts as the stdio MCP server subprocess for TestExecRuntime_RealSubprocess.
func TestStdioHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_STDIO_HELPER") != "1" {
		return
	}
	fakeStdioServer(os.Stdin, os.Stdout)
	os.Exit(0)
}
