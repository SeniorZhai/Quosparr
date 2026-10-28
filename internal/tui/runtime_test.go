package tui

import (
	"context"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/runway"
	"github.com/licoy/quosparr/internal/store"
)

func TestRuntimeRefreshScheduleAndCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var calls atomic.Int32
		release := make(chan struct{})
		r := &Runtime{}
		r.start(context.Background(), Config{Providers: []store.Provider{store.ProviderCodex}, IntervalSeconds: 60, Language: l10n.English},
			func(ctx context.Context, _ store.Provider) providerReading {
				calls.Add(1)
				select {
				case <-release:
				case <-ctx.Done():
				}
				return providerReading{status: "ready"}
			})
		synctest.Wait()
		if calls.Load() != 1 || !r.View().Refreshing || !r.View().NextRefresh.IsZero() {
			t.Fatal("startup must begin one refresh without a guessed deadline")
		}
		for range 5 {
			r.Refresh()
		}
		time.Sleep(2 * time.Second)
		release <- struct{}{}
		synctest.Wait()
		first := r.View()
		if calls.Load() != 1 || first.Refreshing || !first.LastRefresh.Equal(time.Now()) || first.NextRefresh.Sub(first.LastRefresh) != time.Minute {
			t.Fatalf("completed refresh did not arm its next deadline: %+v", first)
		}
		time.Sleep(10 * time.Second)
		r.Refresh()
		synctest.Wait()
		if calls.Load() != 2 || !r.View().Refreshing {
			t.Fatal("manual refresh was not immediate")
		}
		release <- struct{}{}
		synctest.Wait()
		manual := r.View()
		if manual.NextRefresh.Sub(first.NextRefresh) != 10*time.Second {
			t.Fatal("manual completion did not rearm the automatic deadline")
		}
		time.Sleep(59 * time.Second)
		synctest.Wait()
		if calls.Load() != 2 {
			t.Fatal("automatic refresh fired early")
		}
		time.Sleep(time.Second)
		r.Refresh()
		synctest.Wait()
		if calls.Load() != 3 || !r.View().Refreshing {
			t.Fatal("simultaneous automatic/manual refresh should make one request")
		}
		r.Close()
		synctest.Wait()
		if calls.Load() != 3 {
			t.Fatal("close started another request")
		}
	})
}

func TestRuntimeViewUsesAllSelectedProvidersAndFloatingWindows(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := &Runtime{}
		r.start(context.Background(), Config{Providers: store.Providers(), IntervalSeconds: 60, Language: l10n.English},
			func(_ context.Context, p store.Provider) providerReading {
				if p == store.ProviderDeepSeek {
					return providerReading{meters: []runway.Meter{{Title: "Balance", Balance: true, Source: runway.SourceStandard, ValueText: "$3.50"}}}
				}
				if p == store.ProviderCopilot {
					return providerReading{status: "not configured"}
				}
				var meters []runway.Meter
				for _, minutes := range []int{runway.MonthlyMinutes, runway.WeeklyMinutes, runway.FiveHourMinutes} {
					meters = append(meters, runway.NewMeter("quota", runway.MeterWindow{UsedPercent: 25, WindowMinutes: minutes, ResetsAt: time.Now().Add(time.Hour)}, time.Now(), nil, runway.SourceStandard))
				}
				return providerReading{meters: meters}
			})
		defer r.Close()
		synctest.Wait()
		view := r.View()
		if len(view.Floating.Providers) != 6 {
			t.Fatalf("GUI floating provider limit leaked into TUI: %d", len(view.Floating.Providers))
		}
		for _, p := range view.Floating.Providers {
			switch p.Provider {
			case store.ProviderDeepSeek:
				if len(p.Meters) != 1 || p.Meters[0].ValueText != "$3.50" {
					t.Fatalf("lost balance: %+v", p)
				}
			case store.ProviderCopilot:
				if len(p.Meters) != 0 || p.Status != "not configured" {
					t.Fatalf("lost unavailable reason: %+v", p)
				}
			default:
				if len(p.Meters) != 2 || p.Meters[0].RemainingPercent != 75 || p.Meters[0].Reset == "" {
					t.Fatalf("incorrect floating quota: %+v", p)
				}
			}
		}
	})
}
