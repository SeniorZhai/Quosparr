package tui

import (
	"context"
	"math"
	"os"
	"time"

	"github.com/licoy/quosparr/internal/claude"
	"github.com/licoy/quosparr/internal/grok"
	"github.com/licoy/quosparr/internal/platform"
	"github.com/licoy/quosparr/internal/store"
	"github.com/licoy/quosparr/internal/usage"
)

// TokenHistory is local usage grouped by UTC date, independent of account quotas.
// Days contains only recorded dates, including explicitly recorded zero usage.
type TokenHistory struct {
	Days                                   map[string]int64
	From, Through, UpdatedAt               time.Time
	Loading, Available, Incomplete, Failed bool
}

type historyReader struct {
	codex  usage.Scanner
	claude claude.SessionScanner
	grok   grok.SessionScanner
}

func newHistoryReader(home string) *historyReader {
	return &historyReader{
		codex: usage.Scanner{CodexHome: platform.CodexDir(home), Location: time.UTC, Cache: &usage.CacheScanCache{}},
		claude: claude.SessionScanner{ConfigDir: claude.ResolveConfigDir(os.Getenv("CLAUDE_CONFIG_DIR"), home), Home: home,
			Location: time.UTC, Cache: &claude.SessionCache{}},
		grok: grok.SessionScanner{GrokHome: grok.ResolveOfficialHome(os.Getenv("GROK_HOME"), home), Location: time.UTC, Cache: &grok.CacheScanCache{}},
	}
}

// Read is intended for a background refresh, not for the per-second render loop.
func (r *historyReader) Read(ctx context.Context, provider store.Provider, now time.Time) TokenHistory {
	if provider != store.ProviderCodex && provider != store.ProviderClaude && provider != store.ProviderGrok {
		return TokenHistory{}
	}
	now = now.UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	out := TokenHistory{From: today.AddDate(0, 0, -370), Through: today, UpdatedAt: now}
	window := usage.Window{Start: out.From, End: now}
	var days map[string]int64
	switch provider {
	case store.ProviderCodex:
		report, err := r.codex.ScanDaily(ctx, window)
		if err != nil {
			out.Failed = true
			return out
		}
		days, out.Incomplete = report.Days, report.UnreadLines > 0
	case store.ProviderClaude:
		report, err := r.claude.Scan(ctx, claude.SessionScanOptions{SessionLimit: math.MaxInt, CostWindow: window, Now: now})
		if err != nil {
			out.Failed = true
			return out
		}
		days, out.Incomplete = report.DailyTokens, report.BadLines > 0
	case store.ProviderGrok:
		report, err := r.grok.Scan(ctx, grok.SessionScanOptions{SessionLimit: math.MaxInt, CostWindow: window, Now: now})
		if err != nil {
			out.Failed = true
			return out
		}
		quality, err := r.grok.ScanCache(ctx, window, now)
		if err != nil {
			out.Failed = true
			return out
		}
		days, out.Incomplete = report.DailyTokens, quality.UnreadLines > 0
	}
	from, through := out.From.Format(time.DateOnly), out.Through.Format(time.DateOnly)
	out.Days = make(map[string]int64, len(days))
	for day, tokens := range days {
		if day < from || day > through {
			continue
		}
		if tokens < 0 {
			out.Days, out.Failed = nil, true
			return out
		}
		out.Days[day] = tokens
	}
	out.Available = len(out.Days) > 0
	return out
}
