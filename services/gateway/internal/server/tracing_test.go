package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/acme-corp/mcp-runtime/pkg/authz"
)

// TestTracingMiddleware_RecordsSpan verifies the gateway emits a properly
// attributed server span per request, including the principal's org (FR-008).
func TestTracingMiddleware_RecordsSpan(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	mw := tracingMiddleware(tp.Tracer("test"))

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	c := e.NewContext(req, httptest.NewRecorder())
	c.SetPath("/mcp")
	c.Set("principal", &authz.Principal{OrgID: "acme"})

	h := func(c echo.Context) error { return c.String(http.StatusOK, "ok") }
	if err := mw(h)(c); err != nil {
		t.Fatal(err)
	}

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected exactly 1 span, got %d", len(spans))
	}
	sp := spans[0]
	if sp.Name() != "POST /mcp" {
		t.Fatalf("span name = %q, want \"POST /mcp\"", sp.Name())
	}

	strs := map[string]string{}
	ints := map[string]int64{}
	for _, kv := range sp.Attributes() {
		if kv.Value.Type() == attribute.INT64 {
			ints[string(kv.Key)] = kv.Value.AsInt64()
		} else {
			strs[string(kv.Key)] = kv.Value.AsString()
		}
	}
	if strs["http.request.method"] != "POST" {
		t.Fatalf("http.request.method = %q", strs["http.request.method"])
	}
	if ints["http.response.status_code"] != http.StatusOK {
		t.Fatalf("http.response.status_code = %d", ints["http.response.status_code"])
	}
	if strs["mcp.org"] != "acme" {
		t.Fatalf("mcp.org = %q, want \"acme\"", strs["mcp.org"])
	}
}
