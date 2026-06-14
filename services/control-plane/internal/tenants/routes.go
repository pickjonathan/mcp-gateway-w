package tenants

import "github.com/labstack/echo/v4"

// RegisterRoutes mounts the platform (operator) tenant API on e, guarded by the
// platform-admin authorization (platform realm + platform-admin role). The whole
// registry + lifecycle is reachable only via this platform-authorized group; no
// tenant/org token can reach it (Constitution I).
func RegisterRoutes(e *echo.Echo, h *Handlers, v RealmValidator, platformRealm, platformAudience string) {
	g := e.Group("/v1/platform/tenants")
	g.Use(requirePlatformAdmin(v, platformRealm, platformAudience))

	g.POST("", h.Create)
	g.GET("", h.List)
	g.GET("/:slug", h.Get)
	g.GET("/:slug/jobs/:id", h.GetJob)
	g.POST("/:slug/suspend", h.Suspend)
	g.POST("/:slug/resume", h.Resume)
	g.DELETE("/:slug", h.Delete)
}
