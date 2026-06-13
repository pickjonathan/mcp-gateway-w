// Package downstream models the MCP servers the gateway proxies to and a
// per-org catalog of them. Concrete backends are remote HTTP (US2) and
// sandboxed stdio (US3); a Fake is provided for tests. Registrations carry the
// roles permitted to use them (RBAC, US5).
package downstream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/acme-corp/mcp-runtime/services/gateway/internal/aggregate"
)

// ErrNotFound is returned when no downstream is registered for a slug.
var ErrNotFound = errors.New("downstream: not found")

// Downstream is a connected MCP server.
type Downstream interface {
	ListTools(ctx context.Context) ([]aggregate.Tool, error)
	CallTool(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error)
}

// Provider lazily builds a per-user Downstream. Used for per_user credential
// mode, where every user connects with their own stored credentials (US6).
type Provider func(user string) (Downstream, error)

type entry struct {
	ds       Downstream            // fixed instance (none/org_shared); nil for per-user entries
	provider Provider              // per-user factory; nil for fixed entries
	users    map[string]Downstream // built per-user instances (provider entries only)
	roles    []string              // permitted roles; empty = visible to all org members
}

// Registry holds an org's downstream servers, keyed by slug, with their RBAC scope.
type Registry struct {
	mu      sync.RWMutex
	servers map[string]entry
	order   []string
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry { return &Registry{servers: make(map[string]entry)} }

// Add registers ds under slug, visible to all org members.
func (r *Registry) Add(slug string, ds Downstream) { r.AddScoped(slug, ds, nil) }

// AddScoped registers ds under slug, restricted to the given roles (nil/empty =
// all). Replacing an existing slug closes the previous instance(s) — so an
// upsert rebuild (e.g. org-credential rotation) doesn't leak the old client.
func (r *Registry) AddScoped(slug string, ds Downstream, roles []string) {
	r.mu.Lock()
	old, existed := r.servers[slug]
	if !existed {
		r.order = append(r.order, slug)
	}
	r.servers[slug] = entry{ds: ds, roles: roles}
	r.mu.Unlock()
	if existed {
		closeEntry(old)
	}
}

// AddProvider registers a per-user provider under slug, restricted to roles. Each
// user gets their own lazily-built, cached Downstream (per_user mode, US6).
// Replacing an existing slug closes the previous instance(s).
func (r *Registry) AddProvider(slug string, p Provider, roles []string) {
	r.mu.Lock()
	old, existed := r.servers[slug]
	if !existed {
		r.order = append(r.order, slug)
	}
	r.servers[slug] = entry{provider: p, users: make(map[string]Downstream), roles: roles}
	r.mu.Unlock()
	if existed {
		closeEntry(old)
	}
}

// Remove deregisters slug.
func (r *Registry) Remove(slug string) {
	r.mu.Lock()
	e, ok := r.servers[slug]
	if !ok {
		r.mu.Unlock()
		return
	}
	delete(r.servers, slug)
	for i, s := range r.order {
		if s == slug {
			r.order = append(r.order[:i], r.order[i+1:]...)
			break
		}
	}
	r.mu.Unlock()
	// Kill-switch: terminate the downstream(s) — fixed instance + every per-user one.
	closeEntry(e)
}

// Invalidate drops cached per-user instance(s) for slug so the next GetForUser
// rebuilds them from the provider, picking up a rotated secret (US6 rotation,
// T079). user=="" drops all cached users; the dropped instances are closed. It is
// a no-op for fixed entries — rebuild those via AddScoped (upsert re-emit).
func (r *Registry) Invalidate(slug, user string) {
	r.mu.Lock()
	e, ok := r.servers[slug]
	if !ok || e.users == nil {
		r.mu.Unlock()
		return
	}
	var closing []Downstream
	if user == "" {
		for k, inst := range e.users {
			closing = append(closing, inst)
			delete(e.users, k)
		}
	} else if inst, ok := e.users[user]; ok {
		closing = append(closing, inst)
		delete(e.users, user)
	}
	r.mu.Unlock()
	for _, inst := range closing {
		closeDownstream(inst)
	}
}

// Get returns the downstream registered under slug. For a per-user entry it
// returns (nil, true) — callers needing a usable instance must use GetForUser.
func (r *Registry) Get(slug string) (Downstream, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.servers[slug]
	if !ok {
		return nil, false
	}
	return e.ds, true
}

// GetForUser returns the downstream to use for (slug, user). A fixed entry
// returns its shared instance; a per-user entry returns a cached instance or
// builds one from the provider and caches it. Returns ErrNotFound if slug is
// unknown, or the provider's error if a per-user instance cannot be built (e.g.
// the user has no credentials configured).
func (r *Registry) GetForUser(slug, user string) (Downstream, error) {
	r.mu.RLock()
	e, ok := r.servers[slug]
	if !ok {
		r.mu.RUnlock()
		return nil, ErrNotFound
	}
	if e.ds != nil { // fixed instance (none/org_shared)
		r.mu.RUnlock()
		return e.ds, nil
	}
	if inst, ok := e.users[user]; ok { // cached per-user instance
		r.mu.RUnlock()
		return inst, nil
	}
	prov := e.provider
	r.mu.RUnlock()
	if prov == nil {
		return nil, ErrNotFound
	}

	// Build outside the lock, then store under it (double-checking for races).
	inst, err := prov(user)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	e2, ok := r.servers[slug]
	if !ok || e2.provider == nil {
		r.mu.Unlock()
		closeDownstream(inst) // slug removed/replaced while we were building
		return nil, ErrNotFound
	}
	if existing, ok := e2.users[user]; ok {
		r.mu.Unlock()
		closeDownstream(inst) // lost a concurrent build race — keep the winner
		return existing, nil
	}
	e2.users[user] = inst
	r.mu.Unlock()
	return inst, nil
}

func closeDownstream(d Downstream) {
	if d == nil {
		return
	}
	if c, ok := d.(io.Closer); ok {
		_ = c.Close()
	}
}

// closeEntry closes an entry's fixed instance and all its per-user instances.
func closeEntry(e entry) {
	closeDownstream(e.ds)
	for _, inst := range e.users {
		closeDownstream(inst)
	}
}

// CanAccess reports whether a principal with the given roles may use slug.
func (r *Registry) CanAccess(slug string, roles []string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.servers[slug]
	return ok && rolesAllow(e.roles, roles)
}

// VisibleSlugs returns, in registration order, the slugs a principal with the
// given roles may see (RBAC-filtered).
func (r *Registry) VisibleSlugs(roles []string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.order))
	for _, s := range r.order {
		if rolesAllow(r.servers[s].roles, roles) {
			out = append(out, s)
		}
	}
	return out
}

