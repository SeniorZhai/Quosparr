package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/licoy/quosparr/internal/apiquota"
	"github.com/licoy/quosparr/internal/claude"
	"github.com/licoy/quosparr/internal/codex"
	"github.com/licoy/quosparr/internal/codexapi"
	"github.com/licoy/quosparr/internal/grok"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/netx"
	"github.com/licoy/quosparr/internal/platform"
	"github.com/licoy/quosparr/internal/remotequota"
	"github.com/licoy/quosparr/internal/runway"
	"github.com/licoy/quosparr/internal/store"
)

func readProvider(ctx context.Context, home string, p store.Provider, account Account, lc l10n.Localizer, now time.Time) providerReading {
	failed := providerReading{status: lc.Text(l10n.KeyStatusError)}
	unconfigured := providerReading{status: lc.TUI("unconfigured")}
	command := map[store.Provider]string{store.ProviderCodex: "codex login", store.ProviderClaude: "claude", store.ProviderGrok: "grok login --oauth", store.ProviderCopilot: "gh auth login"}[p]
	login := providerReading{status: fmt.Sprintf(lc.TUI("loginCLI"), command)}
	expiredKey := "expiredCLI"
	if account.Mode != "cli" {
		expiredKey = "expiredImported"
	}
	expired := providerReading{status: fmt.Sprintf(lc.TUI(expiredKey), command)}
	var data []byte
	var secret struct {
		APIKey string `json:"apiKey"`
	}
	if account.Mode != "cli" {
		if account.CredentialID == "" {
			return unconfigured
		}
		var err error
		data, err = ReadCredential(home, account)
		if err != nil || json.Unmarshal(data, &secret) != nil {
			return failed
		}
		if secret.APIKey != "" && !p.RemoteQuota() {
			return readAPIQuota(ctx, account.BaseURL, secret.APIKey, p, lc, now)
		}
	}
	switch p {
	case store.ProviderCodex:
		var auth codex.Auth
		var err error
		if account.Mode == "cli" {
			auth, err = codex.NewStore(platform.CodexDir(home), platform.RunwayDir(home)).LoadOfficialAuth()
		} else {
			auth, err = codex.DecodeAuth(data)
		}
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return login
			}
			return failed
		}
		if auth.IsAPIKey() {
			base := account.BaseURL
			if account.Mode == "cli" {
				var config struct {
					ModelProvider  string `toml:"model_provider"`
					ModelProviders map[string]struct {
						BaseURL string `toml:"base_url"`
					} `toml:"model_providers"`
				}
				_, err := toml.DecodeFile(filepath.Join(platform.CodexDir(home), "config.toml"), &config)
				if err != nil && !errors.Is(err, os.ErrNotExist) {
					return failed
				}
				base = config.ModelProviders[config.ModelProvider].BaseURL
			}
			return readAPIQuota(ctx, base, auth.APIKey(), p, lc, now)
		}
		if codex.IsTokenExpired(auth.Tokens.AccessToken, now) {
			return expired
		}
		if auth.LoginUsability(now) != codex.Usable {
			return login
		}
		quota, err := (codexapi.QuotaClient{}).FetchQuota(ctx, codexapi.Auth{
			IDToken: auth.Tokens.IDToken, AccessToken: auth.Tokens.AccessToken, AccountID: auth.Tokens.AccountID,
		})
		if errors.Is(err, codexapi.ErrAuthenticationRequired) {
			return expired
		}
		if err != nil {
			return failed
		}
		return providerReading{meters: runway.QuotaMeters(lc, quota, false), status: lc.TUI("unavailable")}
	case store.ProviderClaude:
		if account.Mode == "cli" {
			env := os.Getenv("CLAUDE_CONFIG_DIR")
			dir := claude.ResolveConfigDir(env, home)
			st := claude.NewStore(dir, platform.RunwayDir(home), home, env, "")
			base, key, err := st.ReadAPISettingsFile()
			if err != nil {
				return failed
			}
			if base != "" && key != "" {
				base, err = claude.NormalizeAPIBaseURL(base)
				if err != nil {
					return failed
				}
				return readAPIQuota(ctx, base, key, p, lc, now)
			}
			data, err = claude.OpenOfficialStore(dir, home, "", env).Load(ctx)
			if errors.Is(err, claude.ErrNotLoggedIn) {
				return login
			}
			if err != nil {
				return failed
			}
		}
		doc, err := claude.ParseAuthDocument(data)
		if err != nil {
			return failed
		}
		if doc.AccessExpired(now) {
			return expired
		}
		if doc.OAuth().AccessToken == "" {
			return login
		}
		raw, _, err := (&claude.UsageClient{}).Fetch(ctx, doc.OAuth().AccessToken)
		if errors.Is(err, claude.ErrUnauthorized) {
			return expired
		}
		if err != nil {
			return failed
		}
		usage, err := claude.ParseUsageSnapshot(raw)
		if err != nil {
			return failed
		}
		reading := providerReading{status: lc.TUI("unavailable")}
		for _, window := range usage.Windows {
			if window.Null || window.UsedPercent == nil {
				continue
			}
			minutes, source := 0, runway.SourceStandard
			switch window.Kind {
			case claude.WindowFiveHour:
				minutes = runway.FiveHourMinutes
			case claude.WindowSevenDay:
				minutes = runway.WeeklyMinutes
			case claude.WindowSevenDayModel:
				minutes, source = runway.WeeklyMinutes, runway.SourceModelSpecific
			default:
				continue
			}
			w := runway.MeterWindow{UsedPercent: int(math.Round(*window.UsedPercent)), WindowMinutes: minutes}
			if window.ResetsAt != nil {
				w.ResetsAt = *window.ResetsAt
			}
			reading.meters = append(reading.meters, runway.NewMeter(runway.WindowTitle(lc, minutes), w, now, runway.DefaultMarkers(), source))
		}
		return reading
	case store.ProviderGrok:
		if account.Mode == "cli" {
			dir := grok.ResolveOfficialHome(os.Getenv("GROK_HOME"), home)
			var err error
			data, err = os.ReadFile(filepath.Join(dir, "auth.json"))
			if errors.Is(err, os.ErrNotExist) {
				return login
			}
			if err != nil {
				return failed
			}
		}
		doc, err := grok.ParseAuthDocument(data)
		if err != nil {
			return failed
		}
		accessExpired := doc.Identity.ExpiresAt != nil && !doc.Identity.ExpiresAt.After(now) || (grok.TokenRefresher{Skew: -1}).NeedsRefresh(doc, now)
		if accessExpired && account.Mode == "imported" {
			dir := grok.ResolveOfficialHome(os.Getenv("GROK_HOME"), home)
			if official, err := os.ReadFile(filepath.Join(dir, "auth.json")); err == nil {
				if current, err := grok.ParseAuthDocument(official); err == nil && current.Identity.HasAccessToken && current.StableID() == doc.StableID() {
					doc = current
					accessExpired = doc.Identity.ExpiresAt != nil && !doc.Identity.ExpiresAt.After(now) || (grok.TokenRefresher{Skew: -1}).NeedsRefresh(doc, now)
				}
			}
		}
		if accessExpired {
			return expired
		}
		token, err := doc.AccessToken()
		if err != nil || token == "" {
			return login
		}
		raw, _, err := (&grok.BillingClient{Version: func() string { return "" }}).FetchToken(ctx, token)
		if errors.Is(err, grok.ErrAuthenticationRequired) {
			return expired
		}
		if err != nil {
			return failed
		}
		quota, err := grok.ParseBillingSnapshot(raw)
		if err != nil {
			return failed
		}
		w := runway.MeterWindow{UsedPercent: int(math.Round(quota.IncludedUsagePercent))}
		if quota.Period != nil {
			switch quota.Period.Kind {
			case grok.BillingWeekly:
				w.WindowMinutes = runway.WeeklyMinutes
			case grok.BillingMonthly:
				w.WindowMinutes = runway.MonthlyMinutes
			}
			if quota.Period.ResetsAt != nil {
				w.ResetsAt = *quota.Period.ResetsAt
			}
		}
		return providerReading{meters: []runway.Meter{runway.NewMeter(lc.Text(l10n.KeyGrokIncludedQuota), w, now, runway.DefaultMarkers(), runway.SourceStandard)}}
	default:
		var reading remotequota.Reading
		var err error
		switch p {
		case store.ProviderKimi:
			reading, err = remotequota.FetchKimi(ctx, secret.APIKey, account.Region)
		case store.ProviderDeepSeek:
			reading, err = remotequota.FetchDeepSeek(ctx, secret.APIKey)
		case store.ProviderCopilot:
			reading, err = remotequota.FetchCopilot(ctx, secret.APIKey)
		default:
			return unconfigured
		}
		switch reading.Code {
		case remotequota.CodeUnconfigured:
			return unconfigured
		case remotequota.CodeUnauthorized:
			return providerReading{status: lc.Text(l10n.KeyStatusLogin)}
		case remotequota.CodeUnavailable:
			return providerReading{status: lc.TUI("unavailable")}
		}
		if err != nil || reading.Code != "" {
			return failed
		}
		result := providerReading{status: lc.TUI("unavailable")}
		for _, pool := range reading.Pools {
			title := runway.WindowTitle(lc, pool.WindowMinutes)
			switch pool.ID {
			case remotequota.PoolPremium:
				title = lc.Text(l10n.KeyRemotePremium)
			case remotequota.PoolChat:
				title = lc.Text(l10n.KeyRemoteChat)
			case remotequota.PoolBalance:
				title = lc.Text(l10n.KeyRemoteBalance)
			case remotequota.PoolCredits:
				title = lc.Text(l10n.KeyRemoteCredits)
			}
			meter := runway.NewMeter(title, runway.MeterWindow{UsedPercent: pool.UsedPercent, WindowMinutes: pool.WindowMinutes, ResetsAt: pool.ResetsAt}, now, runway.DefaultMarkers(), runway.SourceStandard)
			if pool.Balance {
				meter.Balance, meter.ValueText = true, pool.ValueText
				meter.ResetsAt, meter.ResetText, meter.Projection = time.Time{}, "", nil
			}
			result.meters = append(result.meters, meter)
		}
		return result
	}
}

func readAPIQuota(ctx context.Context, base, key string, provider store.Provider, lc l10n.Localizer, now time.Time) providerReading {
	if base == "" {
		return providerReading{status: lc.TUI("unavailable")}
	}
	client, err := netx.ClientFor(ctx, netx.WithoutCookies)
	if err != nil {
		return providerReading{status: lc.Text(l10n.KeyStatusError)}
	}
	quota := apiquota.ProbeForProvider(ctx, client, base, key, string(provider), now)
	if meters := runway.APIQuotaMeters(lc, quota, now); len(meters) > 0 {
		return providerReading{meters: meters}
	}
	switch quota.Status {
	case apiquota.StatusUnauthorized:
		return providerReading{status: lc.Text(l10n.KeyStatusLogin)}
	case apiquota.StatusUnsupported:
		return providerReading{status: lc.TUI("balanceUnsupported")}
	default:
		return providerReading{status: lc.Text(l10n.KeyStatusError)}
	}
}
