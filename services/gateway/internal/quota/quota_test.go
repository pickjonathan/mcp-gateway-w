package quota

import (
	"testing"
	"time"
)

func TestEnforcer_OrgLimit(t *testing.T) {
	e := NewEnforcer(2, 0, time.Minute)
	if !e.Allow("acme", "u1") || !e.Allow("acme", "u2") {
		t.Fatal("first two requests should pass")
	}
	if e.Allow("acme", "u3") {
		t.Fatal("third request should be org-limited")
	}
	if !e.Allow("beta", "u1") {
		t.Fatal("a different org must be unaffected (noisy-neighbor isolation)")
	}
}

func TestEnforcer_UserLimit(t *testing.T) {
	e := NewEnforcer(0, 1, time.Minute)
	if !e.Allow("acme", "u1") {
		t.Fatal("first request should pass")
	}
	if e.Allow("acme", "u1") {
		t.Fatal("second request from the same user should be limited")
	}
	if !e.Allow("acme", "u2") {
		t.Fatal("a different user must be unaffected")
	}
}

func TestEnforcer_NilUnlimited(t *testing.T) {
	var e *Enforcer
	if !e.Allow("x", "y") {
		t.Fatal("nil enforcer must be unlimited")
	}
}
