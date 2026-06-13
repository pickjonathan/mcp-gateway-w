// Package audit records configuration and security-relevant events in a
// tamper-evident, hash-chained log (FR-010, Constitution VI). Each record links
// to the previous via its hash, so any edit/insertion/reorder breaks the chain.
// The in-memory logger is for dev/tests; a durable, Object-Lock'd archive is the
// production backing (T087) behind the same Logger interface.
package audit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Event is a single auditable action. Metadata MUST never contain secret values.
type Event struct {
	Time     time.Time         `json:"time"`
	OrgID    string            `json:"org_id"`
	Actor    string            `json:"actor"`
	Action   string            `json:"action"`
	Target   string            `json:"target"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Record is a sealed Event with its position and hash chain.
type Record struct {
	Event
	Seq      int64  `json:"seq"`
	PrevHash string `json:"prev_hash"`
	Hash     string `json:"hash"`
}

// Logger records and queries audit events.
type Logger interface {
	Record(ctx context.Context, e Event) error
	List(ctx context.Context, org string, limit int) ([]Record, error)
	Verify(ctx context.Context) (bool, error)
}

// MemLogger is an in-memory, append-only, hash-chained Logger.
type MemLogger struct {
	mu      sync.Mutex
	records []Record
	now     func() time.Time
}

// NewMemLogger returns an empty in-memory audit logger.
func NewMemLogger() *MemLogger { return &MemLogger{now: time.Now} }

// Record appends e to the chain.
func (l *MemLogger) Record(_ context.Context, e Event) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if e.Time.IsZero() {
		e.Time = l.now()
	}
	prev := ""
	if n := len(l.records); n > 0 {
		prev = l.records[n-1].Hash
	}
	r := Record{Event: e, Seq: int64(len(l.records)) + 1, PrevHash: prev}
	r.Hash = hashRecord(r)
	l.records = append(l.records, r)
	return nil
}

// List returns up to limit records for org, newest first (limit <= 0 = all).
func (l *MemLogger) List(_ context.Context, org string, limit int) ([]Record, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]Record, 0)
	for i := len(l.records) - 1; i >= 0; i-- {
		if l.records[i].OrgID != org {
			continue
		}
		out = append(out, l.records[i])
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

// Verify recomputes the chain and reports whether it is intact (tamper-evidence).
func (l *MemLogger) Verify(_ context.Context) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	prev := ""
	for _, r := range l.records {
		if r.PrevHash != prev || r.Hash != hashRecord(r) {
			return false, nil
		}
		prev = r.Hash
	}
	return true, nil
}

func hashRecord(r Record) string {
	h := sha256.New()
	fmt.Fprintf(h, "%d\n%s\n%s\n%s\n%s\n%s\n%d\n",
		r.Seq, r.PrevHash, r.OrgID, r.Actor, r.Action, r.Target, r.Time.UnixNano())
	keys := make([]string, 0, len(r.Metadata))
	for k := range r.Metadata {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(h, "%s=%s\n", k, r.Metadata[k])
	}
	return hex.EncodeToString(h.Sum(nil))
}
