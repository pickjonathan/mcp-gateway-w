package telemetry

import (
	"context"
	"net/http"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// TestNewTracing_PropagatorRoundTrip proves NewTracing (even with export disabled)
// installs a working tracer and W3C propagator: a span's context injects a
// traceparent header for downstream propagation.
func TestNewTracing_PropagatorRoundTrip(t *testing.T) {
	tr, err := NewTracing(context.Background(), "test", "")
	if err != nil {
		t.Fatalf("NewTracing: %v", err)
	}
	defer func() { _ = tr.Shutdown(context.Background()) }()

	if tr.Tracer == nil {
		t.Fatal("Tracer must be non-nil")
	}
	ctx, span := tr.Tracer.Start(context.Background(), "op")
	defer span.End()

	h := http.Header{}
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(h))
	if h.Get("traceparent") == "" {
		t.Fatalf("expected a traceparent header to be injected, got headers: %v", h)
	}
}
