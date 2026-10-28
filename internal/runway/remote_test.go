package runway

import (
	"strings"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/store"
)

func TestRemoteStatusDoesNotUseCodex(t *testing.T) {
	now := time.Now()
	codex := testMeter("5小时", 20, FiveHourMinutes, SourceStandard)
	prefs := store.DefaultPreferences()
	prefs.SelectedProvider = store.ProviderKimi
	v := BuildStatusBar(zh, StatusInputs{
		Prefs: prefs, Provider: store.ProviderKimi,
		CodexText: "codex-only", CodexMeters: []Meter{codex}, Now: now,
	})
	if len(v.Components) != 1 || v.Components[0].Provider != store.ProviderKimi {
		t.Fatalf("components = %+v", v.Components)
	}
	if len(v.Components[0].Cells) != 1 || v.Components[0].Cells[0].Percent != "--" {
		t.Fatalf("cells = %+v", v.Components[0].Cells)
	}
	if strings.Contains(v.Tooltip, "80%") || strings.Contains(v.Tooltip, "codex-only") {
		t.Fatalf("tooltip used codex: %q", v.Tooltip)
	}

	kimi := NewMeter("5小时", MeterWindow{UsedPercent: 25, WindowMinutes: FiveHourMinutes, ResetsAt: now.Add(time.Hour)}, now, DefaultMarkers(), SourceStandard)
	balance := NewMeter("", MeterWindow{}, now, nil, SourceStandard)
	balance.Balance = true
	balance.Title = "余额"
	balance.ValueText = "¥12.50"
	premium := NewMeter("", MeterWindow{}, now, nil, SourceStandard)
	premium.Title = "Premium"
	chat := NewMeter("", MeterWindow{}, now, nil, SourceStandard)
	chat.Title = "Chat"
	prefs.SetStatusBarGroups([][]store.Provider{{store.ProviderKimi}, {store.ProviderDeepSeek}, {store.ProviderCopilot}})
	remote := map[store.Provider]RemoteQuota{
		store.ProviderKimi:     {Meters: []Meter{kimi}},
		store.ProviderDeepSeek: {Meters: []Meter{balance}},
		store.ProviderCopilot:  {Meters: []Meter{premium, chat}},
	}
	v = BuildStatusBar(zh, StatusInputs{Prefs: prefs, Remote: remote, CodexMeters: []Meter{codex}, Now: now})
	if len(v.Components) != 3 {
		t.Fatalf("components = %d", len(v.Components))
	}
	if v.Components[0].Cells[0].Percent != "75%" || v.Components[1].Cells[0].Percent != "¥12.50" {
		t.Fatalf("cells = %+v / %+v", v.Components[0].Cells, v.Components[1].Cells)
	}
	if len(v.Components[2].Cells) != 2 || v.Components[2].Cells[0].Label != "Premium" || v.Components[2].Cells[1].Label != "Chat" {
		t.Fatalf("copilot cells = %+v", v.Components[2].Cells)
	}
	if strings.Contains(v.Tooltip, "80%") {
		t.Fatalf("tooltip used codex: %q", v.Tooltip)
	}
}
