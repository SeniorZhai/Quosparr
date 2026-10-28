package tui

import (
	"context"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/store"
)

func TestHistoryDoesNotBlockQuotaRefreshAndStopsOnClose(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := &Runtime{}
		var quotaCalls, historyCalls atomic.Int32
		release := make(chan struct{})
		r.start(context.Background(), Config{Providers: []store.Provider{store.ProviderCodex, store.ProviderDeepSeek}, IntervalSeconds: 30, Language: l10n.English}, func(context.Context, store.Provider) providerReading { quotaCalls.Add(1); return providerReading{} })
		r.startHistory(context.Background(), func(ctx context.Context, p store.Provider, now time.Time) TokenHistory {
			historyCalls.Add(1)
			if p != store.ProviderCodex {
				t.Error("remote provider should not scan local logs")
			}
			select {
			case <-release:
			case <-ctx.Done():
			}
			return TokenHistory{Available: true, Days: map[string]int64{now.UTC().Format("2006-01-02"): 42}, UpdatedAt: now}
		})
		synctest.Wait()
		time.Sleep(30 * time.Second)
		synctest.Wait()
		if quotaCalls.Load() != 4 || historyCalls.Load() != 1 {
			t.Fatal("history scan blocked quota or duplicated work")
		}
		release <- struct{}{}
		synctest.Wait()
		if !r.View().History[store.ProviderCodex].Available {
			t.Fatal("history result was not published")
		}
		r.Refresh()
		synctest.Wait()
		if historyCalls.Load() != 2 {
			t.Fatal("manual refresh did not reload history")
		}
		r.Close()
		synctest.Wait()
	})
}
