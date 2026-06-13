package server

import (
	"context"
	"strconv"

	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/acme-corp/mcp-runtime/services/gateway/internal/mcp"
)

// metricsMiddleware records one mcp_requests_total increment per request,
// attributed by HTTP method and final status code. Only registered when metrics
// are enabled, so s.metrics is non-nil here.
func (s *Server) metricsMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			err := next(c)
			status := c.Response().Status
			if err != nil {
				if he, ok := err.(*echo.HTTPError); ok {
					status = he.Code
				}
			}
			s.metrics.Requests.Add(c.Request().Context(), 1, metric.WithAttributes(
				attribute.String("method", c.Request().Method),
				attribute.String("code", strconv.Itoa(status)),
			))
			return err
		}
	}
}

// observeToolCall records the outcome of a tools/call dispatch. It is a no-op
// for other methods or when metrics are disabled, so it is always safe to call.
func (s *Server) observeToolCall(ctx context.Context, req *mcp.Request, resp *mcp.Response) {
	if s.metrics == nil || req.Method != mcp.MethodToolsCall {
		return
	}
	outcome := "ok"
	if resp != nil && resp.Error != nil {
		outcome = "error"
	}
	s.metrics.ToolCalls.Add(ctx, 1, metric.WithAttributes(attribute.String("outcome", outcome)))
}
