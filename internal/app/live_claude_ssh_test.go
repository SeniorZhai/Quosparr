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
	if !strings.Contains(view.Tooltip, "Claude:") || len(view.Components) != 1 || view.Components[0].Cells[0].Remaining != 75 {
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

func TestClaudeSSHLocalHooks(t *testing.T) {
	previousAfter, previousState := claudeAfterSSH, claudeSSHState
	t.Cleanup(func() { claudeAfterSSH, claudeSSHState = previousAfter, previousState })
	claudeAfterSSH, claudeSSHState = nil, nil
	p := store.DefaultPreferences()
	p.ClaudeSSHHost = "remote-test"
	g := newClaudeBridge(nil, p, nil)
	g.sshFetch = func(context.Context, string) (claude.UsageSnapshot, error) {
		return claude.UsageSnapshot{}, claude.ErrUnauthorized
	}
	ctx := context.Background()
	if err := g.Refresh(ctx, true); !errors.Is(err, claude.ErrUnauthorized) {
		t.Fatal("nil hook changed the SSH error")
	}
	if g.State(time.Now()).Availability != runway.ClaudeReauthRequired {
		t.Fatal("nil hook changed the SSH state")
	}
	calls := 0
	claudeAfterSSH = func(got *claudeBridge, gotCtx context.Context, force bool) {
		calls++
		if got != g || gotCtx != ctx || !force {
			t.Fatal("refresh hook received incorrect arguments")
		}
		got.mu.Lock()
		defer got.mu.Unlock()
		if !errors.Is(got.sshError, claude.ErrUnauthorized) {
			t.Fatal("refresh hook ran before SSH completed")
		}
	}
	if err := g.Refresh(ctx, true); !errors.Is(err, claude.ErrUnauthorized) || calls != 1 {
		t.Fatal("refresh hook was skipped or hid the SSH error")
	}
	for _, handled := range []bool{false, true} {
		stateCalls := 0
		claudeSSHState = func(got *claudeBridge, local runway.ClaudeState) (runway.ClaudeState, bool) {
			stateCalls++
			if got != g || local.RemoteUsage {
				t.Fatal("state hook did not receive the local state")
			}
			return runway.ClaudeState{Availability: runway.ClaudeReady, Plan: "test override"}, handled
		}
		st := g.State(time.Now())
		if stateCalls != 1 {
			t.Fatal("state hook was not called once")
		}
		if handled {
			if st.Availability != runway.ClaudeReady || st.Plan != "test override" {
				t.Fatal("handled state override was discarded")
			}
		} else if st.Availability != runway.ClaudeReauthRequired || !st.RemoteUsage || st.Plan != "" {
			t.Fatal("declined state override changed SSH behavior")
		}
	}
	p.ClaudeSSHHost = ""
	g.setPreferences(p, nil)
	claudeSSHState = func(*claudeBridge, runway.ClaudeState) (runway.ClaudeState, bool) {
		t.Fatal("SSH state hook ran with SSH disabled")
		return runway.ClaudeState{}, false
	}
	if err := g.Refresh(ctx, true); err != nil || calls != 1 {
		t.Fatal("SSH refresh hook ran with SSH disabled")
	}
	_ = g.State(time.Now())
}
