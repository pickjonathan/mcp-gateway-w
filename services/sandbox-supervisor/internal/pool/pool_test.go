package pool

import (
	"context"
	"testing"
	"time"
)

type fakeInstance struct {
	id     int
	closed bool
}

func (f *fakeInstance) Close() error { f.closed = true; return nil }

// fakeFactory returns instances and counts creations.
func fakeFactory(created *int) Factory {
	return func(context.Context) (Instance, error) {
		*created++
		return &fakeInstance{id: *created}, nil
	}
}

func TestPool_ReuseAndCapacity(t *testing.T) {
	created := 0
	p := New(Config{MaxSize: 2, Factory: fakeFactory(&created)})
	ctx := context.Background()

	a, err := p.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	b, err := p.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if created != 2 {
		t.Fatalf("expected 2 creations, got %d", created)
	}
	if _, err := p.Acquire(ctx); err != ErrExhausted {
		t.Fatalf("expected ErrExhausted at capacity, got %v", err)
	}

	// Releasing makes one available; the next Acquire reuses it (no new creation).
	p.Release(a)
	reused, err := p.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if created != 2 {
		t.Fatalf("Acquire after Release must reuse, not create; creations=%d", created)
	}
	if reused != a {
		t.Fatal("expected the released instance to be reused")
	}
	_ = b
}

func TestPool_Prewarm(t *testing.T) {
	created := 0
	p := New(Config{MinWarm: 2, MaxSize: 5, Factory: fakeFactory(&created)})
	if err := p.Prewarm(context.Background()); err != nil {
		t.Fatal(err)
	}
	if s := p.Stats(); s.Warm != 2 || s.InUse != 0 {
		t.Fatalf("after prewarm want warm=2 inUse=0, got %+v", s)
	}
	// Acquire reuses a pre-warmed instance — no cold start.
	if _, err := p.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	if created != 2 {
		t.Fatalf("acquire after prewarm must reuse; creations=%d", created)
	}
}

func TestPool_ReapToMinWarm(t *testing.T) {
	created := 0
	clock := time.Unix(1_000_000, 0)
	p := New(Config{MinWarm: 1, MaxSize: 5, IdleTimeout: time.Minute, Factory: fakeFactory(&created)})
	p.now = func() time.Time { return clock }
	ctx := context.Background()

	// Create 3 warm instances (acquire then release).
	var insts []Instance
	for i := 0; i < 3; i++ {
		in, _ := p.Acquire(ctx)
		insts = append(insts, in)
	}
	for _, in := range insts {
		p.Release(in)
	}
	if s := p.Stats(); s.Warm != 3 {
		t.Fatalf("want 3 warm, got %+v", s)
	}

	// Before the idle timeout elapses, nothing is reaped.
	if n := p.Reap(); n != 0 {
		t.Fatalf("nothing should be reaped before idle timeout, reaped %d", n)
	}

	// Advance past the idle timeout: reap down to MinWarm (1), closing the rest.
	clock = clock.Add(2 * time.Minute)
	if n := p.Reap(); n != 2 {
		t.Fatalf("expected to reap 2 down to MinWarm, reaped %d", n)
	}
	if s := p.Stats(); s.Warm != 1 {
		t.Fatalf("want 1 warm after reap, got %+v", s)
	}
	closed := 0
	for _, in := range insts {
		if in.(*fakeInstance).closed {
			closed++
		}
	}
	if closed != 2 {
		t.Fatalf("expected 2 reaped instances closed, got %d", closed)
	}
}

func TestPool_ScaleToZero(t *testing.T) {
	created := 0
	clock := time.Unix(2_000_000, 0)
	p := New(Config{MinWarm: 0, MaxSize: 3, IdleTimeout: time.Minute, Factory: fakeFactory(&created)})
	p.now = func() time.Time { return clock }
	ctx := context.Background()

	in, _ := p.Acquire(ctx)
	p.Release(in)
	clock = clock.Add(2 * time.Minute)
	if n := p.Reap(); n != 1 {
		t.Fatalf("expected scale-to-zero to reap 1, reaped %d", n)
	}
	if s := p.Stats(); s.Total != 0 {
		t.Fatalf("expected pool drained to zero, got %+v", s)
	}
}

func TestPool_Close(t *testing.T) {
	created := 0
	p := New(Config{MaxSize: 3, Factory: fakeFactory(&created)})
	ctx := context.Background()
	a, _ := p.Acquire(ctx)
	b, _ := p.Acquire(ctx)
	p.Release(b)
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if !a.(*fakeInstance).closed || !b.(*fakeInstance).closed {
		t.Fatal("Close must terminate both in-use and warm instances")
	}
	if s := p.Stats(); s.Total != 0 {
		t.Fatalf("after Close want empty, got %+v", s)
	}
}
