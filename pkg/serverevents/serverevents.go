// Package serverevents defines the change events the control plane publishes
// when admins add/remove MCP servers, and the bus that delivers them to the
// gateway data plane (Redis pub/sub in production; an in-process bus for
// single-process dev and tests).
package serverevents

import (
	"context"
	"sync"
)

// Channel is the Redis pub/sub channel for server-change events.
const Channel = "mcp:server-events"

// Action is the kind of change.
type Action string

const (
	ActionUpsert Action = "upsert"
	ActionRemove Action = "remove"
	// ActionCredentialChanged signals that a per-user credential was rotated, so
	// the gateway should drop the affected user's cached instance and rebuild it
	// with the new secret on next use (US6 rotation, T079). Org-level credential
	// rotation is signalled with a fresh ActionUpsert instead (rebuilds the
	// shared instance).
	ActionCredentialChanged Action = "credential_changed"
)

// Event describes a server change. It carries only non-secret configuration;
// the gateway resolves any downstream credentials from the secrets store.
type Event struct {
	Action      Action `json:"action"`
	OrgID       string `json:"org_id"`
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Type        string            `json:"type"` // remote_http | stdio
	EndpointURL string            `json:"endpoint_url,omitempty"`
	Command      string            `json:"command,omitempty"`
	Args         []string          `json:"args,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
	AllowedRoles   []string        `json:"allowed_roles,omitempty"` // empty = all org members (RBAC, US5)
	CredentialMode string          `json:"credential_mode,omitempty"` // none | org_shared | per_user (US6)
	UserID         string          `json:"user_id,omitempty"`         // target user for ActionCredentialChanged (empty = all)
}

// Bus publishes and subscribes to server-change events.
type Bus interface {
	Publish(ctx context.Context, e Event) error
	// Subscribe delivers events to handler until ctx is cancelled.
	Subscribe(ctx context.Context, handler func(Event)) error
}

// MemBus is an in-process Bus for single-process dev and tests.
type MemBus struct {
	mu       sync.RWMutex
	handlers []func(Event)
}

// NewMemBus returns an empty in-process bus.
func NewMemBus() *MemBus { return &MemBus{} }

// Register synchronously adds a handler.
func (b *MemBus) Register(h func(Event)) {
	b.mu.Lock()
	b.handlers = append(b.handlers, h)
	b.mu.Unlock()
}

// Publish delivers e to all registered handlers synchronously.
func (b *MemBus) Publish(_ context.Context, e Event) error {
	b.mu.RLock()
	hs := make([]func(Event), len(b.handlers))
	copy(hs, b.handlers)
	b.mu.RUnlock()
	for _, h := range hs {
		h(e)
	}
	return nil
}

// Subscribe registers handler and blocks until ctx is cancelled.
func (b *MemBus) Subscribe(ctx context.Context, handler func(Event)) error {
	b.Register(handler)
	<-ctx.Done()
	return ctx.Err()
}
