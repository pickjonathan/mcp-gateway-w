package server

import (
	"context"

	"github.com/acme-corp/mcp-runtime/pkg/serverevents"
)

// ServerSource provides the current desired set of enabled servers (the source
// of truth) for reconciling the gateway's catalog on startup. This is the
// durability backstop for fire-and-forget pub/sub propagation.
type ServerSource interface {
	ListEnabled(ctx context.Context) ([]serverevents.Event, error)
}

// Reconcile loads the enabled servers from src and applies each as an upsert,
// rebuilding the per-org catalog. Safe to call at startup before subscribing to
// live change events.
func (s *Server) Reconcile(ctx context.Context, src ServerSource) error {
	evs, err := src.ListEnabled(ctx)
	if err != nil {
		return err
	}
	for _, e := range evs {
		if e.Action == "" {
			e.Action = serverevents.ActionUpsert
		}
		s.applyServerEvent(e)
	}
	s.log.Info().Int("servers", len(evs)).Msg("reconciled server catalog from source of truth")
	return nil
}
