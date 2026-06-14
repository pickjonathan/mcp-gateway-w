package tenants

import (
	"context"
	"errors"
	"sort"
	"sync"
)

var (
	// ErrNotFound is returned when a tenant or job does not exist.
	ErrNotFound = errors.New("tenant not found")
	// ErrSlugTaken is returned when creating a tenant whose slug already exists.
	ErrSlugTaken = errors.New("tenant slug already exists")
)

// Store persists the platform-scoped tenant registry and provisioning jobs. It is
// not org-scoped (no RLS): only the platform API reaches it.
type Store interface {
	CreateTenant(ctx context.Context, t Tenant) error
	GetTenant(ctx context.Context, slug string) (Tenant, error)
	ListTenants(ctx context.Context) ([]Tenant, error)
	UpdateTenant(ctx context.Context, t Tenant) error
	CountTenants(ctx context.Context) (int, error)

	CreateJob(ctx context.Context, j ProvisioningJob) error
	UpdateJob(ctx context.Context, j ProvisioningJob) error
	GetJob(ctx context.Context, id string) (ProvisioningJob, error)
}

// MemStore is an in-memory Store for dev and tests.
type MemStore struct {
	mu      sync.RWMutex
	tenants map[string]Tenant
	jobs    map[string]ProvisioningJob
}

// NewMemStore returns an empty in-memory tenant store.
func NewMemStore() *MemStore {
	return &MemStore{tenants: map[string]Tenant{}, jobs: map[string]ProvisioningJob{}}
}

func (s *MemStore) CreateTenant(_ context.Context, t Tenant) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tenants[t.Slug]; ok {
		return ErrSlugTaken
	}
	s.tenants[t.Slug] = t
	return nil
}

func (s *MemStore) GetTenant(_ context.Context, slug string) (Tenant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tenants[slug]
	if !ok {
		return Tenant{}, ErrNotFound
	}
	return t, nil
}

func (s *MemStore) ListTenants(_ context.Context) ([]Tenant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Tenant, 0, len(s.tenants))
	for _, t := range s.tenants {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (s *MemStore) UpdateTenant(_ context.Context, t Tenant) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tenants[t.Slug]; !ok {
		return ErrNotFound
	}
	s.tenants[t.Slug] = t
	return nil
}

func (s *MemStore) CountTenants(_ context.Context) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.tenants), nil
}

func (s *MemStore) CreateJob(_ context.Context, j ProvisioningJob) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[j.ID] = j
	return nil
}

func (s *MemStore) UpdateJob(_ context.Context, j ProvisioningJob) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.jobs[j.ID]; !ok {
		return ErrNotFound
	}
	s.jobs[j.ID] = j
	return nil
}

func (s *MemStore) GetJob(_ context.Context, id string) (ProvisioningJob, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	j, ok := s.jobs[id]
	if !ok {
		return ProvisioningJob{}, ErrNotFound
	}
	return j, nil
}
