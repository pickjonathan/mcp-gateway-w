// Package telemetry provides OpenTelemetry instrumentation for the services.
//
// Metrics are recorded through the OpenTelemetry metric API and exported in
// Prometheus text format at /metrics (pull model). This satisfies the
// observability principle (Constitution VI / FR-008) for the metrics signal;
// distributed tracing via OTLP is a follow-up that can register additional
// providers here without touching call sites.
package telemetry

import (
	"context"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// Metrics holds the instruments and the scrape handler for a service. It is
// self-contained (its own Prometheus registry) so tests and multiple services
// in one process do not collide on the global default registry.
type Metrics struct {
	// Requests counts handled HTTP requests, attributed by method and code.
	Requests metric.Int64Counter
	// ToolCalls counts MCP tools/call dispatches, attributed by outcome.
	ToolCalls metric.Int64Counter
	// AuditDropped counts audit events dropped by the rate limiter, by action —
	// so suppression is observable rather than a silent cap.
	AuditDropped metric.Int64Counter

	provider *sdkmetric.MeterProvider
	handler  http.Handler
}

// NewMetrics builds a meter backed by a Prometheus exporter and constructs the
// instruments. service names the instrumentation scope (e.g. "mcp-gateway").
func NewMetrics(service string) (*Metrics, error) {
	reg := prometheus.NewRegistry()
	exporter, err := otelprom.New(otelprom.WithRegisterer(reg))
	if err != nil {
		return nil, err
	}
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(exporter))
	// Register as the global meter provider so packages using the global meter
	// (e.g. tenant provisioning counters) export on this service's /metrics. Each
	// service is its own process, so there is no cross-service collision.
	otel.SetMeterProvider(provider)
	meter := provider.Meter(service)

	requests, err := meter.Int64Counter(
		"mcp_requests_total",
		metric.WithDescription("Total HTTP requests handled by the service"),
	)
	if err != nil {
		return nil, err
	}
	toolCalls, err := meter.Int64Counter(
		"mcp_tool_calls_total",
		metric.WithDescription("Total MCP tools/call dispatches by outcome"),
	)
	if err != nil {
		return nil, err
	}
	auditDropped, err := meter.Int64Counter(
		"mcp_audit_dropped_total",
		metric.WithDescription("Audit events dropped by the rate limiter, by action"),
	)
	if err != nil {
		return nil, err
	}

	return &Metrics{
		Requests:     requests,
		ToolCalls:    toolCalls,
		AuditDropped: auditDropped,
		provider:     provider,
		handler:      promhttp.HandlerFor(reg, promhttp.HandlerOpts{}),
	}, nil
}

// Handler serves the metrics in Prometheus text format (mount at /metrics).
func (m *Metrics) Handler() http.Handler { return m.handler }

// Shutdown flushes and stops the meter provider.
func (m *Metrics) Shutdown(ctx context.Context) error { return m.provider.Shutdown(ctx) }
