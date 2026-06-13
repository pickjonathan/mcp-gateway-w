package admin

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/acme-corp/mcp-runtime/pkg/authz"
	"github.com/acme-corp/mcp-runtime/pkg/secrets"
)

// Credential endpoints are write-only: values are stored in the secrets backend
// and never returned (FR-015). There is no read endpoint that echoes values.

func (h *Handlers) bindValues(c echo.Context) (map[string]string, bool) {
	var v map[string]string
	if err := c.Bind(&v); err != nil || len(v) == 0 {
		return nil, false
	}
	return v, true
}

func principalOf(c echo.Context) *authz.Principal {
	p, _ := c.Get("principal").(*authz.Principal)
	return p
}

// PutOrgCredentials stores an org-level (shared) credential for a server.
func (h *Handlers) PutOrgCredentials(c echo.Context) error {
	org, id := c.Param("org"), c.Param("id")
	srv, err := h.store.Get(org, id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}
	v, ok := h.bindValues(c)
	if !ok {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "non-empty key/value object required")
	}
	if err := h.secrets.Put(c.Request().Context(), secrets.OrgRef(org, id), v); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to store secret")
	}
	if !srv.CredentialSet { // record non-secret "is set" status for the admin console
		srv.CredentialSet = true
		_, _ = h.store.Update(srv)
	}
	h.record(c, "credentials.put", id, map[string]string{"scope": "org"}) // value never recorded
	h.sink.CredentialChanged(srv, "")                                     // rotation applies on next instance start (T079)
	return c.NoContent(http.StatusNoContent)                              // never echo the value
}

// DeleteOrgCredentials removes the org-level credential for a server.
func (h *Handlers) DeleteOrgCredentials(c echo.Context) error {
	org, id := c.Param("org"), c.Param("id")
	if err := h.secrets.Delete(c.Request().Context(), secrets.OrgRef(org, id)); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete secret")
	}
	h.record(c, "credentials.delete", id, map[string]string{"scope": "org"})
	if srv, err := h.store.Get(org, id); err == nil {
		srv.CredentialSet = false
		_, _ = h.store.Update(srv)
		h.sink.CredentialChanged(srv, "")
	}
	return c.NoContent(http.StatusNoContent)
}

// PutMyCredentials stores the calling user's own per-user credential for a server.
func (h *Handlers) PutMyCredentials(c echo.Context) error {
	org, id := c.Param("org"), c.Param("id")
	srv, err := h.store.Get(org, id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}
	p := principalOf(c)
	if p == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "no principal")
	}
	v, ok := h.bindValues(c)
	if !ok {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "non-empty key/value object required")
	}
	if err := h.secrets.Put(c.Request().Context(), secrets.UserRef(org, id, p.UserID), v); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to store secret")
	}
	h.record(c, "credentials.put", id, map[string]string{"scope": "user"})
	h.sink.CredentialChanged(srv, p.UserID) // rotation applies on next instance start (T079)
	return c.NoContent(http.StatusNoContent)
}

// DeleteMyCredentials removes the calling user's per-user credential.
func (h *Handlers) DeleteMyCredentials(c echo.Context) error {
	org, id := c.Param("org"), c.Param("id")
	p := principalOf(c)
	if p == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "no principal")
	}
	if err := h.secrets.Delete(c.Request().Context(), secrets.UserRef(org, id, p.UserID)); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete secret")
	}
	h.record(c, "credentials.delete", id, map[string]string{"scope": "user"})
	if srv, err := h.store.Get(org, id); err == nil {
		h.sink.CredentialChanged(srv, p.UserID)
	}
	return c.NoContent(http.StatusNoContent)
}
