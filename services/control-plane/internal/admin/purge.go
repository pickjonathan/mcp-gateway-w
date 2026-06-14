package admin

import "context"

// OrgPurger removes all of an org's server definitions and emits removal events.
// It is the kill-switch invoked when a tenant is deleted (003-tenant-provisioning
// T041 / FR-021): the gateway consumes the removal events to terminate running
// instances and revoke injected credentials. Reuses the org-scoped store + the
// existing change-event sink, so the data plane needs no change.
type OrgPurger struct {
	store Store
	sink  Sink
}

// NewOrgPurger builds a purger over the server store and change-event sink.
func NewOrgPurger(store Store, sink Sink) *OrgPurger { return &OrgPurger{store: store, sink: sink} }

// PurgeOrg deletes every server of org (RLS-scoped) and emits a removal event for
// each. Returns the number purged.
func (p *OrgPurger) PurgeOrg(_ context.Context, org string) (int, error) {
	servers := p.store.List(org)
	n := 0
	for _, s := range servers {
		if err := p.store.Delete(org, s.ID); err != nil {
			continue
		}
		p.sink.Remove(s)
		n++
	}
	return n, nil
}
