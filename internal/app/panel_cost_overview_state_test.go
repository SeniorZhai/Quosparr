package app

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/store"
	"github.com/licoy/quosparr/internal/usage"
)

func TestCostOverviewCachesAcrossProvidersAndRefreshesDatesAndPrices(t *testing.T) {
	queue := detailMainQueue(t)
	var calls atomic.Int32
	version := "one"
	acts := PanelActions{
		PricingVersion: func(store.Provider) string { return version },
		QueryCostOverview: func(context.Context) (usage.CostOverview, error) {
			calls.Add(1)
			return usage.CostOverview{}, nil
		},
	}
	a, tt := newPanel(t, nil, panelFixture(), acts)
	t.Cleanup(a.cancelCostOverview)
	now := time.Now()
	a.syncCostOverview(acts, now)
	for a.panelState.costOverview.task.Running {
		finishDetail(t, queue)
	}
	if a.panelState.costOverview.report == nil || calls.Load() != 1 {
		t.Fatal("initial overview did not load")
	}
	a.settings.SetSelectedProvider(store.ProviderGrok)
	tt.Frame()
	a.syncCostOverview(acts, now.Add(time.Second))
	if calls.Load() != 1 || a.panelState.costOverview.report == nil {
		t.Fatal("provider switch discarded the all-provider summary")
	}
	version = "two"
	a.syncCostOverview(acts, now.Add(2*time.Second))
	if a.panelState.costOverview.report != nil {
		t.Fatal("old price remained visible")
	}
	for a.panelState.costOverview.task.Running {
		finishDetail(t, queue)
	}
	if calls.Load() != 2 {
		t.Fatal("price change did not refresh")
	}
	a.syncCostOverview(acts, now.AddDate(0, 0, 1))
	for a.panelState.costOverview.task.Running {
		finishDetail(t, queue)
	}
	if calls.Load() != 3 {
		t.Fatal("local date change did not refresh")
	}
}

func TestCostOverviewCancelsStaleResultsAndThrottlesErrors(t *testing.T) {
	queue := detailMainQueue(t)
	started := make(chan context.Context, 2)
	var calls atomic.Int32
	acts := PanelActions{QueryCostOverview: func(ctx context.Context) (usage.CostOverview, error) {
		if calls.Add(1) == 1 {
			started <- ctx
			<-ctx.Done()
			return usage.CostOverview{}, nil
		}
		return usage.CostOverview{}, errors.New("unavailable")
	}}
	a, _ := newPanel(t, nil, panelFixture(), acts)
	t.Cleanup(a.cancelCostOverview)
	now := time.Now()
	a.startCostOverview(acts, now)
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("query did not start")
	}
	a.startCostOverview(acts, now)
	finishDetail(t, queue)
	finishDetail(t, queue)
	s := &a.panelState.costOverview
	if s.task.Running || s.task.Err == nil || s.report != nil || calls.Load() != 2 {
		t.Fatalf("stale completion or hidden error: %+v", s)
	}
	a.syncCostOverview(acts, now.Add(time.Second))
	if calls.Load() != 2 {
		t.Fatal("error retried on every redraw")
	}
	a.syncCostOverview(acts, now.Add(costOverviewTTL+time.Second))
	finishDetail(t, queue)
	if calls.Load() != 3 {
		t.Fatal("error never retried")
	}
}
