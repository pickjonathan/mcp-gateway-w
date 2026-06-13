// Command gateway is the multi-tenant MCP gateway: an OAuth 2.0 protected
// resource server that aggregates and proxies an organization's MCP servers.
package main

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/acme-corp/mcp-runtime/pkg/config"
	"github.com/acme-corp/mcp-runtime/pkg/logging"
	"github.com/acme-corp/mcp-runtime/pkg/serverevents"
	"github.com/acme-corp/mcp-runtime/services/gateway/internal/server"
)

func main() {
	cfg := config.Get()
	log := logging.Init(cfg)

	if err := cfg.Validate(); err != nil {
		log.Fatal().Err(err).Msg("invalid configuration")
	}

	log.Info().
		Str("base_domain", cfg.BaseDomain).
		Str("sandbox_runtime", cfg.SandboxRuntime).
		Msg("starting mcp gateway")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	srv := server.New(cfg, log)
	// Reconcile the catalog from the source of truth on startup — the durability
	// backstop for fire-and-forget pub/sub (a restarted gateway is correct
	// without waiting for change events).
	if cfg.PostgresDSN != "" {
		if src, err := server.NewPostgresSource(ctx, cfg.PostgresDSN); err != nil {
			log.Error().Err(err).Msg("reconcile source unavailable")
		} else {
			if err := srv.Reconcile(ctx, src); err != nil {
				log.Error().Err(err).Msg("startup reconcile failed")
			}
			src.Close()
		}
	}
	// Subscribe to control-plane server-change events (Redis pub/sub).
	srv.SubscribeServers(ctx, serverevents.NewRedisBus(cfg.RedisAddr))

	if err := srv.Start(ctx); err != nil {
		log.Fatal().Err(err).Msg("server error")
	}
	log.Info().Msg("shutdown complete")
}
