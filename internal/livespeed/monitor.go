package livespeed

import (
	"context"
	"sync"
	"time"
)

// Monitor rereads local logs twice a second and keeps the latest rate.
type Monitor struct {
	homes  Homes
	ctx    context.Context
	cancel context.CancelFunc

	mu    sync.Mutex
	parts []Part
}

// Start begins sampling. Current is safe to call before the first sample finishes.
func Start(homes Homes) *Monitor {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Monitor{homes: homes, ctx: ctx, cancel: cancel}
	go m.loop()
	return m
}

// Close stops sampling.
func (m *Monitor) Close() {
	if m != nil {
		m.cancel()
	}
}

// Current is one entry per source that has a live rate. Empty keeps the quota.
func (m *Monitor) Current() []Part {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.parts) == 0 {
		return nil
	}
	out := make([]Part, len(m.parts))
	copy(out, m.parts)
	return out
}

func (m *Monitor) loop() {
	m.sample(time.Now())
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case now := <-tick.C:
			m.sample(now)
		}
	}
}

func (m *Monitor) sample(now time.Time) {
	if m.ctx.Err() != nil {
		return
	}
	responses, flights := Read(now, m.homes)
	parts := Parts(now, responses, flights)
	m.mu.Lock()
	m.parts = parts
	m.mu.Unlock()
}
