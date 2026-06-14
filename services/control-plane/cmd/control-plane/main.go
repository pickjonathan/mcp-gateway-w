// Command control-plane is the admin API: org-scoped MCP server-definition CRUD,
// health checks, and change propagation to the data plane.
package main

import (
	"context"
	"os/signal"
	"strings"
	"syscall"

	"github.com/labstack/echo/v4"

	"github.com/acme-corp/mcp-runtime/pkg/audit"
	"github.com/acme-corp/mcp-runtime/pkg/authz"
	"github.com/acme-corp/mcp-runtime/pkg/config"
	"github.com/acme-corp/mcp-runtime/pkg/logging"
	"github.com/acme-corp/mcp-runtime/pkg/secrets"
	"github.com/acme-corp/mcp-runtime/pkg/serverevents"
	"github.com/acme-corp/mcp-runtime/services/control-plane/internal/admin"
	"github.com/acme-corp/mcp-runtime/services/control-plane/internal/brokering"
	"github.com/acme-corp/mcp-runtime/services/control-plane/internal/idp"
	"github.com/acme-corp/mcp-runtime/services/control-plane/internal/invites"
	"github.com/acme-corp/mcp-runtime/services/control-plane/internal/scimbridge"
	"github.com/acme-corp/mcp-runtime/services/control-plane/internal/tenants"
)

