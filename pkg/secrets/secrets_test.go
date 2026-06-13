package secrets

import (
	"context"
	"errors"
	"os"
	"testing"
)

func TestMemStore_RoundTrip(t *testing.T) {
	s := NewMemStore()
	ctx := context.Background()
	ref := OrgRef("acme", "srv-1")

	if _, err := s.Get(ctx, ref); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if err := s.Put(ctx, ref, map[string]string{"API_KEY": "shh"}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, ref)
	if err != nil || got["API_KEY"] != "shh" {
		t.Fatalf("get = %v, %v", got, err)
	}
	// returned map is a copy (mutating it must not affect the store)
	got["API_KEY"] = "tampered"
	again, _ := s.Get(ctx, ref)
	if again["API_KEY"] != "shh" {
		t.Fatal("store returned a mutable reference")
	}
	if err := s.Delete(ctx, ref); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, ref); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

// TestVaultStore_RoundTrip runs only when MCP_TEST_VAULT_ADDR is set (e.g. the
// dev Vault). Keeps `go test ./...` hermetic by default.
func TestVaultStore_RoundTrip(t *testing.T) {
	addr := os.Getenv("MCP_TEST_VAULT_ADDR")
	if addr == "" {
		t.Skip("set MCP_TEST_VAULT_ADDR (and MCP_TEST_VAULT_TOKEN) to run the Vault integration test")
	}
	s := NewVaultStore(addr, os.Getenv("MCP_TEST_VAULT_TOKEN"))
	ctx := context.Background()
	ref := OrgRef("acme", "vault-itest")
	defer func() { _ = s.Delete(ctx, ref) }()

	if err := s.Put(ctx, ref, map[string]string{"TOKEN": "v-123"}); err != nil {
		t.Fatalf("put: %v", err)
	}
	got, err := s.Get(ctx, ref)
	if err != nil || got["TOKEN"] != "v-123" {
		t.Fatalf("get = %v, %v", got, err)
	}
}
