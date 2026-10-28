package runway

import (
	"context"
	"time"

	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/remotequota"
	"github.com/licoy/quosparr/internal/store"
)

// RemoteQuota is the panel and status-bar view of one quota-only provider.
// It never contains a key.
type RemoteQuota struct {
	Code       string
	Plan       string
	Meters     []Meter
	Lines      []DetailLine
	Refreshing bool
	Configured bool
}

type remoteHeld struct {
	Reading    remotequota.Reading
	Refreshing bool
	Configured bool
}

// SaveRemoteKey replaces one provider's saved secret and refreshes it.
// An empty secret clears that secret. region is only used for Kimi.
func (s *Service) SaveRemoteKey(p store.Provider, secret, region string) error {
	if !p.RemoteQuota() {
		return nil
	}
	dir := s.opts.RemoteQuotaDir
	keys, err := remotequota.Load(dir)
	if err != nil {
		return err
	}
	switch p {
	case store.ProviderKimi:
		keys.KimiKey = secret
		keys.KimiRegion = region
	case store.ProviderDeepSeek:
		keys.DeepSeekKey = secret
	case store.ProviderCopilot:
		keys.CopilotToken = secret
	}
	if err := remotequota.Save(dir, keys); err != nil {
		return err
	}
	s.refreshRemote(p)
	return nil
}

func (s *Service) refreshWantedRemote() {
	s.mu.Lock()
	var want []store.Provider
	for _, p := range store.Providers() {
		if p.RemoteQuota() && s.quotaWantedLocked(p) {
			want = append(want, p)
		}
	}
	s.mu.Unlock()
	for _, p := range want {
		s.refreshRemote(p)
	}
}

func (s *Service) refreshRemote(p store.Provider) {
	if s.opts.RemoteQuotaDir == "" || !p.RemoteQuota() {
		return
	}
	s.spawn(func() {
		s.mu.Lock()
		held := s.remote[p]
		held.Refreshing = true
		s.remote[p] = held
		s.publishLocked()
		dir := s.opts.RemoteQuotaDir
		s.mu.Unlock()

		ctx, cancel := context.WithTimeout(s.root, 30*time.Second)
		defer cancel()
		keys, err := remotequota.Load(dir)
		reading := remotequota.Reading{Code: remotequota.CodeFailed}
		configured := false
		if err != nil {
			reading.Code = remotequota.CodeFailed
		} else {
			reading, configured = fetchRemote(ctx, p, keys)
		}

		s.mu.Lock()
		s.remote[p] = remoteHeld{Reading: reading, Configured: configured}
		s.publishLocked()
		s.mu.Unlock()
	})
}

func fetchRemote(ctx context.Context, p store.Provider, keys remotequota.Keys) (remotequota.Reading, bool) {
	switch p {
	case store.ProviderKimi:
		if keys.KimiKey == "" {
			return remotequota.Reading{Code: remotequota.CodeUnconfigured}, false
		}
		reading, _ := remotequota.FetchKimi(ctx, keys.KimiKey, keys.KimiRegion)
		return reading, true
	case store.ProviderDeepSeek:
		if keys.DeepSeekKey == "" {
			return remotequota.Reading{Code: remotequota.CodeUnconfigured}, false
		}
		reading, _ := remotequota.FetchDeepSeek(ctx, keys.DeepSeekKey)
		return reading, true
	case store.ProviderCopilot:
		reading, _ := remotequota.FetchCopilot(ctx, keys.CopilotToken)
		return reading, keys.CopilotToken != "" || reading.Code != remotequota.CodeUnconfigured
	default:
		return remotequota.Reading{Code: remotequota.CodeFailed}, false
	}
}

func (s *Service) remoteQuotasLocked(now time.Time) map[store.Provider]RemoteQuota {
	if len(s.remote) == 0 {
		return nil
	}
	out := make(map[store.Provider]RemoteQuota, len(s.remote))
	for p, held := range s.remote {
		out[p] = remoteQuotaOf(s.lc, held, now)
	}
	return out
}

func remoteQuotaOf(lc l10n.Localizer, held remoteHeld, now time.Time) RemoteQuota {
	q := RemoteQuota{
		Code: held.Reading.Code, Plan: held.Reading.Plan,
		Refreshing: held.Refreshing, Configured: held.Configured,
	}
	for _, pool := range held.Reading.Pools {
		title := remotePoolTitle(lc, pool.ID)
		if pool.Balance {
			m := NewMeter(title, MeterWindow{UsedPercent: pool.UsedPercent}, now, nil, SourceStandard)
			m.Balance = true
			m.ValueText = pool.ValueText
			m.ResetsAt = time.Time{}
			m.ResetText = ""
			m.Projection = nil
			q.Meters = append(q.Meters, m)
			continue
		}
		m := NewMeter(title, MeterWindow{
			UsedPercent: pool.UsedPercent, WindowMinutes: pool.WindowMinutes, ResetsAt: pool.ResetsAt,
		}, now, DefaultMarkers(), SourceStandard)
		if pool.WindowMinutes <= 0 {
			m.Title = title
		}
		q.Meters = append(q.Meters, m)
	}
	for _, line := range held.Reading.Lines {
		q.Lines = append(q.Lines, DetailLine{Title: line.ID, Value: line.Value})
	}
	return q
}

func remotePoolTitle(lc l10n.Localizer, id string) string {
	switch id {
	case remotequota.PoolSession:
		return WindowTitle(lc, FiveHourMinutes)
	case remotequota.PoolWeek:
		return WindowTitle(lc, WeeklyMinutes)
	case remotequota.PoolMonth:
		return ShortWindowTitle(lc, MonthlyMinutes)
	case remotequota.PoolPremium:
		return lc.Text(l10n.KeyRemotePremium)
	case remotequota.PoolChat:
		return lc.Text(l10n.KeyRemoteChat)
	case remotequota.PoolBalance:
		return lc.Text(l10n.KeyRemoteBalance)
	case remotequota.PoolCredits:
		return lc.Text(l10n.KeyRemoteCredits)
	default:
		return lc.Text(l10n.KeyQuota)
	}
}

func remoteStatusText(lc l10n.Localizer, q RemoteQuota, now time.Time) string {
	if q.Refreshing && len(q.Meters) == 0 {
		return lc.Text(l10n.KeyStatusWait)
	}
	switch q.Code {
	case remotequota.CodeUnconfigured, remotequota.CodeUnauthorized:
		return lc.Text(l10n.KeyStatusLogin)
	case remotequota.CodeFailed, remotequota.CodeUnavailable:
		if len(q.Meters) == 0 {
			return lc.Text(l10n.KeyStatusError)
		}
	}
	if len(q.Meters) == 0 {
		return lc.Text(l10n.KeyStatusUnknown)
	}
	m := q.Meters[0]
	if m.HasReset() {
		return DurationLocalized(m.ResetsAt.Sub(now).Seconds(), lc.Language(), false)
	}
	return percentText(m)
}
