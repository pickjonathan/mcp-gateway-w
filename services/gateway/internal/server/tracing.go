package server

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/acme-corp/mcp-runtime/pkg/authz"
)

// tracingMiddleware starts a server span per request. It extracts any inbound
// W3C trace context (so a trace spans services), records method/route/status and
// the principal's org, and marks the span errored on failure. Attribute values
// are non-secret — credentials never become span attributes.
func tracingMiddleware(tracer trace.Tracer) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			ctx := otel.GetTextMapPropagator().Extract(req.Context(), propagation.HeaderCarrier(req.Header))
			ctx, span := tracer.Start(ctx, req.Method+" "+routePath(c),
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					attribute.String("http.request.method", req.Method),
					attribute.String("url.path", req.URL.Path),
				),
			)
			defer span.End()
			c.SetRequest(req.WithContext(ctx))

			err := next(c)

			status := c.Response().Status
			if err != nil {
				if he, ok := err.(*echo.HTTPError); ok {
					status = he.Code
				}
				span.RecordError(err)
				span.SetStatus(codes.Error, http.StatusText(status))
			}
			span.SetAttributes(attribute.Int("http.response.status_code", status))
			if p, ok := c.Get("principal").(*authz.Principal); ok && p != nil {
				span.SetAttributes(attribute.String("mcp.org", p.OrgID))
				if p.Username != "" {
					span.SetAttributes(attribute.String("mcp.user", p.Username))
				}
			}
			return err
		}
	}
}

// routePath returns the matched route template (e.g. "/mcp"), falling back to the
// raw path so span names stay low-cardinality even for unmatched requests.
func routePath(c echo.Context) string {
	if p := c.Path(); p != "" {
		return p
	}
	return c.Request().URL.Path
}
