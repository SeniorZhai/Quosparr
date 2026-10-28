package tui

import (
	"context"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/licoy/quosparr/internal/claude"
	"github.com/licoy/quosparr/internal/grok"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/livespeed"
	"github.com/licoy/quosparr/internal/netx"
	"github.com/licoy/quosparr/internal/platform"
	"github.com/licoy/quosparr/internal/runway"
	"github.com/licoy/quosparr/internal/store"
)

type providerReading struct {
	meters []runway.Meter
	status string
}

// Runtime reads quota independently of the desktop app and never renews tokens.
type Runtime struct {
	mu                sync.Mutex
	config            Config
	lc                l10n.Localizer
	readings          map[store.Provider]providerReading
	last, next        time.Time
	refreshing        bool
	request           chan struct{}
	done              chan struct{}
	cancel            context.CancelFunc
	live              *livespeed.Monitor
	network           *netx.Context
	history           map[store.Provider]TokenHistory
	historyRefreshing bool
	historyRequest    chan struct{}
	historyDone       chan struct{}
	historyCancel     context.CancelFunc
}

func Start(ctx context.Context, config Config, home string) *Runtime {
	network := netx.SystemContext()
	ctx = netx.WithContext(ctx, network)
	homes := livespeed.Homes{}
	for _, p := range config.Providers {
		switch p {
		case store.ProviderCodex:
			homes.Codex = platform.CodexDir(home)
		case store.ProviderGrok:
			homes.Grok = grok.ResolveOfficialHome(os.Getenv("GROK_HOME"), home)
		case store.ProviderClaude:
			dir := claude.ResolveConfigDir(os.Getenv("CLAUDE_CONFIG_DIR"), home)
			homes.ClaudeProjects = []string{filepath.Join(dir, "projects")}
		}
	}
	r := &Runtime{live: livespeed.Start(homes), network: network}
	r.start(ctx, config, func(ctx context.Context, p store.Provider) providerReading {
		return readProvider(ctx, home, p, config.Accounts[p], l10n.New(config.Language), time.Now())
	})
	r.startHistory(ctx, newHistoryReader(home).Read)
	return r
}

func (r *Runtime) start(ctx context.Context, config Config, fetch func(context.Context, store.Provider) providerReading) {
	ctx, r.cancel = context.WithCancel(ctx)
	config.Providers = slices.Clone(config.Providers)
	r.config, r.lc = config, l10n.New(config.Language)
	r.readings = make(map[store.Provider]providerReading, len(config.Providers))
	r.request, r.done = make(chan struct{}, 1), make(chan struct{})
	for _, p := range config.Providers {
		r.readings[p] = providerReading{status: r.lc.Text(l10n.KeyStatusWait)}
	}
	r.Refresh()
	go func() {
		defer close(r.done)
		defer r.live.Close()
		if r.network != nil {
			defer r.network.CloseIdleConnections()
		}
		timer := time.NewTimer(time.Hour)
		timer.Stop()
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-r.request:
			case <-timer.C:
			}
			if ctx.Err() != nil {
				return
			}
			timer.Stop()
			r.mu.Lock()
			r.refreshing, r.next = true, time.Time{}
			select {
			case <-r.request:
			default:
			}
			r.mu.Unlock()
			for _, p := range config.Providers {
				request, cancel := context.WithTimeout(ctx, 30*time.Second)
				reading := fetch(request, p)
				cancel()
				if ctx.Err() != nil {
					return
				}
				r.mu.Lock()
				r.readings[p] = reading
				r.mu.Unlock()
			}
			r.mu.Lock()
			r.last = time.Now()
			interval := time.Duration(config.IntervalSeconds) * time.Second
			r.next, r.refreshing = r.last.Add(interval), false
			r.mu.Unlock()
			timer.Reset(interval)
		}
	}()
}

func (r *Runtime) Refresh() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.historyRequest != nil && !r.historyRefreshing {
		r.historyRefreshing = true
		select {
		case r.historyRequest <- struct{}{}:
		default:
		}
	}
	if r.refreshing {
		return
	}
	r.refreshing, r.next = true, time.Time{}
	select {
	case r.request <- struct{}{}:
	default:
	}
}

func (r *Runtime) Close() {
	if r.historyCancel != nil {
		r.historyCancel()
	}
	r.cancel()
	<-r.done
	if r.historyDone != nil {
		<-r.historyDone
	}
}

func (r *Runtime) View() View {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	view := View{SourceAt: now, LastRefresh: r.last, NextRefresh: r.next, History: maps.Clone(r.history),
		IntervalSeconds: r.config.IntervalSeconds, Refreshing: r.refreshing}
	live := r.live.Current()
	for _, p := range r.config.Providers {
		reading := r.readings[p]
		in := runway.StatusInputs{Now: now, Provider: p,
			Prefs:       store.Preferences{FloatingWindow: store.FloatingWindow{Providers: []store.Provider{p}}},
			CodexMeters: reading.meters, Grok: runway.GrokState{Meters: reading.meters},
			Claude: runway.ClaudeState{Meters: reading.meters},
			Remote: map[store.Provider]runway.RemoteQuota{p: {Meters: reading.meters}},
		}
		for _, part := range live {
			if part.Source == string(p) {
				in.Live = append(in.Live, runway.LivePart{Provider: p, Text: part.Text, Pending: part.Pending, Last: part.Last})
			}
		}
		floating := runway.BuildFloating(r.lc, in)
		for i := range floating.Providers {
			if len(floating.Providers[i].Meters) == 0 {
				floating.Providers[i].Status = reading.status
			}
		}
		view.Floating.Live = view.Floating.Live || floating.Live
		view.Floating.Providers = append(view.Floating.Providers, floating.Providers...)
	}
	return view
}
