package audit

import (
	"context"
	"strconv"
	"testing"
)

func TestMemLogger_ChainAndTamperEvidence(t *testing.T) {
	l := NewMemLogger()
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if err := l.Record(ctx, Event{OrgID: "acme", Action: "server.create", Target: "s" + strconv.Itoa(i)}); err != nil {
			t.Fatal(err)
		}
	}
	if ok, _ := l.Verify(ctx); !ok {
		t.Fatal("a fresh chain must verify")
	}
	// Tamper with a record in place (without re-chaining) — must be detected.
	l.records[1].Action = "server.delete"
	if ok, _ := l.Verify(ctx); ok {
		t.Fatal("tampering must break verification")
	}
}

func TestMemLogger_ListOrgScopedNewestFirst(t *testing.T) {
	l := NewMemLogger()
	ctx := context.Background()
	_ = l.Record(ctx, Event{OrgID: "acme", Action: "a"})
	_ = l.Record(ctx, Event{OrgID: "beta", Action: "b"})
	_ = l.Record(ctx, Event{OrgID: "acme", Action: "c"})

	recs, _ := l.List(ctx, "acme", 0)
	if len(recs) != 2 {
		t.Fatalf("want 2 acme records, got %d", len(recs))
	}
	if recs[0].Action != "c" {
		t.Fatalf("want newest-first (c), got %q", recs[0].Action)
	}
}
