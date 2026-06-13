// Package secrets stores and retrieves downstream credentials. Values are
// write-only from the API's perspective and MUST never be logged or echoed.
package secrets

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// ErrNotFound is returned by Get when no secret exists at ref.
var ErrNotFound = errors.New("secrets: not found")

// OrgRef is the storage ref for an org-level (shared) server credential.
func OrgRef(org, serverID string) string {
	return fmt.Sprintf("org/%s/server/%s", org, serverID)
}

// UserRef is the storage ref for a per-user server credential.
func UserRef(org, serverID, userID string) string {
	return fmt.Sprintf("org/%s/server/%s/user/%s", org, serverID, userID)
}

// Store persists secret key/value maps under a ref. Implementations: Vault
// (production) and an in-memory store (dev/tests).
type Store interface {
	Put(ctx context.Context, ref string, values map[string]string) error
	Get(ctx context.Context, ref string) (map[string]string, error)
	Delete(ctx context.Context, ref string) error
}

// MemStore is an in-memory Store for dev and tests.
type MemStore struct {
	mu sync.RWMutex
	m  map[string]map[string]string
}

// NewMemStore returns an empty in-memory store.
func NewMemStore() *MemStore { return &MemStore{m: make(map[string]map[string]string)} }

// Put stores a copy of values under ref.
func (s *MemStore) Put(_ context.Context, ref string, values map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[ref] = clone(values)
	return nil
}

// Get returns a copy of the values at ref, or ErrNotFound.
func (s *MemStore) Get(_ context.Context, ref string) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.m[ref]
	if !ok {
		return nil, ErrNotFound
	}
	return clone(v), nil
}

// Delete removes the secret at ref (no error if absent).
func (s *MemStore) Delete(_ context.Context, ref string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, ref)
	return nil
}

func clone(m map[string]string) map[string]string {
	cp := make(map[string]string, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}
