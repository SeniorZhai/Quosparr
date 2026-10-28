package runway

import (
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/store"
)

func TestFloatingShowsTwoStandardWindowsPerProvider(t *testing.T) {
	weekly := testMeter("每周", 60, WeeklyMinutes, SourceStandard)
	five := testMeter("5小时", 20, FiveHourMinutes, SourceStandard)
	spark := testMeter("Spark", 0, WeeklyMinutes, SourceModelSpecific)
	p := store.DefaultPreferences()
	p.FloatingWindow.Providers = []store.Provider{store.ProviderCodex, store.ProviderClaude}
	v := BuildFloating(zh, StatusInputs{
		Prefs: p, Now: time.Now(),
		CodexText: "80%", CodexMeters: []Meter{weekly, spark, five},
		Claude: ClaudeState{Availability: ClaudeNotLoggedIn},
	})
	if len(v.Providers) != 2 {
		t.Fatalf("providers = %+v", v.Providers)
	}
	codex := v.Providers[0]
	if codex.Provider != store.ProviderCodex || len(codex.Meters) != 2 || codex.Status != "" {
		t.Fatalf("codex = %+v", codex)
	}
	if codex.Meters[0].RemainingPercent != 80 || codex.Meters[1].RemainingPercent != 40 || codex.Meters[0].Reset == "" {
		t.Errorf("codex meters = %+v, want the 5-hour window first", codex.Meters)
	}
	claude := v.Providers[1]
	if len(claude.Meters) != 0 || claude.Status == "" {
		t.Errorf("claude = %+v, want a status without meters", claude)
	}
}

func TestFloatingFollowsThePanelProvider(t *testing.T) {
	p := store.DefaultPreferences()
	p.SelectedProvider = store.ProviderGrok
	monthly := testMeter("每月", 10, MonthlyMinutes, SourceStandard)
	v := BuildFloating(zh, StatusInputs{Prefs: p, Now: time.Now(), Grok: GrokState{Availability: GrokReady, Meters: []Meter{monthly}}})
	if len(v.Providers) != 1 || v.Providers[0].Provider != store.ProviderGrok || len(v.Providers[0].Meters) != 1 {
		t.Errorf("view = %+v", v)
	}
}

func TestFloatingWithOnlyModelSpecificWindowShowsIt(t *testing.T) {
	spark := testMeter("Spark", 30, WeeklyMinutes, SourceModelSpecific)
	if got := floatingMeters([]Meter{spark}, codexPreferredWindows); len(got) != 1 || got[0].Title != "Spark" {
		t.Errorf("meters = %+v", got)
	}
	if got := floatingMeters(nil, codexPreferredWindows); len(got) != 0 {
		t.Errorf("no meters = %+v", got)
	}
}

func TestFloatingLiveMatchesStatusBarAndRestoresQuota(t *testing.T) {
	in := StatusInputs{Prefs: store.DefaultPreferences(), Now: time.Now(),
		CodexMeters: []Meter{testMeter("5h", 20, FiveHourMinutes, SourceStandard)},
		Live:        []LivePart{{Provider: store.ProviderCodex, Text: "42 tok/s"}, {Provider: store.ProviderClaude, Text: "8 tok/s"}, {Provider: store.ProviderGrok, Text: "3 tok/s"}}}
	v := BuildFloating(zh, in)
	if !v.Live || len(v.Providers) != 3 {
		t.Fatalf("live view: %+v", v)
	}
	status := BuildStatusBar(zh, in)
	for i, p := range v.Providers {
		if p.LiveText != status.Components[i].Cells[0].MeterText || p.Provider != status.Components[i].Provider {
			t.Fatal("floating speed differs from menu bar")
		}
	}
	if len(v.Providers[0].Meters) != 1 || v.Providers[0].Meters[0].RemainingPercent != 80 {
		t.Fatal("quota gauge missing")
	}
	in.Live = nil
	idle := BuildFloating(zh, in)
	if idle.Live || idle.Providers[0].LiveText != "" || len(idle.Providers[0].Meters) == 0 {
		t.Fatalf("idle did not restore quotas: %+v", idle)
	}
}
