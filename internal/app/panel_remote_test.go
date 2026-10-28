package app

import (
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/remotequota"
	"github.com/licoy/quosparr/internal/runway"
	"github.com/licoy/quosparr/internal/store"
	"testing"
)

func TestConfiguredRemotePanelHidesKeyForm(t *testing.T) {
	for _, p := range []store.Provider{store.ProviderKimi, store.ProviderDeepSeek, store.ProviderCopilot} {
		for _, configured := range []bool{true, false} {
			snap := panelFixture()
			snap.Remote = map[store.Provider]runway.RemoteQuota{p: {Configured: configured, Code: remotequota.CodeUnauthorized}}
			_, tt := newPanel(t, func(prefs *store.Preferences) { prefs.SelectedProvider = p }, snap, PanelActions{})
			l := l10n.New(l10n.English)
			label := xRemoteAPIKey
			if p == store.ProviderCopilot {
				label = xRemoteToken
			}
			if tt.HasText(tx(l, label)) == configured || tt.HasText(tx(l, xRemoteSave)) == configured {
				t.Fatalf("%s configured=%v: wrong credential form visibility", p, configured)
			}
		}
	}
}
