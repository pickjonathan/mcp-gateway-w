// Command control-plane is the admin API: org-scoped MCP server-definition CRUD,
// health checks, and change propagation to the data plane.
package main

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/acme-corp/mcp-runtime/pkg/audit"
	"github.com/acme-corp/mcp-runtime/pkg/authz"
	"github.com/acme-corp/mcp-runtime/pkg/config"
	"github.com/acme-corp/mcp-runtime/pkg/logging"
	"github.com/acme-corp/mcp-runtime/pkg/secrets"
	"github.com/acme-corp/mcp-runtime/pkg/serverevents"
	"github.com/acme-corp/mcp-runtime/services/control-plane/internal/admin"
)

func main() {
	cfg := config.Get()
	log := logging.Init(cfg)
	if err := cfg.Validate(); err != nil {
		log.Fatal().Err(err).Msg("invalid configuration")
	}
	log.Info().Str("base_domain", cfg.BaseDomain).Msg("starting mcp control-plane")

	validator := authz.NewJWTValidator(cfg.BaseDomain, cfg.KeycloakIssuerTemplate, authz.NewJWKSKeySource())
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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := api.Start(ctx); err != nil {
		log.Fatal().Err(err).Msg("control-plane error")
	}
	log.Info().Msg("shutdown complete")
}
