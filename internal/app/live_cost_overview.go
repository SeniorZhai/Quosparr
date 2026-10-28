package app

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/licoy/quosparr/internal/claude"
	"github.com/licoy/quosparr/internal/grok"
	"github.com/licoy/quosparr/internal/pricing"
	"github.com/licoy/quosparr/internal/usage"
)

// queryCostOverview reads each provider once, independently of login and panel ranges.
func (rt *liveApp) queryCostOverview(ctx context.Context) (usage.CostOverview, error) {
	if err := ctx.Err(); err != nil {
		return usage.CostOverview{}, err
	}
	now := time.Now()
	window := usage.Window{Start: usage.Today(now, time.Local).Window.Start.AddDate(0, 0, -29), End: now}
	snapshot := pricing.Snapshot{}
	if rt.pricing != nil {
		snapshot = rt.pricing.manager.Snapshot()
	}
	prices := func() pricing.Snapshot { return snapshot }
	var summaries [3]usage.Summary
	var failures [3]error
	var scans sync.WaitGroup
	for i := range summaries {
		scans.Add(1)
		go func() {
			defer scans.Done()
			switch i {
			case 0:
				if rt.repo != nil {
					results, err := rt.repo.Summaries(ctx, []usage.Query{{ID: "overview", Window: window, DayBoundary: usage.DayLocal}}, now, usage.RefreshIfChanged, nil)
					summaries[i], failures[i] = results["overview"], err
				} else if home := rt.codexFolder(); home != "" {
					scanner := usage.Scanner{CodexHome: home, Location: time.Local, Prices: usage.SnapshotPrices(prices).PriceBook(ctx)}
					summary, diagnostics, err := scanner.ScanAPIEquivalentReport(ctx, window, now)
					if diagnostics.MalformedCandidateLines > 0 {
						summary.Warnings = append(summary.Warnings, "malformed-jsonl-lines")
					}
					summaries[i], failures[i] = summary, err
				} else {
					failures[i] = errDetailUnavailable
				}
			case 1:
				if rt.claudeMod == nil || rt.claudeMod.Store == nil {
					failures[i] = errDetailUnavailable
					return
				}
				store := rt.claudeMod.Store
				scanner := claude.SessionScanner{ConfigDir: store.ConfigDir(), Home: store.HomeDir(), Location: time.Local, PricingSnapshot: prices}
				if rt.claudeB != nil {
					scanner.Cache = &rt.claudeB.sessions
				}
				result, err := scanner.Scan(ctx, claude.SessionScanOptions{SessionLimit: math.MaxInt, CostWindow: window, Now: now})
				summaries[i], failures[i] = result.CostSummary, err
			case 2:
				if rt.grokMod == nil || rt.grokMod.Store == nil {
					failures[i] = errDetailUnavailable
					return
				}
				scanner := grok.SessionScanner{GrokHome: rt.grokMod.Store.OfficialHome(), Location: time.Local, PricingSnapshot: prices}
				result, err := scanner.Scan(ctx, grok.SessionScanOptions{SessionLimit: math.MaxInt, CostWindow: window, Now: now})
				summaries[i], failures[i] = result.CostSummary, err
			}
		}()
	}
	scans.Wait()
	if err := ctx.Err(); err != nil {
		return usage.CostOverview{}, err
	}
	if rt.pricing != nil && snapshot.Version() != rt.pricing.manager.Snapshot().Version() {
		return usage.CostOverview{}, context.Canceled
	}
	failed := 0
	for i, err := range failures {
		if err != nil {
			failed++
			summaries[i] = usage.Summary{}
		}
	}
	if failed == len(failures) {
		return usage.CostOverview{}, errDetailUnavailable
	}
	return usage.BuildCostOverview(now, time.Local, summaries[:], failed > 0), nil
}
