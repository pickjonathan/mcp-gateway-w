// Package pool implements a warm pool of reusable sandbox instances with
// on-demand assignment and idle scale-down (T048). It cuts cold-start latency by
// keeping MinWarm instances pre-created, caps concurrency at MaxSize, and reaps
// idle instances back toward MinWarm (to zero when MinWarm is 0).
//
// It is deliberately decoupled from any concrete sandbox backend: instances are
// produced by a Factory and only need to Close. This keeps the pool logic
// hermetically testable while the real Firecracker/Kata/gVisor backend plugs in
// behind the Factory.
package pool

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrExhausted is returned by Acquire when the pool is at MaxSize with no idle
// instance available.
var ErrExhausted = errors.New("pool: exhausted")

// Instance is a poolable resource (e.g. a running microVM sandbox).
type Instance interface {
	Close() error
}

// Factory creates a new Instance.
type Factory func(ctx context.Context) (Instance, error)

// Config configures a Pool.
type Config struct {
	MinWarm     int           // instances kept pre-created and idle (0 = scale to zero)
	MaxSize     int           // hard cap on total instances (warm + in use)
	IdleTimeout time.Duration // idle instances beyond MinWarm are reaped after this
	Factory     Factory
}

type pooledInstance struct {
	inst     Instance
	lastUsed time.Time
}

// Pool is a concurrency-safe warm pool.
type Pool struct {
	mu    sync.Mutex
	cfg   Config
	now   func() time.Time
	warm  []*pooledInstance // idle, ready for assignment
	inUse map[Instance]*pooledInstance
}

// Stats is a point-in-time snapshot of pool occupancy.
type Stats struct {
	Warm  int
	InUse int
	Total int
}

// New builds a pool. MaxSize < 1 is treated as 1.
func New(cfg Config) *Pool {
	if cfg.MaxSize < 1 {
		cfg.MaxSize = 1
	}
	if cfg.MinWarm > cfg.MaxSize {
		cfg.MinWarm = cfg.MaxSize
	}
	return &Pool{cfg: cfg, now: time.Now, inUse: make(map[Instance]*pooledInstance)}
}

// Prewarm creates instances until MinWarm are ready. Call at startup.
func (p *Pool) Prewarm(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for len(p.warm)+len(p.inUse) < p.cfg.MinWarm {
		inst, err := p.cfg.Factory(ctx)
		if err != nil {
			return err
		}
		p.warm = append(p.warm, &pooledInstance{inst: inst, lastUsed: p.now()})
	}
	return nil
}

// Acquire returns a ready instance: a warm one if available, otherwise a freshly
// created one (up to MaxSize). Returns ErrExhausted at capacity.
func (p *Pool) Acquire(ctx context.Context) (Instance, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if n := len(p.warm); n > 0 {
		pi := p.warm[n-1]
		p.warm = p.warm[:n-1]
		pi.lastUsed = p.now()
		p.inUse[pi.inst] = pi
		return pi.inst, nil
	}
	if len(p.inUse) >= p.cfg.MaxSize { // warm is empty here, so inUse == total
		return nil, ErrExhausted
	}
	inst, err := p.cfg.Factory(ctx)
	if err != nil {
		return nil, err
	}
	p.inUse[inst] = &pooledInstance{inst: inst, lastUsed: p.now()}
	return inst, nil
}

// Release returns an instance to the warm set. Unknown instances (or a double
// release) are ignored.
func (p *Pool) Release(inst Instance) {
	p.mu.Lock()
	defer p.mu.Unlock()
	pi, ok := p.inUse[inst]
	if !ok {
		return
	}
	delete(p.inUse, inst)
	pi.lastUsed = p.now()
	p.warm = append(p.warm, pi)
}

// Reap closes idle warm instances that have been idle longer than IdleTimeout,
// keeping at least MinWarm ready (warm + in use). With MinWarm 0 it can drain to
// zero. Returns the number reaped. Drive it from a ticker, or call directly.
func (p *Pool) Reap() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	cutoff := p.now().Add(-p.cfg.IdleTimeout)
	kept := make([]*pooledInstance, 0, len(p.warm))
	reaped := 0
	for _, pi := range p.warm {
		belowFloor := len(kept)+len(p.inUse) < p.cfg.MinWarm
		if belowFloor || pi.lastUsed.After(cutoff) {
			kept = append(kept, pi)
			continue
		}
		_ = pi.inst.Close()
		reaped++
	}
	p.warm = kept
	return reaped
}

// Stats returns current occupancy.
func (p *Pool) Stats() Stats {
	p.mu.Lock()
	defer p.mu.Unlock()
	return Stats{Warm: len(p.warm), InUse: len(p.inUse), Total: len(p.warm) + len(p.inUse)}
}

// Close terminates every instance (warm and in use).
func (p *Pool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, pi := range p.warm {
		_ = pi.inst.Close()
	}
	for _, pi := range p.inUse {
		_ = pi.inst.Close()
	}
	p.warm = nil
	p.inUse = make(map[Instance]*pooledInstance)
	return nil
}
