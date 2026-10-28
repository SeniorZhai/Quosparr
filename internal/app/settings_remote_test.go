package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/remotequota"
	"github.com/licoy/quosparr/internal/runway"
	"github.com/licoy/quosparr/internal/store"
)

func TestRemoteQuotaSettingsSaveAndClear(t *testing.T) {
	u := cardSettingsUI(t, store.LanguageEnglish)
	var calls []store.Provider
	var secrets, regions []string
	u.opts.SaveRemoteKey = func(p store.Provider, secret, region string) error {
		calls = append(calls, p)
		secrets = append(secrets, secret)
		regions = append(regions, region)
		return nil
	}
	u.kimiKey = "kimi-secret"
	u.kimiIntl = true
	tt := ui.NewTester(func(c *ui.Context) { applyTheme(c); u.remoteQuotaGroup(c) }, 700, 900)
	if err := tt.Click(tx(u.L(), xRemoteSave)); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0] != store.ProviderKimi || secrets[0] != "kimi-secret" || regions[0] != remotequota.RegionIntl || u.kimiKey != "" {
		t.Fatalf("save calls = %v regions = %v", calls, regions)
	}
	if err := tt.Click(tx(u.L(), xRemoteClear)); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 4 || calls[1] != store.ProviderKimi || secrets[1] != "" || regions[1] != remotequota.RegionIntl || calls[3] != store.ProviderCopilot {
		t.Fatalf("clear calls = %v regions = %v", calls, regions)
	}
}

func TestRemoteConfiguredKeyRequiresExplicitEdit(t *testing.T) {
	u := cardSettingsUI(t, store.LanguageEnglish)
	u.opts.RemoteQuota = func(p store.Provider) runway.RemoteQuota {
		return runway.RemoteQuota{Configured: p == store.ProviderDeepSeek}
	}
	u.opts.SaveRemoteKey = func(store.Provider, string, string) error { return nil }
	tt := ui.NewTester(func(c *ui.Context) {
		applyTheme(c)
		settingsGroup(c, "Keys", "", nil, func(row rowFunc) {
			u.remoteKeyRow(c, row, store.ProviderDeepSeek, &u.deepseekKey, false)
		})
	}, 700, 500)
	if !tt.HasText(tx(u.L(), xRemoteConfigured)) || tt.HasText(tx(u.L(), xRemoteAPIKey)) {
		t.Fatal("saved key still prompts for input")
	}
	if err := tt.Click(tx(u.L(), xRemoteReplace)); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText(tx(u.L(), xRemoteAPIKey)) {
		t.Fatal("replacement input missing")
	}
	u.deepseekKey = "replacement-fixture"
	u.saveRemoteDrafts()
	tt.Frame()
	if tt.HasText(tx(u.L(), xRemoteAPIKey)) {
		t.Fatal("input remained open after saving")
	}
}
