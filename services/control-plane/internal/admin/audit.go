package admin

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/acme-corp/mcp-runtime/pkg/audit"
)

// record writes an audit event for the current request (no-op if no logger).
// Metadata must never contain secret values.
func (h *Handlers) record(c echo.Context, action, target string, meta map[string]string) {
	if h.audit == nil {
		return
	}
	actor := ""
	if p := principalOf(c); p != nil {
		actor = p.UserID
	}
	_ = h.audit.Record(c.Request().Context(), audit.Event{
		OrgID:    c.Param("org"),
		Actor:    actor,
		Action:   action,
		Target:   target,
		Metadata: meta,
	})
}

// ListAudit returns the org's recent audit records (newest first).
func (h *Handlers) ListAudit(c echo.Context) error {
	if h.audit == nil {
		return c.JSON(http.StatusOK, []audit.Record{})
	}
	recs, err := h.audit.List(c.Request().Context(), c.Param("org"), 200)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "audit query failed")
	}
	return c.JSON(http.StatusOK, recs)
}
