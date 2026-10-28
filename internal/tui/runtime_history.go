package tui

import (
	"context"
	"time"

	"github.com/licoy/quosparr/internal/store"
)

// Log scans have their own worker so a large history cannot delay quota requests.
func (r *Runtime) startHistory(ctx context.Context, read func(context.Context, store.Provider, time.Time) TokenHistory) {
	ctx, r.historyCancel = context.WithCancel(ctx)
	r.historyDone = make(chan struct{})
	r.mu.Lock()
	r.history = make(map[store.Provider]TokenHistory)
	r.historyRequest = make(chan struct{}, 1)
	r.historyRefreshing = true
	var providers []store.Provider
	for _, p := range r.config.Providers {
		if !p.RemoteQuota() {
			providers = append(providers, p)
			r.history[p] = TokenHistory{Loading: true}
		}
	}
	interval := time.Duration(max(300, r.config.IntervalSeconds)) * time.Second
	r.mu.Unlock()
	go func() {
		defer close(r.historyDone)
		timer := time.NewTimer(interval)
		defer timer.Stop()
		for {
			if ctx.Err() != nil {
				return
			}
			r.mu.Lock()
			r.historyRefreshing = true
			select {
			case <-r.historyRequest:
			default:
			}
			for _, p := range providers {
				h := r.history[p]
				h.Loading = true
				r.history[p] = h
			}
			r.mu.Unlock()
			for _, p := range providers {
				history := read(ctx, p, time.Now())
				if ctx.Err() != nil {
					return
				}
				r.mu.Lock()
				r.history[p] = history
				r.mu.Unlock()
			}
			r.mu.Lock()
			r.historyRefreshing = false
			r.mu.Unlock()
			timer.Reset(interval)
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
			case <-r.historyRequest:
				timer.Stop()
			}
		}
	}()
}
