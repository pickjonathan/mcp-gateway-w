package admin

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/acme-corp/mcp-runtime/pkg/audit"
	"github.com/acme-corp/mcp-runtime/pkg/secrets"
)

// Sink receives server add/remove events for propagation to the data plane.
// In production this is a Redis-pub/sub adapter the gateway subscribes to (the
// gateway builds the actual downstream clients); a no-op/fake is used otherwise.
type Sink interface {
	Add(s Server)
	Remove(s Server)
	// CredentialChanged signals that a credential for s was rotated. userID is the
	// affected user for per-user credentials, or "" for org-level (T079).
	CredentialChanged(s Server, userID string)
}

// NoopSink discards propagation events.
type NoopSink struct{}

// Add implements Sink.
func (NoopSink) Add(Server) {}

// Remove implements Sink.
func (NoopSink) Remove(Server) {}

// CredentialChanged implements Sink.
func (NoopSink) CredentialChanged(Server, string) {}

// Handlers implements the server-definition CRUD and credential endpoints.
type Handlers struct {
	store   Store
	sink    Sink
	secrets secrets.Store
	audit   audit.Logger
	clock   func() time.Time
}

// NewHandlers builds handlers over the store + secrets store + audit log,
// emitting changes to sink.
func NewHandlers(store Store, sink Sink, sec secrets.Store, auditLog audit.Logger) *Handlers {
	return &Handlers{store: store, sink: sink, secrets: sec, audit: auditLog, clock: time.Now}
}

type createReq struct {
	Slug           string            `json:"slug"`
	Type           ServerType        `json:"type"`
	EndpointURL    string            `json:"endpoint_url"`
	Command        string            `json:"command"`
	Args           []string          `json:"args"`
	Env            map[string]string `json:"env"`
	CredentialMode string            `json:"credential_mode"`
	AllowedRoles   []string          `json:"allowed_roles"`
}

// Create validates, health-checks (remote), stores, and propagates a server.
func (h *Handlers) Create(c echo.Context) error {
	org := c.Param("org")
	var req createReq
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body")
	}
	if req.Slug == "" {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "slug required")
	}
	switch req.Type {
	case TypeRemoteHTTP:
		if req.EndpointURL == "" {
			return echo.NewHTTPError(http.StatusUnprocessableEntity, "endpoint_url required for remote_http")
		}
	case TypeStdio:
		if req.Command == "" {
			return echo.NewHTTPError(http.StatusUnprocessableEntity, "command required for stdio")
		}
	default:
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "type must be remote_http or stdio")
	}

	s := Server{
		OrgID: org, Slug: req.Slug, Type: req.Type,
		EndpointURL: req.EndpointURL, Command: req.Command, Args: req.Args, Env: req.Env,
		CredentialMode: req.CredentialMode, AllowedRoles: req.AllowedRoles,
		Enabled: true, Health: HealthUnknown, CreatedAt: h.clock(),
	}
	if s.Type == TypeRemoteHTTP {
		s.Health, s.HealthDetail = ProbeRemote(c.Request().Context(), s.EndpointURL, nil)
	}

	created, err := h.store.Create(s)
	switch err {
	case nil:
	case ErrSlugTaken:
		return echo.NewHTTPError(http.StatusConflict, "slug already in use")
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if created.Enabled {
		h.sink.Add(created)
	}
	h.record(c, "server.create", created.Slug, map[string]string{"type": string(created.Type)})
	return c.JSON(http.StatusCreated, created)
}

// List returns the org's servers.
func (h *Handlers) List(c echo.Context) error {
	return c.JSON(http.StatusOK, h.store.List(c.Param("org")))
}

// Get returns a single server.
func (h *Handlers) Get(c echo.Context) error {
	s, err := h.store.Get(c.Param("org"), c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}
	return c.JSON(http.StatusOK, s)
}

// patchReq carries the mutable server fields. All are pointers so a PATCH only
// touches the fields the caller actually sends (the console edits type/endpoint/
// command/args/env/credentials/allowed_roles/enabled).
type patchReq struct {
	Enabled        *bool              `json:"enabled"`
	Type           *ServerType        `json:"type"`
	EndpointURL    *string            `json:"endpoint_url"`
	Command        *string            `json:"command"`
	Args           *[]string          `json:"args"`
	Env            *map[string]string `json:"env"`
	CredentialMode *string            `json:"credential_mode"`
	AllowedRoles   *[]string          `json:"allowed_roles"`
}

// Patch updates the provided mutable fields and re-propagates to the data plane.
func (h *Handlers) Patch(c echo.Context) error {
	org, id := c.Param("org"), c.Param("id")
	s, err := h.store.Get(org, id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}
	var req patchReq
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body")
	}
	if req.Enabled != nil {
		s.Enabled = *req.Enabled
	}
	if req.Type != nil {
		s.Type = *req.Type
	}
	if req.EndpointURL != nil {
		s.EndpointURL = *req.EndpointURL
	}
	if req.Command != nil {
		s.Command = *req.Command
	}
	if req.Args != nil {
		s.Args = *req.Args
	}
	if req.Env != nil {
		s.Env = *req.Env
	}
	if req.CredentialMode != nil {
		s.CredentialMode = *req.CredentialMode
	}
	if req.AllowedRoles != nil {
		s.AllowedRoles = *req.AllowedRoles
	}
	updated, err := h.store.Update(s)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if updated.Enabled {
		h.sink.Add(updated)
	} else {
		h.sink.Remove(updated)
	}
	h.record(c, "server.update", updated.Slug, nil)
	return c.JSON(http.StatusOK, updated)
}

// Delete removes a server and propagates the removal (the slug is needed for
// data-plane deregistration, so the record is fetched first).
func (h *Handlers) Delete(c echo.Context) error {
	org, id := c.Param("org"), c.Param("id")
	s, err := h.store.Get(org, id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}
	if err := h.store.Delete(org, id); err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}
	h.sink.Remove(s)
	h.record(c, "server.delete", s.Slug, nil)
	return c.NoContent(http.StatusNoContent)
}
