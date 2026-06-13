package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// Tracing owns a tracer provider and the tracer to instrument with.
type Tracing struct {
	provider *sdktrace.TracerProvider
	// Tracer is the service's tracer; always usable, even when export is disabled.
	Tracer trace.Tracer
}

// NewTracing builds a tracer provider for service. When otlpEndpoint is non-empty,
// spans are exported over OTLP/HTTP to that endpoint (host:port); when empty,
// spans are still created (so instrumentation is always live) but not exported.
// It installs the provider and a W3C TraceContext+Baggage propagator globally.
func NewTracing(ctx context.Context, service, otlpEndpoint string) (*Tracing, error) {
	res, err := resource.New(ctx, resource.WithAttributes(semconv.ServiceName(service)))
	if err != nil {
		return nil, err
	}
	opts := []sdktrace.TracerProviderOption{sdktrace.WithResource(res)}
	if otlpEndpoint != "" {
		exp, err := otlptracehttp.New(ctx,
			otlptracehttp.WithEndpoint(otlpEndpoint),
			otlptracehttp.WithInsecure(),
		)
		if err != nil {
			return nil, err
		}
		opts = append(opts, sdktrace.WithBatcher(exp))
	}
	tp := sdktrace.NewTracerProvider(opts...)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))
	return &Tracing{provider: tp, Tracer: tp.Tracer(service)}, nil
}

// Shutdown flushes pending spans and stops the provider.
func (t *Tracing) Shutdown(ctx context.Context) error { return t.provider.Shutdown(ctx) }
