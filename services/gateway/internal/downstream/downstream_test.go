package downstream

import (
	"errors"
	"testing"
)

type closableFake struct {
	*Fake
	closed bool
}

func (c *closableFake) Close() error { c.closed = true; return nil }

// TestRegistry_RemoveClosesDownstream verifies the kill-switch terminates a
// running downstream (stdio sandbox process) on removal (US7 / SC-004).
func TestRegistry_RemoveClosesDownstream(t *testing.T) {
	r := NewRegistry()
	c := &closableFake{Fake: &Fake{}}
	r.Add("x", c)
	r.Remove("x")
	if !c.closed {
		t.Fatal("Remove must Close a downstream that implements io.Closer")
	}
	if _, ok := r.Get("x"); ok {
		t.Fatal("server still present after Remove")
	}
}

// TestRegistry_PerUserProvider verifies per_user mode (US6): each user gets an
// isolated, cached instance built from the provider; users without credentials
// get an error; and the kill-switch closes every per-user instance.
func TestRegistry_PerUserProvider(t *testing.T) {
	r := NewRegistry()
	built := map[string]int{}
	instances := map[string]*closableFake{}
	r.AddProvider("srv", func(user string) (Downstream, error) {
		if user == "no-creds" {
			return nil, errors.New("no credentials configured")
		}
		built[user]++
		inst := &closableFake{Fake: &Fake{}}
		instances[user] = inst
		return inst, nil
	}, nil)

	// First call builds; second call reuses the cached instance.
	a1, err := r.GetForUser("srv", "alice")
	if err != nil {
		t.Fatalf("alice: %v", err)
	}
	a2, err := r.GetForUser("srv", "alice")
	if err != nil {
		t.Fatalf("alice (cached): %v", err)
	}
	if a1 != a2 {
		t.Fatal("expected the per-user instance to be cached and reused")
	}
	if built["alice"] != 1 {
		t.Fatalf("expected exactly 1 build for alice, got %d", built["alice"])
	}

	// A different user gets an isolated instance (no credential bleed).
	b1, err := r.GetForUser("srv", "bob")
	if err != nil {
		t.Fatalf("bob: %v", err)
	}
	if b1 == a1 {
		t.Fatal("alice and bob must get isolated per-user instances")
	}

	// A user without credentials gets the provider error and nothing is cached.
	if _, err := r.GetForUser("srv", "no-creds"); err == nil {
		t.Fatal("expected an error for a user with no credentials")
	}

	// Unknown slug → ErrNotFound.
	if _, err := r.GetForUser("absent", "alice"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for unknown slug, got %v", err)
	}

	// Kill-switch closes every per-user instance.
	r.Remove("srv")
	if !instances["alice"].closed || !instances["bob"].closed {
		t.Fatal("Remove must close every per-user instance")
	}
}

// TestRegistry_Invalidate verifies credential rotation (T079): invalidating a
// user's cached instance closes it and forces a rebuild (with the new secret) on
// the next access.
func TestRegistry_Invalidate(t *testing.T) {
	r := NewRegistry()
	builds := 0
	var instances []*closableFake
	r.AddProvider("srv", func(string) (Downstream, error) {
		builds++
		inst := &closableFake{Fake: &Fake{}}
		instances = append(instances, inst)
		return inst, nil
	}, nil)

	first, _ := r.GetForUser("srv", "alice")
	if builds != 1 {
		t.Fatalf("expected 1 build, got %d", builds)
	}

	r.Invalidate("srv", "alice")
	if !instances[0].closed {
		t.Fatal("Invalidate must close the dropped instance")
	}

	second, _ := r.GetForUser("srv", "alice")
	if builds != 2 {
		t.Fatalf("expected a rebuild after Invalidate, got %d builds", builds)
	}
	if first == second {
		t.Fatal("expected a freshly built instance after Invalidate")
	}
}

// TestRegistry_ReplaceClosesPrevious verifies that re-registering a slug closes
// the previous instance — the mechanism org-credential rotation relies on (the
// upsert rebuild must not leak the old client).
func TestRegistry_ReplaceClosesPrevious(t *testing.T) {
	r := NewRegistry()
	old := &closableFake{Fake: &Fake{}}
	r.Add("x", old)
	r.Add("x", &closableFake{Fake: &Fake{}}) // replace
	if !old.closed {
		t.Fatal("replacing a slug must close the previous instance")
	}
}
