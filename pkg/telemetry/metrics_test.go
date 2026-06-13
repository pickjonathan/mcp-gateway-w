package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func TestMetrics_ExposesRecordedCounters(t *testing.T) {
	m, err := NewMetrics("test")
	if err != nil {
		t.Fatalf("NewMetrics: %v", err)
	}
	ctx := context.Background()
	m.Requests.Add(ctx, 3, metric.WithAttributes(
		attribute.String("method", "GET"),
		attribute.String("code", "200"),
	))
	m.ToolCalls.Add(ctx, 1, metric.WithAttributes(attribute.String("outcome", "ok")))

	body := scrape(t, m)
	if !strings.Contains(body, "mcp_requests_total") {
		t.Fatalf("expected mcp_requests_total in scrape:\n%s", body)
	}
	// The counter value must reflect the recorded increment (3), with labels.
	if !strings.Contains(body, `mcp_requests_total{`) || !strings.Contains(body, "} 3") {
		t.Fatalf("expected mcp_requests_total value 3 with labels:\n%s", body)
	}
	if !strings.Contains(body, "mcp_tool_calls_total") {
		t.Fatalf("expected mcp_tool_calls_total in scrape:\n%s", body)
	}
}

// Each Metrics has its own registry, so two instances do not collide.
func TestMetrics_IsolatedRegistries(t *testing.T) {
	if _, err := NewMetrics("a"); err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, err := NewMetrics("b"); err != nil {
		t.Fatalf("second NewMetrics must not panic on duplicate registration: %v", err)
	}
}

func scrape(t *testing.T, m *Metrics) string {
	t.Helper()
	rr := httptest.NewRecorder()
	m.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("scrape status = %d", rr.Code)
	}
	return rr.Body.String()
}