func rolesAllow(allowed, have []string) bool {
	if len(allowed) == 0 {
		return true // unrestricted
	}
	for _, a := range allowed {
		for _, h := range have {
			if a == h {
				return true
			}
		}
	}
	return false
}

// Catalog holds a separate Registry per organization. Servers are never shared
// across orgs (HC-1): every lookup is scoped by org id.
type Catalog struct {
	mu   sync.RWMutex
	orgs map[string]*Registry
}

// NewCatalog returns an empty catalog.
func NewCatalog() *Catalog { return &Catalog{orgs: make(map[string]*Registry)} }

// For returns the registry for org, creating it on first use.
func (c *Catalog) For(org string) *Registry {
	c.mu.RLock()
	r := c.orgs[org]
	c.mu.RUnlock()
	if r != nil {
		return r
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if r = c.orgs[org]; r == nil {
		r = NewRegistry()
		c.orgs[org] = r
	}
	return r
}

// Add registers ds for (org, slug), visible to all org members.
func (c *Catalog) Add(org, slug string, ds Downstream) { c.For(org).Add(slug, ds) }

// AddScoped registers ds for (org, slug), restricted to roles.
func (c *Catalog) AddScoped(org, slug string, ds Downstream, roles []string) {
	c.For(org).AddScoped(slug, ds, roles)
}

// AddProvider registers a per-user provider for (org, slug), restricted to roles.
func (c *Catalog) AddProvider(org, slug string, p Provider, roles []string) {
	c.For(org).AddProvider(slug, p, roles)
}

// Invalidate drops cached per-user instance(s) for (org, slug); see Registry.Invalidate.
func (c *Catalog) Invalidate(org, slug, user string) { c.For(org).Invalidate(slug, user) }

// Remove deregisters (org, slug).
func (c *Catalog) Remove(org, slug string) { c.For(org).Remove(slug) }

// Fake is an in-memory Downstream for tests.
type Fake struct {
	Tools   []aggregate.Tool
	Results map[string]json.RawMessage // original tool name -> canned MCP result
}

// ListTools implements Downstream.
func (f *Fake) ListTools(context.Context) ([]aggregate.Tool, error) { return f.Tools, nil }

// CallTool implements Downstream.
func (f *Fake) CallTool(_ context.Context, name string, _ json.RawMessage) (json.RawMessage, error) {
	if r, ok := f.Results[name]; ok {
		return r, nil
	}
	return nil, fmt.Errorf("downstream: unknown tool %q", name)
}
