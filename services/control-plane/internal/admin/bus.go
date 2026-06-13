package admin

import (
	"context"

	"github.com/acme-corp/mcp-runtime/pkg/serverevents"
)

// busSink publishes server changes onto a serverevents.Bus for the data plane
// (the gateway subscribes and builds the actual downstream clients).
type busSink struct{ bus serverevents.Bus }

// NewBusSink returns a Sink that publishes changes onto bus.
func NewBusSink(bus serverevents.Bus) Sink { return &busSink{bus: bus} }

func (s *busSink) Add(srv Server) {
	_ = s.bus.Publish(context.Background(), serverevents.Event{
		Action:      serverevents.ActionUpsert,
		OrgID:       srv.OrgID,
		ID:          srv.ID,
		Slug:        srv.Slug,
		Type:        string(srv.Type),
		EndpointURL: srv.EndpointURL,
		Command:      srv.Command,
		Args:         srv.Args,
		Env:            srv.Env,
		AllowedRoles:   srv.AllowedRoles,
		CredentialMode: srv.CredentialMode,
	})
}

func (s *busSink) Remove(srv Server) {
	_ = s.bus.Publish(context.Background(), serverevents.Event{
		Action: serverevents.ActionRemove,
		OrgID:  srv.OrgID,
		ID:     srv.ID,
		Slug:   srv.Slug,
	})
}

// CredentialChanged drives the gateway to rebuild with the rotated secret on next
// use (T079). Org-level rotation re-emits a full upsert (rebuilds the shared
// instance); per-user rotation emits a targeted credential-changed event (drops
// just that user's cached instance).
func (s *busSink) CredentialChanged(srv Server, userID string) {
	if userID == "" {
		s.Add(srv)
		return
	}
	_ = s.bus.Publish(context.Background(), serverevents.Event{
		Action: serverevents.ActionCredentialChanged,
		OrgID:  srv.OrgID,
		ID:     srv.ID,
		Slug:   srv.Slug,
		UserID: userID,
	})
}
