package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const auditPrefix = "audit/"

// S3Archive is a durable, tamper-evident audit Logger backed by S3-compatible
// object storage. Each sealed record is written once as an immutable object under
// Object Lock (WORM) with a retention period — durable retention for compliance
// (T087, SOC 2). The hash chain (shared with MemLogger) makes tampering
// detectable; Object Lock makes it preventable for the retention window.
//
// The chain is a single global sequence, so writes are serialized (one writer).
// Multi-writer coordination (a shared sequence/CAS) is a follow-up.
type S3Archive struct {
	client    *minio.Client
	bucket    string
	retention time.Duration
	now       func() time.Time

	mu       sync.Mutex
	seq      int64
	lastHash string
}

// NewS3Archive connects to an S3-compatible endpoint, ensures an Object-Lock
// bucket exists, rebuilds the chain tip from storage, and returns the archive.
func NewS3Archive(ctx context.Context, endpoint, accessKey, secretKey, bucket string, useSSL bool, retention time.Duration) (*S3Archive, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, err
	}
	a := &S3Archive{client: client, bucket: bucket, retention: retention, now: time.Now}
	if err := a.ensureBucket(ctx); err != nil {
		return nil, err
	}
	if err := a.loadTip(ctx); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *S3Archive) ensureBucket(ctx context.Context) error {
	exists, err := a.client.BucketExists(ctx, a.bucket)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	// ObjectLocking enables versioning + object lock (required for WORM retention).
	return a.client.MakeBucket(ctx, a.bucket, minio.MakeBucketOptions{ObjectLocking: true})
}

// loadTip scans existing records to recover the chain head (seq + last hash) so a
// restarted writer continues the same chain (durability across restarts).
func (a *S3Archive) loadTip(ctx context.Context) error {
	var maxSeq int64
	var tipKey string
	for obj := range a.client.ListObjects(ctx, a.bucket, minio.ListObjectsOptions{Prefix: auditPrefix, Recursive: true}) {
		if obj.Err != nil {
			return obj.Err
		}
		if s := seqFromKey(obj.Key); s > maxSeq {
			maxSeq, tipKey = s, obj.Key
		}
	}
	a.seq = maxSeq
	if tipKey == "" {
		return nil
	}
	r, err := a.getRecord(ctx, tipKey)
	if err != nil {
		return err
	}
	a.lastHash = r.Hash
	return nil
}

// Record seals e into the chain and writes it as an immutable, retention-locked
// object.
func (a *S3Archive) Record(ctx context.Context, e Event) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if e.Time.IsZero() {
		e.Time = a.now()
	}
	r := Record{Event: e, Seq: a.seq + 1, PrevHash: a.lastHash}
	r.Hash = hashRecord(r)
	body, err := json.Marshal(r)
	if err != nil {
		return err
	}
	opts := minio.PutObjectOptions{ContentType: "application/json"}
	if a.retention > 0 {
		opts.Mode = minio.Compliance
		opts.RetainUntilDate = a.now().Add(a.retention)
	}
	if _, err := a.client.PutObject(ctx, a.bucket, keyForSeq(r.Seq), bytes.NewReader(body), int64(len(body)), opts); err != nil {
		return err
	}
	a.seq, a.lastHash = r.Seq, r.Hash
	return nil
}

// List returns up to limit records for org from storage, newest first.
func (a *S3Archive) List(ctx context.Context, org string, limit int) ([]Record, error) {
	recs, err := a.all(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Record, 0)
	for i := len(recs) - 1; i >= 0; i-- {
		if recs[i].OrgID != org {
			continue
		}
		out = append(out, recs[i])
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

// Verify recomputes the chain from storage and reports whether it is intact.
func (a *S3Archive) Verify(ctx context.Context) (bool, error) {
	recs, err := a.all(ctx)
	if err != nil {
		return false, err
	}
	prev := ""
	for _, r := range recs {
		if r.PrevHash != prev || r.Hash != hashRecord(r) {
			return false, nil
		}
		prev = r.Hash
	}
	return true, nil
}

func (a *S3Archive) all(ctx context.Context) ([]Record, error) {
	var recs []Record
	for obj := range a.client.ListObjects(ctx, a.bucket, minio.ListObjectsOptions{Prefix: auditPrefix, Recursive: true}) {
		if obj.Err != nil {
			return nil, obj.Err
		}
		r, err := a.getRecord(ctx, obj.Key)
		if err != nil {
			return nil, err
		}
		recs = append(recs, r)
	}
	sort.Slice(recs, func(i, j int) bool { return recs[i].Seq < recs[j].Seq })
	return recs, nil
}

func (a *S3Archive) getRecord(ctx context.Context, key string) (Record, error) {
	obj, err := a.client.GetObject(ctx, a.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return Record{}, err
	}
	defer func() { _ = obj.Close() }()
	var r Record
	if err := json.NewDecoder(obj).Decode(&r); err != nil {
		return Record{}, err
	}
	return r, nil
}

// keyForSeq zero-pads so lexical object order matches sequence order.
func keyForSeq(seq int64) string { return fmt.Sprintf("%s%020d.json", auditPrefix, seq) }

func seqFromKey(key string) int64 {
	s := strings.TrimSuffix(strings.TrimPrefix(key, auditPrefix), ".json")
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return n
}
