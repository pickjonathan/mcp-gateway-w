package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// TestTracingMiddleware_RecordsSpan verifies the control-plane emits a span per
// request with method/route/status and the path org (FR-008, tracing symmetry
// with the gateway).
func TestTracingMiddleware_RecordsSpan(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	mw := tracingMiddleware(tp.Tracer("test"))

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/orgs/acme/servers", nil)
	c := e.NewContext(req, httptest.NewRecorder())
	c.SetPath("/v1/orgs/:org/servers")
	c.SetParamNames("org")
	c.SetParamValues("acme")

	h := func(c echo.Context) error { return c.String(http.StatusOK, "ok") }
	if err := mw(h)(c); err != nil {
		t.Fatal(err)
	}

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected exactly 1 span, got %d", len(spans))
	}
	sp := spans[0]
	if sp.Name() != "GET /v1/orgs/:org/servers" {
		t.Fatalf("span name = %q", sp.Name())
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
	if strs["http.request.method"] != "GET" {
		t.Fatalf("http.request.method = %q", strs["http.request.method"])
	}
	if ints["http.response.status_code"] != http.StatusOK {
		t.Fatalf("http.response.status_code = %d", ints["http.response.status_code"])
	}
	if strs["mcp.org"] != "acme" {
		t.Fatalf("mcp.org = %q, want \"acme\"", strs["mcp.org"])
	}
}
