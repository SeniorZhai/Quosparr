package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/claude"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/runway"
	"github.com/licoy/quosparr/internal/store"
)

func TestClaudeSSHRefreshAndSourceSwitch(t *testing.T) {
	p := store.DefaultPreferences()
	p.ClaudeSSHHost = "macmini"
	g := newClaudeBridge(nil, p, nil)
	calls := 0
	g.sshFetch = func(context.Context, string) (claude.UsageSnapshot, error) {
		calls++
		snap, err := claude.ParseUsageSnapshot([]byte(`{"five_hour":{"utilization":25}}`))
		snap.FetchedAt = time.Now()
		return snap, err
	}
	if err := g.Refresh(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	st := g.State(time.Now())
	if !st.RemoteUsage || st.Availability != runway.ClaudeReady || len(st.Meters) != 1 || st.Meters[0].RemainingPercent != 75 || st.API != nil || st.Plan != "" {
		t.Fatalf("incorrect remote state: %+v", st)
	}
	if st.Widget == nil || len(st.Widget.Quota) != 0 {
		t.Fatal("remote quota reached local widget")
	}
	if err := g.Refresh(context.Background(), false); err != nil || calls != 1 {
		t.Fatal("automatic refresh was not throttled")
	}
	if err := g.Refresh(context.Background(), true); err != nil || calls != 2 {
		t.Fatal("manual refresh did not fetch")
	}
	view := runway.BuildStatusBar(l10n.New(l10n.English), runway.StatusInputs{Prefs: p, Provider: store.ProviderClaude, Claude: st})
	if !strings.Contains(view.Tooltip, "SSH") || len(view.Components) != 1 || view.Components[0].Cells[0].Remaining != 75 {
		t.Fatal("remote quota did not reach status bar")
	}
	g.sshFetch = func(context.Context, string) (claude.UsageSnapshot, error) {
		return claude.UsageSnapshot{}, claude.ErrUnauthorized
	}
	if err := g.Refresh(context.Background(), true); !errors.Is(err, claude.ErrUnauthorized) {
		t.Fatal(err)
	}
	st = g.State(time.Now())
	if len(st.Meters) != 0 || st.LastError == "" || st.Availability != runway.ClaudeReauthRequired {
		t.Fatal("failed SSH query retained a quota")
	}
	p.ClaudeSSHHost = ""
	g.setPreferences(p, nil)
	if st = g.State(time.Now()); st.RemoteUsage || len(st.Meters) != 0 {
		t.Fatal("remote state survived local switch")
	}
}

func TestClaudeSSHDiscardChangedSource(t *testing.T) {
	p := store.DefaultPreferences()
	p.ClaudeSSHHost = "first"
	g := newClaudeBridge(nil, p, nil)
	g.sshFetch = func(context.Context, string) (claude.UsageSnapshot, error) {
		p.ClaudeSSHHost = "second"
		g.setPreferences(p, nil)
		return claude.ParseUsageSnapshot([]byte(`{"five_hour":{"utilization":25}}`))
	}
	if err := g.Refresh(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if st := g.State(time.Now()); len(st.Meters) != 0 || st.Availability != runway.ClaudeLoading {
		t.Fatal("previous host result reached new source")
	}
}

func TestClaudeSSHSettingsSaveAndClear(t *testing.T) {
	u := cardSettingsUI(t, store.LanguageEnglish)
	refreshes := 0
	u.opts.Actions.Refresh = func() { refreshes++ }
	tt := ui.NewTester(func(c *ui.Context) { applyTheme(c); u.claudeSSHGroup(c) }, 700, 600)
	for i, host := range []string{"macmini", ""} {
		u.claudeSSHHost = host
		tt.Frame()
		if err := tt.Click(tx(u.L(), xClaudeSSHSave)); err != nil {
			t.Fatal(err)
		}
		if u.s.Prefs().ClaudeSSHHost != host || refreshes != i+1 {
			t.Fatal("save did not persist source and request refresh")
		}
		encoded, err := store.EncodePreferences(u.s.Prefs())
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := store.DecodePreferences(encoded)
		if err != nil || decoded.ClaudeSSHHost != host {
			t.Fatal("SSH preference did not round trip")
		}
	}
	u.claudeSSHHost = "-oProxyCommand=bad"
	tt.Frame()
	_ = tt.Click(tx(u.L(), xClaudeSSHSave))
	if refreshes != 2 || u.s.Prefs().ClaudeSSHHost != "" {
		t.Fatal("invalid destination saved")
	}
}

func TestClaudeSSHPanelSeparatesLocalIdentityAndModules(t *testing.T) {
	snap := panelFixture()
	snap.Claude = runway.ClaudeState{RemoteUsage: true, Availability: runway.ClaudeReady,
		Meters: []runway.Meter{{Title: "Remote quota", RemainingPercent: 75}},
		Speed:  speedFixture(),
	}
	snap.Claude.Accounts.Display.DisplayName = "local-account-must-not-appear"
	_, tt := newPanel(t, func(p *store.Preferences) { p.SelectedProvider = store.ProviderClaude }, snap, PanelActions{})
	l := l10n.New(l10n.English)
	if !tt.HasText(tx(l, xClaudeSSHSource)) || !tt.HasText("Remote quota") {
		t.Fatal("remote source or quota is missing")
	}
	if tt.HasText("local-account-must-not-appear") || !tt.HasText(tx(l, xSpeedTitle)) {
		t.Fatal("remote panel mixed local identity or lost local speed")
	}
}

func TestClaudeSSHRetainsLocalStatisticsOnFailure(t *testing.T) {
	g := &claudeBridge{lc: l10n.New(l10n.English)}
	local := runway.ClaudeState{Speed: speedFixture(), SpeedError: "local scan warning",
		Plan: "API", Meters: []runway.Meter{{Title: "Local balance"}},
		API: &runway.ClaudeAPIView{},
	}
	for _, failed := range []bool{false, true} {
		if failed {
			g.sshError = claude.ErrUnauthorized
		}
		got := g.remoteStateLocked(local)
		if got.Speed != local.Speed || got.SpeedError != local.SpeedError {
			t.Fatal("local statistics lost")
		}
		if got.API != nil || got.Plan != "" || len(got.Meters) != 0 {
			t.Fatal("local balance leaked into SSH quota")
		}
	}
}
