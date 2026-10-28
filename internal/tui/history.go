package tui

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"time"

	"github.com/licoy/quosparr/internal/claude"
	"github.com/licoy/quosparr/internal/codex"
	"github.com/licoy/quosparr/internal/codexapi"
	"github.com/licoy/quosparr/internal/grok"
	"github.com/licoy/quosparr/internal/platform"
	"github.com/licoy/quosparr/internal/store"
	"github.com/licoy/quosparr/internal/usage"
)

// TokenHistory keeps local and official account usage in separate UTC series.
// Days contains only recorded dates, including explicitly recorded zero usage.
type TokenHistory struct {
	Days                                   map[string]int64
	AllDevices                             map[string]int64
	OfficialStatsAsOf                      string
	OfficialFailed                         bool
	From, Through, UpdatedAt               time.Time
	Loading, Available, Incomplete, Failed bool
}

type historyReader struct {
	home    string
	account Account
	codex   usage.Scanner
	claude  claude.SessionScanner
	grok    grok.SessionScanner
}

func newHistoryReader(home string, account Account) *historyReader {
	return &historyReader{
		home: home, account: account,
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
	year := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
	out := TokenHistory{From: year, Through: today, UpdatedAt: now}
	window := usage.Window{Start: out.From, End: now}
	var days map[string]int64
	switch provider {
	case store.ProviderCodex:
		report, err := r.codex.ScanDaily(ctx, window)
		if err != nil {
			out.Failed = true
		} else {
			days, out.Incomplete = report.Days, report.UnreadLines > 0
		}
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
	if provider != store.ProviderCodex || r.account.Mode != "cli" && r.account.Mode != "imported" {
		return out
	}
	var auth codex.Auth
	var err error
	if r.account.Mode == "cli" {
		auth, err = codex.NewStore(platform.CodexDir(r.home), platform.RunwayDir(r.home)).LoadOfficialAuth()
	} else {
		var data []byte
		data, err = ReadCredential(r.home, r.account)
		if err == nil {
			var secret struct {
				APIKey string `json:"apiKey"`
			}
			err = json.Unmarshal(data, &secret)
			if err == nil && secret.APIKey != "" {
				return out
			}
			if err == nil {
				auth, err = codex.DecodeAuth(data)
			}
		}
	}
	if err != nil {
		out.OfficialFailed = true
		return out
	}
	if auth.IsAPIKey() {
		return out
	}
	if codex.IsTokenExpired(auth.Tokens.AccessToken, now) || auth.LoginUsability(now) != codex.Usable {
		out.OfficialFailed = true
		return out
	}
	request, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	profile, err := (codexapi.QuotaClient{}).FetchTokenUsageProfile(request, codexapi.Auth{
		IDToken: auth.Tokens.IDToken, AccessToken: auth.Tokens.AccessToken, AccountID: auth.Tokens.AccountID,
	})
	if err != nil {
		out.OfficialFailed = true
		return out
	}
	out.AllDevices = make(map[string]int64, len(profile.DailyTokens))
	for day, tokens := range profile.DailyTokens {
		if day >= from && day <= through {
			out.AllDevices[day] = int64(tokens)
		}
	}
	out.OfficialStatsAsOf = profile.StatsAsOf
	out.Available = out.Available || len(out.AllDevices) > 0
	return out
}
