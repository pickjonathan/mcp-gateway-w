// Package admin implements the control-plane: server-definition CRUD, health
// checks, admin authorization, and change propagation to the data plane.
package admin

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// ServerType is the kind of MCP server.
type ServerType string

const (
	TypeRemoteHTTP ServerType = "remote_http"
	TypeStdio      ServerType = "stdio"
)

// Health is a server's last observed status.
type Health string

const (
	HealthUnknown       Health = "unknown"
	HealthHealthy       Health = "healthy"
	HealthUnreachable   Health = "unreachable"
	HealthAuthFailed    Health = "auth_failed"
	HealthStartupFailed Health = "startup_failed"
)

// Server is a registered MCP server definition (the control-plane source of truth).
type Server struct {
	ID             string            `json:"id"`
	OrgID          string            `json:"org_id"`
	Slug           string            `json:"slug"`
	Type           ServerType        `json:"type"`
	EndpointURL    string            `json:"endpoint_url,omitempty"`
	Command        string            `json:"command,omitempty"`
	Args           []string          `json:"args,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	CredentialMode string            `json:"credential_mode,omitempty"`
	AllowedRoles   []string          `json:"allowed_roles,omitempty"`
	Enabled        bool              `json:"enabled"`
	Health         Health            `json:"health"`
	HealthDetail   string            `json:"health_detail,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
}

// Store errors.
var (
	ErrNotFound  = errors.New("server not found")
	ErrSlugTaken = errors.New("slug already in use")
)

// Store persists server definitions, scoped by org. In-memory for now; a
// PostgreSQL implementation lands with T007.
type Store interface {
	Create(s Server) (Server, error)
	Get(org, id string) (Server, error)
	List(org string) []Server
	Update(s Server) (Server, error)
	Delete(org, id string) error
}

// MemStore is an in-memory, org-scoped Store.
type MemStore struct {
	mu   sync.RWMutex
	byID map[string]Server // key: org + "/" + id
	seq  int
}

// NewMemStore returns an empty in-memory store.
func NewMemStore() *MemStore { return &MemStore{byID: make(map[string]Server)} }

func storeKey(org, id string) string { return org + "/" + id }

// Create stores a new server, rejecting duplicate slugs within the org (HC-1
// scoping: keys always include org).
func (m *MemStore) Create(s Server) (Server, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.byID {
		if e.OrgID == s.OrgID && e.Slug == s.Slug {
			return Server{}, ErrSlugTaken
		}
	}
	m.seq++
	s.ID = fmt.Sprintf("srv-%d", m.seq)
	m.byID[storeKey(s.OrgID, s.ID)] = s
	return s, nil
}

// Get returns a server scoped to org; a cross-org id is reported as not found.
func (m *MemStore) Get(org, id string) (Server, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.byID[storeKey(org, id)]
	if !ok {
		return Server{}, ErrNotFound
	}
	return s, nil
}

// List returns all servers for org.
func (m *MemStore) List(org string) []Server {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Server, 0)
	for _, s := range m.byID {
		if s.OrgID == org {
			out = append(out, s)
		}
	}
	return out
}

// Update replaces an existing server.
func (m *MemStore) Update(s Server) (Server, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := storeKey(s.OrgID, s.ID)
	if _, ok := m.byID[k]; !ok {
		return Server{}, ErrNotFound
	}
	m.byID[k] = s
	return s, nil
}

// Delete removes a server scoped to org.
func (m *MemStore) Delete(org, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := storeKey(org, id)
	if _, ok := m.byID[k]; !ok {
		return ErrNotFound
	}
	delete(m.byID, k)
	return nil
}
