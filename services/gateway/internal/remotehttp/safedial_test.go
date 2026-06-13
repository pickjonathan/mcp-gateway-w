package remotehttp

import (
	"context"
	"net"
	"strings"
	"testing"
)

func TestIsBlockedIP(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1":            true,  // loopback
		"::1":                  true,  // loopback v6
		"169.254.169.254":      true,  // link-local (cloud metadata)
		"10.0.0.5":             true,  // private
		"192.168.1.1":          true,  // private
		"172.16.0.1":           true,  // private
		"fd00::1":              true,  // IPv6 ULA (private)
		"fe80::1":              true,  // link-local v6
		"0.0.0.0":              true,  // unspecified
		"224.0.0.1":            true,  // multicast
		"8.8.8.8":              false, // public
		"1.1.1.1":              false, // public
		"2606:4700:4700::1111": false, // public v6
	}
	for s, want := range cases {
		if got := isBlockedIP(net.ParseIP(s)); got != want {
			t.Errorf("isBlockedIP(%s) = %v, want %v", s, got, want)
		}
	}
	if !isBlockedIP(nil) {
		t.Error("nil IP must be treated as blocked")
	}
}

// TestWithBlockPrivate_BlocksLoopback proves the guard refuses an admin-supplied
// endpoint that resolves to an internal address (here, loopback), while the same
// target is reachable with the guard off.
func TestWithBlockPrivate_BlocksLoopback(t *testing.T) {
	srv := fakeMCP(t)
	defer srv.Close()
	ctx := context.Background()

	blocked := New(srv.URL, WithBlockPrivate(true))
	_, err := blocked.ListTools(ctx)
	if err == nil {
		t.Fatal("expected egress to loopback to be blocked")
	}
	if !strings.Contains(err.Error(), "blocked egress") {
		t.Fatalf("expected a blocked-egress error, got %v", err)
	}

	allowed := New(srv.URL, WithBlockPrivate(false))
	if _, err := allowed.ListTools(ctx); err != nil {
		t.Fatalf("guard off must allow loopback, got %v", err)
	}
}