func main() {
	cfg := config.Get()
	log := logging.Init(cfg)
	if err := cfg.Validate(); err != nil {
		log.Fatal().Err(err).Msg("invalid configuration")
	}
	log.Info().Str("base_domain", cfg.BaseDomain).Msg("starting mcp control-plane")

	validator := authz.NewJWTValidator(cfg.BaseDomain, cfg.KeycloakIssuerTemplate, cfg.ResourceTemplate, authz.NewJWKSKeySource())
	bus := serverevents.NewRedisBus(cfg.RedisAddr)
	var sec secrets.Store = secrets.NewMemStore()
	if cfg.VaultAddr != "" {
		sec = secrets.NewVaultStore(cfg.VaultAddr, cfg.VaultToken)
	}
	var store admin.Store = admin.NewMemStore()
	if cfg.PostgresDSN != "" {
		ps, err := admin.NewPostgresStore(context.Background(), cfg.PostgresDSN)
		if err != nil {
			log.Fatal().Err(err).Msg("connect postgres")
		}
		store = ps
		log.Info().Msg("using postgres store")
	}
	// In-memory audit log for dev; a durable, Object-Lock'd S3 archive (T087) when
	// configured — same audit.Logger interface either way.
	var auditLog audit.Logger = audit.NewMemLogger()
	if cfg.AuditS3Endpoint != "" {
		arch, err := audit.NewS3Archive(context.Background(), cfg.AuditS3Endpoint, cfg.AuditS3AccessKey,
			cfg.AuditS3SecretKey, cfg.AuditS3Bucket, cfg.AuditS3UseSSL, cfg.AuditRetention)
		if err != nil {
			log.Fatal().Err(err).Msg("connect audit archive")
		}
		auditLog = arch
		log.Info().Str("bucket", cfg.AuditS3Bucket).Msg("using durable audit archive")
	}
	api := admin.NewAPI(cfg, log, store, admin.NewBusSink(bus), sec, auditLog, validator)

	// 003 tenant provisioning: platform-scoped tenant registry + operator API,
	// mounted on the same server, authorized against the platform realm.
	var tenantStore tenants.Store = tenants.NewMemStore()
	if cfg.PostgresDSN != "" {
		ts, err := tenants.NewPostgresStore(context.Background(), cfg.PostgresDSN)
		if err != nil {
			log.Fatal().Err(err).Msg("connect postgres (tenants)")
		}
		tenantStore = ts
		log.Info().Msg("using postgres tenant store")
	}
	// The privileged Keycloak admin credential is resolved from the secret store
	// (never read from env) and is held only here, never on the per-request path.
	var kc idp.Keycloak
	if cfg.KeycloakAdminClientID != "" {
		secret := cfg.KeycloakAdminSecret // dev direct secret
		if secret == "" && cfg.KeycloakAdminSecretRef != "" {
			if vals, err := sec.Get(context.Background(), cfg.KeycloakAdminSecretRef); err == nil {
				secret = vals["secret"]
			} else {
				log.Warn().Err(err).Msg("read keycloak admin secret ref")
			}
		}
		kc = idp.NewRESTClient(cfg.KeycloakAdminURL, "master", cfg.KeycloakAdminClientID, secret)
		log.Info().Str("url", cfg.KeycloakAdminURL).Msg("tenant provisioning enabled")
	} else {
		log.Info().Msg("tenant provisioning disabled (set MCP_KEYCLOAK_ADMIN_CLIENT_ID to enable)")
	}
	tenantSvc := tenants.NewService(tenantStore, kc, auditLog, log, tenants.Config{
		BaseDomain:         cfg.BaseDomain,
		ConsoleOrigin:      firstOrigin(cfg.ConsoleOrigins),
		AdminAudience:      cfg.AdminAudience,
		ResourceTemplate:   cfg.ResourceTemplate,
		ReservedSlugs:      cfg.TenantReservedSlugs,
		AuditRetentionDays: cfg.AuditRetentionDays,
		Ceiling:            cfg.TenantCeiling,
		AccessTTL:          900, SSOIdle: 28800, SSOMax: 86400,
		SSLRequired: sslRequired(cfg),
	})
	tenantHandlers := tenants.NewHandlers(tenantSvc, tenantStore)
	api.Mount(func(e *echo.Echo) {
		tenants.RegisterRoutes(e, tenantHandlers, validator, cfg.PlatformRealm, cfg.PlatformAudience)
	})

	// US2 invitations: org-scoped invite management + public accept.
	var inviteStore invites.Store = invites.NewMemStore()
	if cfg.PostgresDSN != "" {
		is, err := invites.NewPostgresStore(context.Background(), cfg.PostgresDSN)
		if err != nil {
			log.Fatal().Err(err).Msg("connect postgres (invites)")
		}
		inviteStore = is
		log.Info().Msg("using postgres invitation store")
	}
	inviteHandlers := invites.NewHandlers(inviteStore, kc, auditLog, log, nil)
	api.Mount(func(e *echo.Echo) {
		invites.RegisterRoutes(e, inviteHandlers, validator, cfg.AdminAudience)
	})

	// US4 brokering: org-scoped SSO IdP configuration applied to the realm.
	var brokerStore brokering.Store = brokering.NewMemStore()
	if cfg.PostgresDSN != "" {
		bs, err := brokering.NewPostgresStore(context.Background(), cfg.PostgresDSN)
		if err != nil {
			log.Fatal().Err(err).Msg("connect postgres (brokering)")
		}
		brokerStore = bs
		log.Info().Msg("using postgres brokering store")
	}
	var broker idp.Broker
	if rc, ok := kc.(*idp.RESTClient); ok {
		broker = rc // the REST client implements both Keycloak and Broker
	}
	brokerHandlers := brokering.NewHandlers(brokerStore, broker, sec, auditLog)
	api.Mount(func(e *echo.Echo) {
		brokering.RegisterRoutes(e, brokerHandlers, validator, cfg.AdminAudience)
	})

	// US4 SCIM directory sync: per-tenant bearer + Keycloak apply (deactivation
	// removes gateway access by next token).
	var scimStore scimbridge.Store = scimbridge.NewMemStore()
	if cfg.PostgresDSN != "" {
		ss, err := scimbridge.NewPostgresStore(context.Background(), cfg.PostgresDSN)
		if err != nil {
			log.Fatal().Err(err).Msg("connect postgres (scim)")
		}
		scimStore = ss
		log.Info().Msg("using postgres scim store")
	}
	var dir idp.Directory
	if rc, ok := kc.(*idp.RESTClient); ok {
		dir = rc
	}
	scimHandlers := scimbridge.NewHandlers(scimStore, dir, auditLog, cfg.BaseDomain)
	api.Mount(func(e *echo.Echo) {
		scimbridge.RegisterRoutes(e, scimHandlers, validator, cfg.AdminAudience)
	})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := api.Start(ctx); err != nil {
		log.Fatal().Err(err).Msg("control-plane error")
	}
	log.Info().Msg("shutdown complete")
}

// firstOrigin returns the first non-empty CSV origin (the per-tenant console
// redirect base), defaulting to the dev console URL.
func firstOrigin(csv string) string {
	for _, p := range strings.Split(csv, ",") {
		if p = strings.TrimSpace(p); p != "" {
			return p
		}
	}
	return "http://localhost:5173"
}

// sslRequired is "external" in prod and "none" in dev (matching the seed script's
// dev relaxation so http://localhost OIDC works).
func sslRequired(cfg *config.Config) string {
	if cfg.IsProd() {
		return "external"
	}
	return "none"
}
