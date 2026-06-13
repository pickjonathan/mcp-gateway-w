package admin

import (
	"strconv"

	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/acme-corp/mcp-runtime/pkg/telemetry"
)

// metricsMiddleware records one mcp_requests_total increment per request,
// attributed by HTTP method and final status code.
func metricsMiddleware(m *telemetry.Metrics) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			err := next(c)
			status := c.Response().Status
			if err != nil {
				if he, ok := err.(*echo.HTTPError); ok {
					status = he.Code
				}
			}
			m.Requests.Add(c.Request().Context(), 1, metric.WithAttributes(
				attribute.String("method", c.Request().Method),
				attribute.String("code", strconv.Itoa(status)),
			))
			return err
		}
	}
}
