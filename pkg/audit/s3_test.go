package audit

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
)

// TestS3Archive_DurableAndWORM runs only when MCP_TEST_S3_ENDPOINT is set (e.g.
// the dev MinIO). It proves the archive is durable (records survive and the chain
// verifies from storage, including after a "restart") and write-once (a sealed
// record version cannot be deleted within its Object-Lock retention).
func TestS3Archive_DurableAndWORM(t *testing.T) {
	endpoint := os.Getenv("MCP_TEST_S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("set MCP_TEST_S3_ENDPOINT (+ optional keys) to run the S3 audit archive test")
	}
	ak := envOr("MCP_TEST_S3_ACCESS_KEY", "minioadmin")
	sk := envOr("MCP_TEST_S3_SECRET_KEY", "minioadmin")
	ctx := context.Background()
	// Fixed object-lock bucket; a unique org per run keeps List assertions exact
	// without needing to delete (impossible) locked objects.
	const bucket = "mcp-audit-itest"
	org := fmt.Sprintf("org-%d", time.Now().UnixNano())

	a, err := NewS3Archive(ctx, endpoint, ak, sk, bucket, false, time.Minute)
	if err != nil {
		t.Fatalf("new archive: %v", err)
	}
	base := a.seq

	for i := 0; i < 3; i++ {
		if err := a.Record(ctx, Event{OrgID: org, Actor: "admin", Action: "server.create", Target: fmt.Sprintf("s%d", i)}); err != nil {
			t.Fatalf("record %d: %v", i, err)
		}
	}

	// Durable + tamper-evident: chain verifies when read back from storage.
	if ok, err := a.Verify(ctx); err != nil || !ok {
		t.Fatalf("verify: ok=%v err=%v", ok, err)
	}

	// List is org-scoped and newest-first.
	recs, err := a.List(ctx, org, 0)
	if err != nil || len(recs) != 3 {
		t.Fatalf("list: got %d recs, err=%v", len(recs), err)
	}
	if recs[0].Target != "s2" {
		t.Fatalf("expected newest-first (s2), got %q", recs[0].Target)
	}

	// Durability across restart: a fresh archive over the same bucket recovers the
	// chain tip and continues the sequence.
	a2, err := NewS3Archive(ctx, endpoint, ak, sk, bucket, false, time.Minute)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if a2.seq != base+3 {
		t.Fatalf("tip not rebuilt from storage: seq=%d want %d", a2.seq, base+3)
	}

	// WORM: the sealed version cannot be deleted within retention (COMPLIANCE).
	key := keyForSeq(base + 1)
	info, err := a.client.StatObject(ctx, bucket, key, minio.StatObjectOptions{})
	if err != nil {
		t.Fatalf("stat sealed object: %v", err)
	}
	if err := a.client.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{VersionID: info.VersionID}); err == nil {
		t.Fatal("WORM breach: deleting the locked audit object version succeeded")
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
