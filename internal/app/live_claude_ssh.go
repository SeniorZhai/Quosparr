package app

import (
	"context"
	"errors"
	"time"

	"github.com/licoy/quosparr/internal/claude"
	"github.com/licoy/quosparr/internal/runway"
)

func (g *claudeBridge) refreshSSH(ctx context.Context, host string, force bool) error {
	g.mu.Lock()
	if !force && time.Since(g.sshAttempt) < time.Duration(max(60, g.prefs.RefreshIntervalSeconds))*time.Second {
		g.mu.Unlock()
		return nil
	}
	fetch := g.sshFetch
	g.sshAttempt = time.Now()
	g.mu.Unlock()
	if fetch == nil {
		fetch = claude.FetchUsageSSH
	}
	snapshot, err := fetch(ctx, host)
	g.mu.Lock()
	if g.prefs.ClaudeSSHHost == host {
		g.sshError = err
		g.sshSnapshot = nil
		if err == nil {
			g.sshSnapshot = &snapshot
		}
	}
	g.mu.Unlock()
	g.reload(true)
	return err
}

func (g *claudeBridge) remoteStateLocked(local runway.ClaudeState) runway.ClaudeState {
	st := runway.ClaudeState{
		RemoteUsage: true, Availability: runway.ClaudeLoading,
		Accounts: local.Accounts, AccountQuotas: local.AccountQuotas,
		Refreshing: local.Refreshing, Widget: local.Widget,
		Cost: local.Cost, Heatmap: local.Heatmap, RecentSessions: local.RecentSessions,
		Speed: local.Speed, SpeedError: local.SpeedError,
	}
	if g.sshError != nil {
		st.Availability = runway.ClaudeFailed
		st.LastError = tx(g.lc, xClaudeSSHFailed)
		switch {
		case errors.Is(g.sshError, claude.ErrUnauthorized):
			st.Availability = runway.ClaudeReauthRequired
		case errors.Is(g.sshError, claude.ErrUsageParse):
			st.Availability = runway.ClaudeUsageParseFailed
		default:
			var rate *claude.RateLimitedError
			if errors.As(g.sshError, &rate) {
				st.Availability = runway.ClaudeRateLimited
			}
		}
		return st
	}
	if snap := g.sshSnapshot; snap != nil {
		st.Availability = runway.ClaudeReady
		st.Meters = claudeMeters(g.lc, *snap, snap.FetchedAt)
		st.UpdatedAt = snap.FetchedAt
		st.Detail = snap
	}
	return st
}
